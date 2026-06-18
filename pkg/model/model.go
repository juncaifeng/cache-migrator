package model

import "fmt"

// Strategy 表示迁移方式
type Strategy string

const (
	StrategyEnv        Strategy = "env"        // 写入 ~/.bashrc / ~/.zshrc 等环境变量
	StrategyConfig     Strategy = "config"     // 修改配置文件
	StrategySystemd    Strategy = "systemd"    // 修改 systemd service
	StrategyDocker     Strategy = "docker"     // 修改 /etc/docker/daemon.json
	StrategySymlink    Strategy = "symlink"    // 创建软链接
)

// Cache 表示一个缓存项
type Cache struct {
	Name        string   // 英文标识，如 docker/ollama/npm
	DisplayName string   // 中文显示名
	Description string   // 说明
	Strategy    Strategy // 迁移策略

	CurrentPath string // 当前路径
	TargetPath  string // 目标路径
	SizeBytes   int64  // 字节大小
	Exists      bool   // 是否存在
	Err         error  // 检测时产生的错误
}

func (c *Cache) SizeHuman() string {
	return HumanSize(c.SizeBytes)
}

func HumanSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// Disk 表示一个可用磁盘/挂载点
type Disk struct {
	Device     string
	MountPoint string
	Total      uint64
	Free       uint64
	Used       uint64
	FSType     string
}

func (d *Disk) UsagePercent() float64 {
	if d.Total == 0 {
		return 0
	}
	return float64(d.Used) / float64(d.Total) * 100
}

func (d *Disk) FreeHuman() string {
	return HumanSize(int64(d.Free))
}

func (d *Disk) TotalHuman() string {
	return HumanSize(int64(d.Total))
}
