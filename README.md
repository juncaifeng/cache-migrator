# cache-migrator

一个用 Go 编写的交互式 CLI 工具，用于扫描 Linux 系统上常见的开发缓存（Docker、Ollama、npm、Go、Cargo/Rustup、pip、uv 等），并把它们迁移到更大的磁盘。

## 解决的问题

很多机器有两块盘：

- 系统盘小（例如 200 GB 的 SSD）
- 数据盘大（例如 1 TB 的 HDD/SSD）

Docker、Ollama 模型、npm/Go/Rust 缓存默认都放在家目录或 `/var/lib` 下，很容易把系统盘撑满。`cache-migrator` 可以帮你：

1. 扫描这些缓存的实际位置和大小
2. 交互式选择要迁移的缓存
3. 选择目标磁盘
4. 自动移动数据并修改环境变量 / systemd / Docker 配置

## 支持的缓存

| 缓存 | 检测方式 | 迁移方式 |
|---|---|---|
| Docker | `/etc/docker/daemon.json` 的 `data-root` | 修改 `daemon.json`，提示重启 Docker |
| Ollama 模型 | `OLLAMA_MODELS` 或 `~/.ollama/models` | 修改 `ollama.service` 的 `Environment`，提示重启 Ollama |
| npm | `npm config get cache` | 设置 `npm_config_cache` 环境变量 |
| Bun | `bun pm cache dir` | 设置 `BUN_INSTALL_CACHE_DIR` |
| pnpm | `pnpm config get store-dir` | 设置 `PNPM_HOME` |
| Go | `go env GOPATH` | 设置 `GOPATH` 并更新 `PATH` |
| Cargo | `CARGO_HOME` 或 `~/.cargo` | 设置 `CARGO_HOME` 并更新 `PATH` |
| Rustup | `RUSTUP_HOME` 或 `~/.rustup` | 设置 `RUSTUP_HOME` |
| pip | `pip cache dir` | 设置 `PIP_CACHE_DIR` |
| uv | `uv cache dir` | 设置 `UV_CACHE_DIR` |
| Conda | `conda info --base` | 设置 `CONDA_ROOT` |

## 安装

```bash
cd cache-migrator
go build -o cache-migrator .
# 可选：放到 PATH
sudo cp cache-migrator /usr/local/bin/
```

纯标准库实现，无需下载任何 Go 模块依赖。

## 用法

### 扫描当前用户的缓存

```bash
./cache-migrator scan
```

### 扫描指定用户（需要 root 权限）

```bash
sudo ./cache-migrator scan --user wxchy
```

### 交互式迁移

```bash
./cache-migrator migrate
```

流程：

1. 显示检测到的缓存列表
2. 选择要迁移的编号（空格/逗号分隔，输入 `0` 全选）
3. 选择目标磁盘
4. 确认后开始迁移

### 指定目标目录迁移

```bash
./cache-migrator migrate --target /mnt/data
```

### 仅预览，不实际迁移

```bash
./cache-migrator migrate --target /mnt/data --dry-run
```

### 为其他用户迁移

```bash
sudo ./cache-migrator migrate --target /mnt/data --user wxchy
```

多用户场景下，缓存会自动按用户名分目录，例如：

- root 的 npm：`/mnt/data/cache/npm`
- wxchy 的 npm：`/mnt/data/cache/wxchy/npm`

## 迁移后注意事项

- **Docker / Ollama**：工具会更新配置文件，但需要手动重启服务：
  ```bash
  systemctl restart docker
  systemctl restart ollama
  ```
- **Shell 环境变量**：工具会写入 `~/.bashrc` / `~/.zshrc` / `~/.profile`。当前已打开的终端需要执行 `source ~/.bashrc` 或重新打开终端才能生效。
- **PATH 更新**：Go / Cargo 迁移后会自动把新 `bin` 目录追加到 `PATH`。

## 安全机制

- 迁移前会先检测目标路径是否已存在，避免覆盖。
- 如果缓存已经在目标盘下，会自动跳过。
- 跨文件系统迁移时使用“复制 + 删除源目录”策略，复制失败不会删除源数据。

## 项目结构

```
cache-migrator/
├── main.go                  # CLI 入口
├── go.mod
├── README.md
└── pkg/
    ├── model/               # 数据模型
    ├── scanner/             # 缓存检测
    ├── disk/                # 磁盘扫描
    ├── migrator/            # 迁移逻辑
    └── prompt/              # 交互式提示
```

## License

MIT
