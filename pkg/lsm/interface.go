package lsm

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var ErrNoDeviceClass = errors.New("no such device class")
var ErrNoVolume = errors.New("no such volume")

type LogicalVolume struct {
	Name        string
	DeviceClass string
	Size        uint64
}

type NodeStats struct {
	DeviceClasses []*DeviceClassStats
	Default       *DeviceClassStats
}

type VolumeStats struct {
	TotalBytes uint64
	UsedBytes  uint64
}

type DeviceClassStats struct {
	VolumeStats
	DeviceClass string
}

type Client interface {
	manager.Runnable

	GetLVList(deviceClass string) ([]*LogicalVolume, error)
	CreateLV(name, deviceClass string, noCow bool, size uint64) (*LogicalVolume, error)
	RemoveLV(name, deviceClass string) error
	ResizeLV(name, deviceClass string, size uint64) error
	CreateLVSnapshot(name, deviceClass, sourceVolID string, size uint64, accessType string) (*LogicalVolume, error)

	GetPath(v *LogicalVolume) string

	VolumeStats(name, deviceClass string) (*VolumeStats, error)
	NodeStats() (*NodeStats, error)

	Watch() chan struct{}
}
