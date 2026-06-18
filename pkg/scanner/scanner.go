package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"example.com/cache-migrator/pkg/model"
)

// Scanner 保存一次扫描的上下文
type Scanner struct {
	home   string
	user   string // 为空表示当前用户，否则用 sudo -u user 执行命令
}

// All 返回所有支持的缓存类型定义（不含检测结果）
func All() []*model.Cache {
	return []*model.Cache{
		{Name: "docker", DisplayName: "Docker", Description: "Docker 镜像/容器/卷数据 (/etc/docker/daemon.json)", Strategy: model.StrategyDocker},
		{Name: "ollama", DisplayName: "Ollama 模型", Description: "Ollama 下载的模型文件", Strategy: model.StrategySystemd},
		{Name: "npm", DisplayName: "npm 缓存", Description: "npm 包缓存", Strategy: model.StrategyEnv},
		{Name: "bun", DisplayName: "Bun 缓存", Description: "Bun 包管理器缓存", Strategy: model.StrategyEnv},
		{Name: "pnpm", DisplayName: "pnpm 仓库", Description: "pnpm store 目录", Strategy: model.StrategyEnv},
		{Name: "go", DisplayName: "Go 模块/编译缓存", Description: "GOPATH/pkg/mod + GOCACHE", Strategy: model.StrategyEnv},
		{Name: "cargo", DisplayName: "Cargo 注册表", Description: "Rust Cargo 依赖缓存", Strategy: model.StrategyEnv},
		{Name: "rustup", DisplayName: "Rustup 工具链", Description: "Rust 工具链安装目录", Strategy: model.StrategyEnv},
		{Name: "pip", DisplayName: "pip 缓存", Description: "Python pip 包缓存", Strategy: model.StrategyEnv},
		{Name: "uv", DisplayName: "uv 缓存", Description: "Python uv 工具缓存", Strategy: model.StrategyEnv},
		{Name: "conda", DisplayName: "Conda 环境", Description: "Conda 安装根目录", Strategy: model.StrategyEnv},
	}
}

// Scan 检测指定用户家目录下每个缓存的实际路径和大小
func Scan(home, user string) []*model.Cache {
	s := &Scanner{home: home, user: user}
	caches := All()
	for _, c := range caches {
		switch c.Name {
		case "docker":
			s.scanDocker(c)
		case "ollama":
			s.scanOllama(c)
		case "npm":
			s.scanNpm(c)
		case "bun":
			s.scanBun(c)
		case "pnpm":
			s.scanPnpm(c)
		case "go":
			s.scanGo(c)
		case "cargo":
			s.scanCargo(c)
		case "rustup":
			s.scanRustup(c)
		case "pip":
			s.scanPip(c)
		case "uv":
			s.scanUv(c)
		case "conda":
			s.scanConda(c)
		}
		if c.CurrentPath != "" {
			c.SizeBytes, c.Exists = dirSize(c.CurrentPath)
		}
	}
	return caches
}

// scanDocker 读取 /etc/docker/daemon.json 的 data-root
func (s *Scanner) scanDocker(c *model.Cache) {
	const defaultPath = "/var/lib/docker"
	c.CurrentPath = defaultPath
	data, err := os.ReadFile("/etc/docker/daemon.json")
	if err != nil {
		return
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	if v, ok := cfg["data-root"].(string); ok && v != "" {
		c.CurrentPath = v
	}
}

// scanOllama 检查 OLLAMA_MODELS 环境变量，否则 ~/.ollama/models
func (s *Scanner) scanOllama(c *model.Cache) {
	if v := s.env("OLLAMA_MODELS"); v != "" {
		c.CurrentPath = v
		return
	}
	c.CurrentPath = filepath.Join(s.home, ".ollama", "models")
}

// scanNpm 使用 npm config get cache
func (s *Scanner) scanNpm(c *model.Cache) {
	c.CurrentPath = s.runCmd("npm", "config", "get", "cache")
}

// scanBun 使用 bun pm cache dir
func (s *Scanner) scanBun(c *model.Cache) {
	c.CurrentPath = s.runCmd("bun", "pm", "cache", "dir")
}

// scanPnpm 使用 pnpm config get store-dir
func (s *Scanner) scanPnpm(c *model.Cache) {
	c.CurrentPath = s.runCmd("pnpm", "config", "get", "store-dir")
}

// scanGo 使用 go env GOPATH
func (s *Scanner) scanGo(c *model.Cache) {
	gopath := s.runCmd("go", "env", "GOPATH")
	if gopath == "" {
		c.CurrentPath = filepath.Join(s.home, "go")
		return
	}
	c.CurrentPath = gopath
	c.Description = fmt.Sprintf("GOPATH=%s (含 GOCACHE/GOMODCACHE 可单独配置)", gopath)
}

// scanCargo 检查 CARGO_HOME 环境变量或 ~/.cargo
func (s *Scanner) scanCargo(c *model.Cache) {
	if v := s.env("CARGO_HOME"); v != "" {
		c.CurrentPath = v
		return
	}
	c.CurrentPath = filepath.Join(s.home, ".cargo")
}

// scanRustup 检查 RUSTUP_HOME 环境变量或 ~/.rustup
func (s *Scanner) scanRustup(c *model.Cache) {
	if v := s.env("RUSTUP_HOME"); v != "" {
		c.CurrentPath = v
		return
	}
	c.CurrentPath = filepath.Join(s.home, ".rustup")
}

// scanPip 使用 pip cache dir
func (s *Scanner) scanPip(c *model.Cache) {
	c.CurrentPath = s.runCmd("pip", "cache", "dir")
}

// scanUv 使用 uv cache dir
func (s *Scanner) scanUv(c *model.Cache) {
	c.CurrentPath = s.runCmd("uv", "cache", "dir")
}

// scanConda 使用 conda info --base
func (s *Scanner) scanConda(c *model.Cache) {
	c.CurrentPath = s.runCmd("conda", "info", "--base")
}

// env 获取指定环境变量；如果指定了用户，用 sudo 读取该用户环境
func (s *Scanner) env(key string) string {
	if s.user == "" {
		return os.Getenv(key)
	}
	return s.runCmd("printenv", key)
}

func (s *Scanner) runCmd(name string, args ...string) string {
	var cmd *exec.Cmd
	if s.user != "" {
		// 用 sudo 切换到目标用户执行，同时设置 HOME 为其家目录
		all := append([]string{"-u", s.user, "-H", name}, args...)
		cmd = exec.Command("sudo", all...)
	} else {
		cmd = exec.Command(name, args...)
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	outStr := strings.TrimSpace(string(out))
	if outStr == "undefined" || outStr == "null" {
		return ""
	}
	lines := strings.Split(outStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// dirSize 计算目录总大小，返回字节数和是否存在
func dirSize(path string) (int64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	if !info.IsDir() {
		return info.Size(), true
	}

	var total int64
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, false
	}
	return total, true
}

// FilterExists 只保留检测到的缓存
func FilterExists(caches []*model.Cache) []*model.Cache {
	var out []*model.Cache
	for _, c := range caches {
		if c.Exists && c.SizeBytes > 0 {
			out = append(out, c)
		}
	}
	return out
}
