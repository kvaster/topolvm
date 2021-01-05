package lvm

import (
	"context"
	"errors"
	"github.com/fsnotify/fsnotify"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/resource"
	"os/exec"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
)

var btrfsLogger = ctrl.Log.WithName("lvm").WithName("btrfs")

var limitRegexp = regexp.MustCompile(" *Limit referenced: *(\\d+)\\s*")
var usageRegexp = regexp.MustCompile(" *Usage referenced: *(\\d+)\\s*")

var subvolParseError = errors.New("error parsing subvolume info")
var watchError = errors.New("watch error")
var execError = errors.New("execute error")
var noDeviceClassError = errors.New("no such device class")
var noVolumeError = errors.New("no such volume")

const configFile = "devices.yml"

type deviceClassConfig struct {
	Name string `json:"name"`
	Default bool `json:"default"`
	Size string `json:"size"`
}

type config struct {
	DeviceClasses []*deviceClassConfig `json:"device-classes"`
}

type btrfsVolume struct {
	Name string
	Size uint64
}

type deviceClass struct {
	Name string
	Default bool
	Size uint64
	Volumes []*btrfsVolume
}

type btrfs struct {
	poolPath string

	deviceClasses []*deviceClass
	mu            sync.Mutex
}

func newBtrfs(path string) (*btrfs, error) {
	return &btrfs{poolPath: path}, nil
}

func (c *btrfs) GetLVList(deviceClass string) ([]*LogicalVolume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var volumes []*LogicalVolume

	dc := c.findDeviceClass(deviceClass)
	if dc != nil {
		for _, v := range dc.Volumes {
			volumes = append(volumes, &LogicalVolume{Name: v.Name, DeviceClass: deviceClass, Size: v.Size, Tags: []string{}})
		}
	}

	return volumes, nil
}

func (c *btrfs) CreateLV(name, deviceClass string, size uint64, tags []string) (*LogicalVolume, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return nil, noDeviceClassError
	}

	path := c.GetPath(name, deviceClass)

	_, err := runCmd("/sbin/btrfs", "subvol", "create", path)
	if err != nil {
		return nil, err
	}

	_, err = runCmd("/sbin/btrfs", "qgroup", "limit", strconv.FormatUint(size, 10), path)
	if err != nil {
		_, _ = runCmd("/sbin/btrfs", "subvol", "delete", path)
		return nil, err
	}

	dc.Volumes = append(dc.Volumes, &btrfsVolume{Name: name, Size: size})

	return &LogicalVolume{Name: name, DeviceClass: deviceClass, Size: size, Tags: tags}, nil
}

func (c *btrfs) RemoveLV(name, deviceClass string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return noDeviceClassError
	}

	v := dc.findVolume(name)
	if v == nil {
		return noVolumeError
	}

	path := c.GetPath(name, deviceClass)

	_, err := runCmd("/bin/btrfs", "subvol", "delete", path)
	if err != nil {
		return err
	}

	dc.removeVolume(name)

	return nil
}

func (c *btrfs) ResizeLV(name, deviceClass string, size uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return noDeviceClassError
	}

	v := dc.findVolume(name)
	if v == nil {
		return noVolumeError
	}

	path := c.GetPath(name, deviceClass)

	_, err := runCmd("/sbin/btrfs", "qgroup", "limit", strconv.FormatUint(size, 10), path)
	if err != nil {
		return err
	}

	v.Size = size

	return nil
}

func (c *btrfs) GetPath(name, deviceClass string) string {
	return filepath.Join(c.poolPath, deviceClass, name)
}

func (c *btrfs) VolumeStats(name, deviceClass string) (*VolumeStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	dc := c.findDeviceClass(deviceClass)
	if dc == nil {
		return nil, noDeviceClassError
	}

	v := dc.findVolume(name)
	if v == nil {
		return nil, noVolumeError
	}

	path := c.GetPath(name, deviceClass)
	limit, used, err := parseSubvolume(path)
	if err != nil {
		return nil, err
	}

	return &VolumeStats{TotalBytes: limit, UsedBytes: used}, nil
}

