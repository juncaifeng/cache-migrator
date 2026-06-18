package migrator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/juncaifeng/cache-migrator/pkg/model"
)

// targetPathFor 根据缓存类型、目标根目录和用户生成合理的目标路径
func targetPathFor(name, targetRoot, user string) string {
	// 多用户场景下，按用户名分目录
	var base string
	if user == "" || user == "root" {
		base = filepath.Join(targetRoot, "cache")
	} else {
		base = filepath.Join(targetRoot, "cache", user)
	}

	switch name {
	case "docker":
		return filepath.Join(targetRoot, "docker")
	case "ollama":
		return filepath.Join(targetRoot, "ollama", "models")
	case "go":
		return filepath.Join(targetRoot, "go")
	case "conda":
		return filepath.Join(targetRoot, "conda")
	default:
		return filepath.Join(base, name)
	}
}

// isSubDir 判断 child 是否在 parent 目录下
func isSubDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// Migrate 执行单个缓存迁移：移动数据 + 配置持久化
func Migrate(c *model.Cache, targetRoot, home, user string, dryRun bool) error {
	if !c.Exists {
		return fmt.Errorf("%s 当前路径不存在，无法迁移", c.DisplayName)
	}

	// 生成新路径
	c.TargetPath = targetPathFor(c.Name, targetRoot, user)

	// 如果已经在目标盘下，跳过
	if isSubDir(targetRoot, c.CurrentPath) {
		if dryRun {
			fmt.Printf("[DRY-RUN] %s 已在目标盘 %s 下，跳过\n", c.DisplayName, c.CurrentPath)
		} else {
			fmt.Printf("  ⏭️  %s 已在目标盘 %s 下，跳过\n", c.DisplayName, c.CurrentPath)
		}
		return nil
	}

	if dryRun {
		fmt.Printf("[DRY-RUN] 将 %s 从 %s 迁移到 %s\n", c.DisplayName, c.CurrentPath, c.TargetPath)
		return nil
	}

	fmt.Printf("\n正在迁移 %s ...\n", c.DisplayName)
	fmt.Printf("  源路径: %s (%s)\n", c.CurrentPath, c.SizeHuman())
	fmt.Printf("  目标路径: %s\n", c.TargetPath)

	// 1. 创建目标目录
	if err := os.MkdirAll(filepath.Dir(c.TargetPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 2. 如果目标已存在，拒绝覆盖
	if _, err := os.Stat(c.TargetPath); err == nil {
		return fmt.Errorf("目标路径已存在: %s", c.TargetPath)
	}

	// 3. 根据策略迁移
	switch c.Strategy {
	case model.StrategyEnv:
		return migrateEnv(c, home)
	case model.StrategyDocker:
		return migrateDocker(c)
	case model.StrategySystemd:
		return migrateSystemd(c, home)
	case model.StrategySymlink:
		return migrateSymlink(c)
	default:
		return fmt.Errorf("未知迁移策略: %s", c.Strategy)
	}
}

// migrateEnv 移动目录并写入 shell profile
func migrateEnv(c *model.Cache, home string) error {
	if err := moveDir(c.CurrentPath, c.TargetPath); err != nil {
		return err
	}

	varName := envVarFor(c.Name)
	if varName == "" {
		return fmt.Errorf("未定义 %s 对应的环境变量", c.Name)
	}

	profiles, err := shellProfiles(home)
	if err != nil {
		return err
	}

	lines := []string{fmt.Sprintf("export %s=%s", varName, c.TargetPath)}
	// Go/Cargo 的二进制目录通常在 PATH 中，迁移后需要更新 PATH
	if c.Name == "go" {
		lines = append(lines, fmt.Sprintf("export PATH=$PATH:%s", filepath.Join(c.TargetPath, "bin")))
	}
	if c.Name == "cargo" {
		lines = append(lines, fmt.Sprintf("export PATH=$PATH:%s", filepath.Join(c.TargetPath, "bin")))
	}

	for _, p := range profiles {
		for _, line := range lines {
			if err := appendUniqueLine(p, line); err != nil {
				return err
			}
		}
	}

	fmt.Printf("  已写入环境变量 %s 到 %s\n", varName, strings.Join(profiles, ", "))
	if len(lines) > 1 {
		fmt.Printf("  同时更新了 PATH，新 shell 或 source profile 后生效\n")
	}
	return nil
}

// migrateDocker 修改 /etc/docker/daemon.json 并提示重启
func migrateDocker(c *model.Cache) error {
	const daemonPath = "/etc/docker/daemon.json"

	// 先移动数据
	if err := moveDir(c.CurrentPath, c.TargetPath); err != nil {
		return err
	}

	data, err := os.ReadFile(daemonPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	cfg := make(map[string]interface{})
	if len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("解析 daemon.json 失败: %w", err)
		}
	}
	cfg["data-root"] = c.TargetPath

	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(daemonPath, out, 0644); err != nil {
		return err
	}

	fmt.Println("  已更新 /etc/docker/daemon.json")
	fmt.Println("  请手动执行: systemctl restart docker")
	return nil
}

// migrateSystemd 修改 ollama.service 并提示重启
func migrateSystemd(c *model.Cache, home string) error {
	if err := moveDir(c.CurrentPath, c.TargetPath); err != nil {
		return err
	}

	const svcPath = "/etc/systemd/system/ollama.service"
	data, err := os.ReadFile(svcPath)
	if err != nil {
		return fmt.Errorf("读取 ollama.service 失败: %w", err)
	}

	content := string(data)
	newEnv := fmt.Sprintf(`Environment="OLLAMA_MODELS=%s"`, c.TargetPath)

	if strings.Contains(content, "OLLAMA_MODELS=") {
		// 替换已存在的环境变量行
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(line, "OLLAMA_MODELS=") {
				lines[i] = newEnv
			}
		}
		content = strings.Join(lines, "\n")
	} else {
		// 在 [Service] 段下追加
		content = strings.Replace(content, "[Service]\n", "[Service]\n"+newEnv+"\n", 1)
	}

	if err := os.WriteFile(svcPath, []byte(content), 0644); err != nil {
		return err
	}

	exec.Command("systemctl", "daemon-reload").Run()
	fmt.Println("  已更新 /etc/systemd/system/ollama.service")

	// 同时写入 shell profile，让当前/新终端和 cache-migrator scan 都能识别
	profiles, err := shellProfiles(home)
	if err == nil {
		line := fmt.Sprintf("export OLLAMA_MODELS=%s", c.TargetPath)
		for _, p := range profiles {
			_ = appendUniqueLine(p, line)
		}
		fmt.Printf("  已写入 OLLAMA_MODELS 到 %s\n", strings.Join(profiles, ", "))
	}

	fmt.Println("  请手动执行: systemctl restart ollama")
	return nil
}

