package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"example.com/cache-migrator/pkg/disk"
	"example.com/cache-migrator/pkg/migrator"
	"example.com/cache-migrator/pkg/model"
	"example.com/cache-migrator/pkg/prompt"
	"example.com/cache-migrator/pkg/scanner"
)

func main() {
	var (
		scanCmd     = flag.NewFlagSet("scan", flag.ExitOnError)
		migrateCmd  = flag.NewFlagSet("migrate", flag.ExitOnError)
		scanUser    = scanCmd.String("user", "", "扫描指定用户的家目录缓存")
		target      = migrateCmd.String("target", "", "目标目录（如 /mnt/data），不指定则交互选择磁盘")
		dryRun      = migrateCmd.Bool("dry-run", false, "仅预览，不实际迁移")
		migrateUser = migrateCmd.String("user", "", "迁移指定用户的家目录缓存（需要 root 权限），默认当前用户")
	)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "scan":
		scanCmd.Parse(os.Args[2:])
		runScan(*scanUser)
	case "migrate":
		migrateCmd.Parse(os.Args[2:])
		runMigrate(*target, *dryRun, *migrateUser)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("未知命令: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`cache-migrator - 交互式开发缓存迁移工具

用法:
  cache-migrator scan [--user username]     扫描并显示缓存位置和大小
  cache-migrator migrate [选项]             交互式选择并迁移缓存

migrate 选项:
  --target /mnt/data    指定目标目录（默认交互选择磁盘）
  --dry-run             只打印迁移计划，不执行
  --user username       为指定用户迁移其家目录下的缓存（需要 root 权限）

示例:
  cache-migrator scan
  cache-migrator migrate --target /mnt/data --dry-run
  cache-migrator migrate --target /mnt/data --user wxchy`)
}

func homeDir(userFlag string) string {
	if userFlag != "" {
		// 简单解析 /etc/passwd 获取家目录
		info, err := os.ReadFile("/etc/passwd")
		if err == nil {
			lines := splitLines(string(info))
			for _, line := range lines {
				parts := splitParts(line, ':')
				if len(parts) >= 6 && parts[0] == userFlag {
					return parts[5]
				}
			}
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return home
}

func runScan(userFlag string) {
	home := homeDir(userFlag)
	caches := scanner.Scan(home, userFlag)
	existing := scanner.FilterExists(caches)

	fmt.Printf("\n扫描用户: %s (HOME=%s)\n", currentUserDesc(userFlag), home)
	fmt.Println("=" + repeat("=", 60))
	if len(existing) == 0 {
		fmt.Println("未检测到占用空间的缓存。")
		return
	}
	var total int64
	for _, c := range existing {
		fmt.Printf("%-16s %-12s %s\n", c.DisplayName, c.SizeHuman(), c.CurrentPath)
		total += c.SizeBytes
	}
	fmt.Println("-" + repeat("-", 60))
	fmt.Printf("总计: %s\n", model.HumanSize(total))
}

func runMigrate(targetFlag string, dryRun bool, userFlag string) {
	home := homeDir(userFlag)
	caches := scanner.Scan(home, userFlag)
	existing := scanner.FilterExists(caches)

	fmt.Printf("\n迁移用户: %s (HOME=%s)\n", currentUserDesc(userFlag), home)
	selected := prompt.SelectCaches(existing)
	if len(selected) == 0 {
		fmt.Println("未选择任何缓存，退出。")
		return
	}

	var targetRoot string
	if targetFlag != "" {
		targetRoot = filepath.Clean(targetFlag)
	} else {
		disks, err := disk.List()
		if err != nil {
			fmt.Printf("读取磁盘失败: %v\n", err)
			os.Exit(1)
		}
		d, err := disk.Pick(disks)
		if err != nil {
			fmt.Printf("选择磁盘失败: %v\n", err)
			os.Exit(1)
		}
		targetRoot = d.MountPoint
	}

	fmt.Printf("\n目标根目录: %s\n", targetRoot)
	if dryRun {
		fmt.Println("【演习模式】不会实际移动文件或修改配置")
	}

	if !prompt.Confirm("确认开始迁移吗？") {
		fmt.Println("已取消。")
		return
	}

	var failed int
	for _, c := range selected {
		if err := migrator.Migrate(c, targetRoot, home, userFlag, dryRun); err != nil {
			fmt.Printf("  ❌ 失败: %v\n", err)
			failed++
		} else if !dryRun {
			fmt.Printf("  ✅ 完成\n")
		}
	}

	if !dryRun {
		fmt.Println("\n迁移完成。部分服务（docker/ollama）可能需要手动重启才能生效。")
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func currentUserDesc(userFlag string) string {
	if userFlag != "" {
		return userFlag
	}
	u := os.Getenv("USER")
	if u == "" {
		u = "current"
	}
	return u
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func splitParts(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
