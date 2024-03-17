package lsm

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	chattr "github.com/g0rbe/go-chattr"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

var btrfsLogger = ctrl.Log.WithName("lsm").WithName("btrfs")

var limitRegexp = regexp.MustCompile(`\s*Limit referenced:\s*(\d+)\s*`)
var usageRegexp = regexp.MustCompile(`\s*Usage referenced:\s*(\d+)\s*`)
var subvolRegexp = regexp.MustCompile(`\s*Subvolume ID:\s*(\d+)\s*`)

var errParseInfo = errors.New("error parsing info")
var errWatch = errors.New("watch error")
var errExec = errors.New("execute error")
var errNoDeviceClass = errors.New("no such device class")
var errNoVolume = errors.New("no such volume")

const configFile = "devices.yml"

type deviceClassConfig struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
	Size    string `json:"size"`
}

type config struct {
	DeviceClasses []*deviceClassConfig `json:"device-classes"`
}

type deviceClass struct {
	Name    string
	Default bool
	Size    uint64
	Volumes []*LogicalVolume
}

type btrfs struct {
	poolPath string

	deviceClasses []*deviceClass
	mu            sync.Mutex
	watches       []chan struct{}
}

func newBtrfs(path string) (*btrfs, error) {
	fs := &btrfs{poolPath: path}
	fs.loadConfig()
	return fs, nil
}

func (c *btrfs) Watch() chan struct{} {
	ch := make(chan struct{})
	c.watches = append(c.watches, ch)
	return ch
}

func (c *btrfs) notify() {
	for _, w := range c.watches {
		var _w = w
		// use separate goroutine to avoid deadlock
		go func() {
			_w <- struct{}{}
		}()
	}
}

func (c *btrfs) GetLVList(deviceClass string) ([]*LogicalVolume, error) {
	btrfsLogger.Info("GetLVList", "DeviceClass", deviceClass)

	c.mu.Lock()
	defer c.mu.Unlock()

	var volumes []*LogicalVolume

	dc := c.findDeviceClass(deviceClass)
	if dc != nil {
		for _, v := range dc.Volumes {
			volumes = append(volumes, &LogicalVolume{Name: v.Name, DeviceClass: dc.Name, Size: v.Size})
		}
	}

	return volumes, nil
}

func (c *btrfs) CreateLV(name, deviceClass string, noCow bool, size uint64) (*LogicalVolume, error) {
	btrfsLogger.Info("CreateLV", "Name", name, "DeviceClass", deviceClass, "Size", size)

	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return nil, errNoDeviceClass
	}

	v := &LogicalVolume{Name: name, DeviceClass: dc.Name, Size: size}
	path := c.GetPath(v)

	_, err := runCmd("/sbin/btrfs", "subvol", "create", path)
	if err != nil {
		return nil, err
	}

	_, err = runCmd("/sbin/btrfs", "qgroup", "limit", strconv.FormatUint(size, 10), path)
	if err != nil {
		_ = removeSubvol(path)
		return nil, err
	}

	if noCow {
		f, err := os.OpenFile(path, os.O_RDONLY, 0666)
		if err == nil {
			defer func() { _ = f.Close() }()
			err = chattr.SetAttr(f, chattr.FS_NOCOW_FL)
		}

		if err != nil {
			_ = removeSubvol(path)
			return nil, err
		}
	}

	dc.Volumes = append(dc.Volumes, v)

	c.notify()

	return v, nil
}

func (c *btrfs) CreateLVSnapshot(name, deviceClass, sourceVolID string, size uint64, accessType string) (*LogicalVolume, error) {
	btrfsLogger.Info("CreateLVSNapshot", "Name", name, "DeviceClass", deviceClass, "Size", size, "sourceVolID", sourceVolID, "accessType", accessType)

	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return nil, errNoDeviceClass
	}

	v := &LogicalVolume{Name: name, DeviceClass: dc.Name, Size: size}
	path := c.GetPath(v)

	sv := &LogicalVolume{Name: sourceVolID, DeviceClass: dc.Name, Size: size}
	srcPath := c.GetPath(sv)

	args := []string{"subvol", "snapshot"}
	if accessType == "ro" {
		args = append(args, "-r")
	}
	args = append(args, srcPath, path)

	_, err := runCmd("/sbin/btrfs", args...)
	if err != nil {
		return nil, err
	}

	_, err = runCmd("/sbin/btrfs", "qgroup", "limit", strconv.FormatUint(size, 10), path)
	if err != nil {
		_ = removeSubvol(path)
		return nil, err
	}

	dc.Volumes = append(dc.Volumes, v)

	c.notify()

	return v, nil
}