func (c *btrfs) NodeStats() (*NodeStats, error) {
	var defaultDc *DeviceClassStats
	var stats []*DeviceClassStats

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
		if name == d.Name {
			return d
		}
	}

	return nil
}

func (d *deviceClass) findVolume(name string) *btrfsVolume {
	for _, v := range d.Volumes {
		if name == v.Name {
			return v
		}
	}

	return nil
}

func (d *deviceClass) removeVolume(name string) {
	var vs []*btrfsVolume
	for _, v := range d.Volumes {
		if name != v.Name {
			vs = append(vs, v)
		}
	}
	d.Volumes = vs
}

func (c *btrfs) Start(ch <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-ch
		cancel()
	}()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(filepath.Join(c.poolPath, configFile)); err != nil {
		return err
	}

	c.loadConfigWithInfo()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.Events:
			if !ok {
				btrfsLogger.Error(nil, "Not OK on fs event")
				return watchError
			}

			if event.Op & (fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename) != 0 {
				c.loadConfigWithInfo()
			}

			break
		case err, ok := <-w.Errors:
			if !ok {
				btrfsLogger.Error(nil, "Not OK on watch error")
				return watchError
			}
			btrfsLogger.Error(err, "Watch error")
			break
		}
	}
}

func (c *btrfs) loadConfigWithInfo() {
	err := c.loadConfig()
	if err != nil {
		btrfsLogger.Error(err, "Error loading config")
	}
}

func (c *btrfs) loadConfig() error {
	btrfsLogger.Info("Loading config")

	b, err := ioutil.ReadFile(filepath.Join(c.poolPath, configFile))
	if err != nil {
		return err
	}

	cnf := &config{}
	err = yaml.Unmarshal(b, &cnf)
	if err != nil {
		return err
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

			files, err := ioutil.ReadDir(filepath.Join(c.poolPath, dcc.Name))
			if err != nil {
				btrfsLogger.Error(err, "Error reading device class path", "DeviceClass", dcc.Name)
				return err
			}

			var volumes []*btrfsVolume
			for _, file := range files {
				limit, _, err := parseSubvolume(filepath.Join(c.poolPath, file.Name()))
				if err != nil {
					btrfsLogger.Error(err, "Error parsing subvolume info", "DeviceClass", dcc.Name)
					return err
				}

				volumes = append(volumes, &btrfsVolume{Name: file.Name(), Size: limit})
			}

			dc = &deviceClass{Name: dcc.Name, Volumes: volumes}
		}

		dcs = append(dcs, dc)
		dcMap[dc.Name] = true

		dc.Default = dcc.Default
		size, err := resource.ParseQuantity(dcc.Size)
		if err != nil {
			btrfsLogger.Info("Can't parse size", "DeviceClass", dcc.Name, "Size", dcc.Size)
			return nil
		}
		dc.Size = uint64(size.Value())
	}

	for _, d := range c.deviceClasses {
		if !dcMap[d.Name] {
			btrfsLogger.Info("Removing device class", "DeviceClass", d.Name)
		}
	}

	c.deviceClasses = dcs

	btrfsLogger.Info("Config loaded")

	return nil
}

func parseSubvolume(path string) (uint64, uint64, error) {
	out, err := runCmd("/sbin/btrfs", "subvol", "show", "--raw", path)
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(out, "\n")
	var limit uint64 = -1
	var used uint64 = -1
	for _, line := range lines {
		if m := limitRegexp.FindStringSubmatch(line); m != nil {
			limit, err = strconv.ParseUint(m[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		} else if m := usageRegexp.FindStringSubmatch(line); m != nil {
			used, err = strconv.ParseUint(m[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		}
	}

	if limit <= 0 || used <= 0 {
		return 0, 0, subvolParseError
	}

	return limit, used, nil
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
	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}
	if err := c.Wait(); err != nil {
		return "", err
	}
	if c.ProcessState.ExitCode() != 0 {
		btrfsLogger.Info("Exit code is non-zero", "ExitCode", c.ProcessState.ExitCode())
		return "", execError
	}
	return string(out), nil
}
