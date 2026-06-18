//go:build linux || darwin

package disk

import (
	"syscall"

	"github.com/juncaifeng/cache-migrator/pkg/model"
)

// statDisk 返回指定挂载点的容量信息
func statDisk(mount string) (model.Disk, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mount, &stat); err != nil {
		return model.Disk{}, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return model.Disk{
		MountPoint: mount,
		Total:      total,
		Free:       free,
		Used:       used,
	}, nil
}