func removeSubvol(path string) error {
	_, _, subvolId, err := parseSubvolume(path)
	if err != nil {
		btrfsLogger.Info("Error parsing subvolume info", "Err", err.Error(), "Path", path)
		return err
	}

	_, err = runCmd("/sbin/btrfs", "qgroup", "destroy", "0/"+strconv.FormatUint(subvolId, 10), path)
	if err != nil {
		btrfsLogger.Info("Warning: error on qgroup destroy", "Err", err.Error(), "Path", path)
	}

	_, err = runCmd("/sbin/btrfs", "subvol", "delete", "-c", path)
	if err != nil {
		btrfsLogger.Info("Error on subvol delete", "Err", err.Error(), "Path", path)
		return err
	}

	return nil
}

func (c *btrfs) RemoveLV(name, deviceClass string) error {
	btrfsLogger.Info("RemoveLV", "Name", name, "DeviceClass", deviceClass)

	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return errNoDeviceClass
	}

	v := dc.findVolume(name)
	if v == nil {
		return errNoVolume
	}

	path := c.GetPath(v)

	if err := removeSubvol(path); err != nil {
		return err
	}

	dc.removeVolume(name)

	c.notify()

	return nil
}

func (c *btrfs) ResizeLV(name, deviceClass string, size uint64) error {
	btrfsLogger.Info("ResizeLV", "Name", name, "DeviceClass", deviceClass, "Size", size)

	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return errNoDeviceClass
	}

	v := dc.findVolume(name)
	if v == nil {
		return errNoVolume
	}

	path := c.GetPath(v)

	_, err := runCmd("/sbin/btrfs", "qgroup", "limit", strconv.FormatUint(size, 10), path)
	if err != nil {
		return err
	}

	v.Size = size

	c.notify()

	return nil
}

func (c *btrfs) GetPath(v *LogicalVolume) string {
	return filepath.Join(c.poolPath, v.DeviceClass, v.Name)
}

func (c *btrfs) VolumeStats(name, deviceClass string) (*VolumeStats, error) {
	btrfsLogger.Info("VolumeStats", "Name", name, "DeviceClass", deviceClass)

	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return nil, errNoDeviceClass
	}

	v := dc.findVolume(name)
	if v == nil {
		return nil, errNoVolume
	}

	path := c.GetPath(v)
	limit, used, _, err := parseSubvolume(path)
	if err != nil {
		btrfsLogger.Info("Error parsing subvolume info", "DeviceClass", dc.Name, "Name", name, "Err", err.Error())
		return nil, err
	}

	return &VolumeStats{TotalBytes: limit, UsedBytes: used}, nil
}

func (c *btrfs) NodeStats() (*NodeStats, error) {
	btrfsLogger.Info("NodeStats called")

	var defaultDc *DeviceClassStats
	var stats []*DeviceClassStats

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, dc := range c.deviceClasses {
		var used uint64 = 0
		for _, v := range dc.Volumes {
			used += v.Size
		}
		s := &DeviceClassStats{VolumeStats: VolumeStats{TotalBytes: dc.Size, UsedBytes: used}, DeviceClass: dc.Name}
		stats = append(stats, s)
		if dc.Default {
			defaultDc = s
		}
	}

	return &NodeStats{DeviceClasses: stats, Default: defaultDc}, nil
}

func (c *btrfs) findDeviceClass(name string) *deviceClass {
	for _, d := range c.deviceClasses {
		if name == d.Name || (name == "" && d.Default) {
			return d
		}
	}

	return nil
}

func (d *deviceClass) findVolume(name string) *LogicalVolume {
	for _, v := range d.Volumes {
		if name == v.Name {
			return v
		}
	}

	return nil
}

func (d *deviceClass) removeVolume(name string) {
	var vs []*LogicalVolume
	for _, v := range d.Volumes {
		if name != v.Name {
			vs = append(vs, v)
		}
	}
	d.Volumes = vs
}

func (c *btrfs) Start(ctx context.Context) error {
	btrfsLogger.Info("Starting config watcher")

	w, err := fsnotify.NewWatcher()
	if err != nil {
		btrfsLogger.Error(err, "Error creating watcher")
		return err
	}
	if err := w.Add(c.poolPath); err != nil {
		btrfsLogger.Error(err, "Error watching for configFile dir")
		return err
	}

	// reload config after watcher is enabled
	c.loadConfig()

	for {
		select {
		case <-ctx.Done():
			btrfsLogger.Info("Finishing")
			return nil
		case event, ok := <-w.Events:
			if !ok {
				btrfsLogger.Error(nil, "Not OK on fs event")
				return errWatch
			}

			btrfsLogger.Info("Watch event", "Name", event.Name, "Op", event.Op)

			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
				if filepath.Base(event.Name) == configFile {
					c.loadConfig()
				}
			}
		case err, ok := <-w.Errors:
			if !ok {
				btrfsLogger.Error(nil, "Not OK on watch error")
				return errWatch
			}
			btrfsLogger.Error(err, "Watch error")
		}
	}
}

