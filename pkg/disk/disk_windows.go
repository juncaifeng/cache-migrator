//go:build windows

package disk

import (
	"fmt"

	"github.com/juncaifeng/cache-migrator/pkg/model"
)

func statDisk(mount string) (model.Disk, error) {
	return model.Disk{}, fmt.Errorf("Windows 暂不支持磁盘扫描，请使用 --target 手动指定目标目录")
}
