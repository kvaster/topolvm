package lvm

import "path/filepath"

type btrfs struct {
	poolPath string
}

func (c *btrfs) GetLVList(deviceClass string) ([]*LogicalVolume, error) {
	return nil, nil
}

func (c *btrfs) CreateLV(name, deviceClass string, size uint64, tags []string) (*LogicalVolume, error) {
	return nil, nil
}

func (c *btrfs) RemoveLV(name, deviceClass string) error {
	return nil
}

func (c *btrfs) ResizeLV(name, deviceClass string, size uint64) error {
	return nil
}

func (c *btrfs) GetPath(name, deviceClass string) string {
	return filepath.Join(c.poolPath, deviceClass, name)
}

func (c *btrfs) VolumeStats(name, deviceClass string) (*VolumeStats, error) {
	return nil, nil
}

func (c *btrfs) NodeStats() (*NodeStats, error) {
	return nil, nil
}