func (c *btrfs) loadConfig() {
	btrfsLogger.Info("Loading config")

	b, err := os.ReadFile(filepath.Join(c.poolPath, configFile))
	if err != nil {
		btrfsLogger.Info("Error loading config file", "Err", err.Error())
		return
	}

	cnf := &config{}
	err = yaml.Unmarshal(b, &cnf)
	if err != nil {
		btrfsLogger.Info("Error parsing config", "Err", err.Error())
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	dcMap := make(map[string]bool)
	var dcs []*deviceClass
	for _, dcc := range cnf.DeviceClasses {
		dc := (*deviceClass)(nil)
		for _, d := range c.deviceClasses {
			if d.Name == dcc.Name {
				dc = d
				break
			}
		}

		if dc == nil {
			btrfsLogger.Info("Adding device class", "DeviceClass", dcc.Name)

			files, err := os.ReadDir(filepath.Join(c.poolPath, dcc.Name))
			if err != nil {
				btrfsLogger.Error(err, "Error listing device class path", "DeviceClass", dcc.Name)
				return
			}

			var volumes []*LogicalVolume
			for _, file := range files {
				limit, _, _, err := parseSubvolume(filepath.Join(c.poolPath, dcc.Name, file.Name()))
				if err != nil {
					btrfsLogger.Info("Error parsing subvolume info", "DeviceClass", dcc.Name, "Path", file.Name(), "Err", err.Error())
					return
				}
				if limit == 0 {
					btrfsLogger.Info("Error: subvolume limit is undefined", "DeviceClass", dcc.Name, "Path", file.Name())
					return
				}

				volumes = append(volumes, &LogicalVolume{Name: file.Name(), Size: limit, DeviceClass: dcc.Name})
			}

			dc = &deviceClass{Name: dcc.Name, Volumes: volumes}
		}

		dcs = append(dcs, dc)
		dcMap[dc.Name] = true

		dc.Default = dcc.Default
		size, err := resource.ParseQuantity(dcc.Size)
		if err != nil {
			btrfsLogger.Info("Can't parse size", "DeviceClass", dcc.Name, "Size", dcc.Size, "Err", err.Error())
			return
		}
		dc.Size = uint64(size.Value())
	}

	for _, d := range c.deviceClasses {
		if !dcMap[d.Name] {
			btrfsLogger.Info("Removing device class", "DeviceClass", d.Name)
		}
	}

	c.deviceClasses = dcs

	c.notify()

	btrfsLogger.Info("Config loaded")
}

func parseSubvolume(path string) (uint64, uint64, uint64, error) {
	out, err := runCmd("/sbin/btrfs", "subvol", "show", "--raw", path)
	if err != nil {
		return 0, 0, 0, err
	}

	var limit uint64 = 0
	var used uint64 = 0
	var volId uint64 = 0

	for _, line := range strings.Split(out, "\n") {
		err = nil
		var name string
		if m := limitRegexp.FindStringSubmatch(line); m != nil {
			limit, err = strconv.ParseUint(m[1], 10, 64)
			name = "limit"
		} else if m := usageRegexp.FindStringSubmatch(line); m != nil {
			used, err = strconv.ParseUint(m[1], 10, 64)
			name = "usage"
		} else if m := subvolRegexp.FindStringSubmatch(line); m != nil {
			volId, err = strconv.ParseUint(m[1], 10, 64)
			name = "subvolId"
		}

		if err != nil {
			btrfsLogger.Info("Parse error", "Name", name, "Err", err.Error())
			return 0, 0, 0, errParseInfo
		}
	}

	if volId == 0 {
		btrfsLogger.Info("No VolumeID", "Path", path)
		return 0, 0, 0, errParseInfo
	}

	return limit, used, volId, nil
}

func runCmd(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	c.Stderr = c.Stdout

	stdout, err := c.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := c.Start(); err != nil {
		return "", err
	}
	out, err := io.ReadAll(stdout)
	if err != nil {
		return "", err
	}
	if err := c.Wait(); err != nil {
		return "", err
	}
	if c.ProcessState.ExitCode() != 0 {
		btrfsLogger.Info("Exit code is non-zero", "ExitCode", c.ProcessState.ExitCode(), "Cmd", cmd, "Args", args)
		return "", errExec
	}
	return string(out), nil
}