// migrateSymlink 移动目录并创建软链接
func migrateSymlink(c *model.Cache) error {
	if err := moveDir(c.CurrentPath, c.TargetPath); err != nil {
		return err
	}
	if err := os.Symlink(c.TargetPath, c.CurrentPath); err != nil {
		return fmt.Errorf("创建软链接失败: %w", err)
	}
	fmt.Printf("  已创建软链接 %s -> %s\n", c.CurrentPath, c.TargetPath)
	return nil
}

// moveDir 使用 os.Rename 跨设备失败时回退到复制+删除
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 跨设备，复制后删除
	if err := copyDir(src, dst); err != nil {
		return fmt.Errorf("复制目录失败: %w", err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("删除源目录失败: %w", err)
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	// 使用 1MB 缓冲区提升大文件跨设备复制速度
	buf := make([]byte, 1024*1024)
	_, err = io.CopyBuffer(out, in, buf)
	return err
}

func envVarFor(name string) string {
	m := map[string]string{
		"npm":    "npm_config_cache",
		"bun":    "BUN_INSTALL_CACHE_DIR",
		"pnpm":   "PNPM_HOME",
		"go":     "GOPATH",
		"cargo":  "CARGO_HOME",
		"rustup": "RUSTUP_HOME",
		"pip":    "PIP_CACHE_DIR",
		"uv":     "UV_CACHE_DIR",
		"conda":  "CONDA_ROOT",
	}
	return m[name]
}

func shellProfiles(home string) ([]string, error) {
	if home == "" {
		return nil, fmt.Errorf("无法获取 HOME 目录")
	}

	var profiles []string
	for _, rc := range []string{".bashrc", ".zshrc", ".profile"} {
		p := filepath.Join(home, rc)
		if _, err := os.Stat(p); err == nil {
			profiles = append(profiles, p)
		}
	}
	if len(profiles) == 0 {
		// 兜底创建 .bashrc
		profiles = append(profiles, filepath.Join(home, ".bashrc"))
	}
	return profiles, nil
}

func appendUniqueLine(path, line string) error {
	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, line) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n# cache-migrator\n%s\n", line)
	return err
}

func SizeHuman(b int64) string {
	return model.HumanSize(b)
}

var _ = model.Disk{} // 确保 model 包被引用
