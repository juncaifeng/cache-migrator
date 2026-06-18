package disk

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"example.com/cache-migrator/pkg/model"
)

// List 读取 /proc/mounts 中真实的块设备挂载点，并返回可用空间
func List() ([]model.Disk, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var disks []model.Disk
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		device := fields[0]
		mount := fields[1]
		fstype := fields[2]

		// 跳过虚拟文件系统
		if !isRealDevice(device) {
			continue
		}
		// 跳过重复挂载点
		if seen[mount] {
			continue
		}
		seen[mount] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free

		disks = append(disks, model.Disk{
			Device:     device,
			MountPoint: mount,
			Total:      total,
			Free:       free,
			Used:       used,
			FSType:     fstype,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return disks, nil
}

func isRealDevice(device string) bool {
	// 常见的真实块设备前缀
	prefixes := []string{"/dev/sd", "/dev/nvme", "/dev/vd", "/dev/hd", "/dev/xvd", "/dev/mmcblk"}
	for _, p := range prefixes {
		if strings.HasPrefix(device, p) {
			return true
		}
	}
	return false
}

// Pick 打印磁盘列表并让用户选择一个
func Pick(disks []model.Disk) (*model.Disk, error) {
	if len(disks) == 0 {
		return nil, fmt.Errorf("未找到可用的物理磁盘")
	}
	fmt.Println("\n可选磁盘：")
	for i, d := range disks {
		fmt.Printf("  [%d] %s 挂载于 %s  总 %s / 已用 %.1f%% / 可用 %s  [%s]\n",
			i+1, d.Device, d.MountPoint, d.TotalHuman(), d.UsagePercent(), d.FreeHuman(), d.FSType)
	}
	fmt.Print("请选择目标磁盘编号: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil {
		return nil, fmt.Errorf("输入无效: %w", err)
	}
	if choice < 1 || choice > len(disks) {
		return nil, fmt.Errorf("选择超出范围")
	}
	return &disks[choice-1], nil
}
