# Design — Linux 适配与 Docker 运行模式支持

## 1. 现状评估（关键结论）

Go 代码已基本跨平台，Linux 适配**不是重写**，而是补齐运行时缺口 + 打包：

| 关注点 | 现状 | 位置 |
|---|---|---|
| 进程组信号/优雅关闭/存活探测 | Linux 已实现 | `internal/process/platform_unix.go` |
| 服务器启动器解析 `PalServer.sh` | 已实现 | `internal/process/platform.go:15-31` |
| SteamCMD 可执行名 `steamcmd.sh` | 已实现 | `internal/steamcmd/steamcmd.go:78-83` |
| SteamCMD Linux tar.gz 下载 | 已实现 | `internal/steamcmd/download.go:18-20` |
| tar 解压保留文件权限（含 +x） | 已实现 | `internal/steamcmd/utils.go:130` |
| 打开浏览器 `xdg-open` | 已实现（best-effort） | `internal/server/browser.go:22` |
| 监听地址可配 `0.0.0.0` | 已支持（env `HOST`） | `internal/config/config.go:13` |

### 真正的缺口
1. **Palworld Linux 专用服运行依赖 `steamclient.so`**：Palworld 启动时按
   `$HOME/.steam/sdk64/steamclient.so` 查找 Steam 客户端库；缺失会导致启动失败或反复报错。
   这是"Linux 完整可运行"的硬需求，Windows 无此问题。
2. **容器/无头环境浏览器自动打开**：`xdg-open` 在容器不存在，仅打印 warning。可接受但应可关闭以保持日志干净。
3. **打包/运行时基础镜像**：需 glibc 基础镜像 + 32 位库（SteamCMD 为 32 位）。

## 2. 方案设计

### 2.1 Go 侧改动（最小、平台隔离）

#### (A) Steam SDK 软链接（新增，Linux 专属）
- 新增 `internal/steamcmd/steamclient_unix.go`（`//go:build !windows`）与
  `steamclient_windows.go`（`//go:build windows`，no-op）。
- 导出 `EnsureSteamClientLinks(steamcmdPath string) error`：
  - 计算 `home := os.UserHomeDir()`。
  - 创建目录 `~/.steam/sdk64`、`~/.steam/sdk32`。
  - 建立软链接：
    - `~/.steam/sdk64/steamclient.so` → `<steamcmdPath>/linux64/steamclient.so`
    - `~/.steam/sdk32/steamclient.so` → `<steamcmdPath>/linux32/steamclient.so`
  - 软链接允许指向"尚不存在"的目标（首启时 SteamCMD 尚未解包完成也无妨，链接在 SteamCMD 安装后即生效）。
  - 已存在正确链接则跳过；存在错误目标则重建。best-effort：失败仅告警不阻断（保持与 `runInitialUpdate` 一致的容错基调）。
- 调用点：`steamcmd.CheckAndInstall` 成功后调用 `EnsureSteamClientLinks`，使
  **原生 Linux 与容器都受益**（不把该逻辑塞进 entrypoint，避免 Docker-only）。
  - Windows 版 `EnsureSteamClientLinks` 返回 nil，`CheckAndInstall` 无差别调用。

理由：把运行时依赖修复放在 Go 层，原生 Linux 用户 `go build .` 直接跑也能用，Docker 只是复用。

#### (B) 移除应用内浏览器自动打开（改由调试脚本负责）
> 决策变更（用户明确要求）：真实工具不应有任何"打开浏览器"行为。该行为仅在
> 重建后调试时才需要，应由 `debug.bat` 负责，而非编进产品二进制。

- 删除 `internal/server/server.go` 的 `openWebUI()` 方法及其在 `Start()` 中的调用（并移除不再使用的 `time` 导入）。
- 删除 `internal/server/browser.go`（`openBrowser` + `xdg-open`/`rundll32`/`open` 分支整体移除）。
- 不新增任何 `OpenBrowser`/`OPEN_BROWSER` 配置项。
- `debug.bat`：在 `go run .` 前后台启动一个延时打开浏览器的命令
  （`start "" /b cmd /c "timeout /t 3 >nul & start "" http://127.0.0.1:8080/"`），仅用于本地调试便利。
- 影响：Docker/Linux/Windows 生产运行都不再尝试打开浏览器，日志更干净，并去掉了一处平台分支。

### 2.2 Docker 镜像设计（多阶段）

`Dockerfile`（三阶段）：

```
Stage 1 frontend  (oven/bun:1)
  WORKDIR /app/ui
  COPY ui/package.json ui/bun.lock ./
  RUN bun install --frozen-lockfile
  COPY ui/ ./
  RUN bun run build            # 产出 /app/ui/out

Stage 2 backend   (golang:1.26-bookworm)
  WORKDIR /app
  COPY go.mod go.sum ./        # go.sum 若不存在则改为仅 go.mod + GOFLAGS
  RUN go mod download
  COPY . .
  COPY --from=frontend /app/ui/out ./ui/out
  RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/psm .

Stage 3 runtime   (debian:bookworm-slim)
  RUN dpkg --add-architecture i386 && apt-get update && \
      apt-get install -y --no-install-recommends \
        ca-certificates lib32gcc-s1 libstdc++6 locales curl tzdata && \
      rm -rf /var/lib/apt/lists/*
  # 非 root 用户
  RUN useradd -m -u 10000 steam
  COPY --from=backend /out/psm /usr/local/bin/psm
  COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
  USER steam
  WORKDIR /data
  EXPOSE 8080/tcp 8211/udp 27015/udp
  ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
  CMD ["psm"]
```

要点：
- 运行镜像用 **debian slim（glibc）**，满足 Palworld 64 位二进制与 SteamCMD 32 位库需求。
- `CGO_ENABLED=0`：项目用纯 Go SQLite（`modernc.org/sqlite`），静态二进制，运行镜像无需 Go 运行时。
- SteamCMD 与游戏服**不预装进镜像**，由应用运行时下载到 `/data`（持久化卷），保持镜像小、升级游戏服不重建镜像。
- `go.sum`：仓库当前可能未提交 `go.sum`。构建阶段策略：若无 `go.sum`，`go mod download` 仍可运行但会生成之；为稳妥 `COPY go.mod ./` 后 `go mod download`（缺 go.sum 时 Go 会按 go.mod 拉取）。实现阶段先核实是否存在 `go.sum`。

### 2.3 entrypoint.sh
- 作用：确保 `/data` 下子目录存在（steamcmd/palworld/logs），随后 `exec "$@"`。
- Steam SDK 软链已由 Go 侧 `EnsureSteamClientLinks` 负责，无需在此重复；entrypoint 保持极薄，仅建目录 + `exec`。
- 以 `steam` 用户运行，`$HOME=/home/steam`，`EnsureSteamClientLinks` 落在 `/home/steam/.steam/...`。

### 2.4 配置与路径（容器内）
通过环境变量（无 config.yaml → 走 env 分支）：
- `HOST=0.0.0.0`，`PORT=8080`
- `DATABASE_PATH=/data/palworld.db`
- `STEAMCMD_PATH=/data/steamcmd`
- `PALWORLD_BASE_PATH=/data/palworld`
- `LOG_DIR=/data/logs`
- `JWT_SECRET`（用户必须改）

### 2.5 docker-compose.yml
```yaml
services:
  palworld-server-manager:
    build: .
    image: palworld-server-manager:local
    container_name: palworld-server-manager
    restart: unless-stopped
    ports:
      - "8080:8080/tcp"     # Web UI
      - "8211:8211/udp"     # 游戏
      - "27015:27015/udp"   # 查询
    environment:
      HOST: "0.0.0.0"
      PORT: "8080"
      JWT_SECRET: "change-me-in-production"   # ⚠ 请修改
      DATABASE_PATH: "/data/palworld.db"
      STEAMCMD_PATH: "/data/steamcmd"
      PALWORLD_BASE_PATH: "/data/palworld"
      LOG_DIR: "/data/logs"
    volumes:
      - psm-data:/data
volumes:
  psm-data:
```
- 端口：游戏/查询端口与 Palworld 默认一致；若用户改 `-port`/`-QueryPort`，需同步调整映射（文档说明）。

### 2.6 .dockerignore
排除：`.git`、`.trellis`、`ui/node_modules`、`ui/out`（镜像内重建）、`*.db*`、`logs/`、本地二进制、`config.yaml` 等，缩小上下文并避免把宿主机 config.yaml 带进镜像覆盖 env。

## 3. 兼容性与回归
- 所有平台差异用 build tag 或 `runtime.GOOS` 隔离；Windows 编译路径与行为不变（AC6）。
- 新增配置项默认值保证旧行为（浏览器默认打开）。
- 不动数据库 schema、API 契约、前端。

## 4. 风险
- R-1：Palworld 首启仍可能因个别缺失库报错（不同发行版差异）。缓解：runtime 装 `lib32gcc-s1 libstdc++6`，文档记录排查路径（看 `/data/logs` 与 `Pal.log`）。
- R-2：SteamCMD 首次自更新耗时/退出码非 0。现有代码已按 best-effort 容忍（`steamcmd.go:50-52`）。
- R-3：`go.sum` 缺失导致构建阶段行为差异。实现阶段先核实并在 Dockerfile 中处理。
- R-4：不同 compose 版本对 `udp` 端口写法。采用标准 `"host:container/udp"` 字符串形式，兼容性最好。

## 5. 验证方式
- 静态：`GOOS=linux GOOS=windows go build`（交叉编译两平台通过）、`go vet`。
- 动态（尽力，取决于本机 Docker 可用性）：`docker compose build`；如可运行则 `up -d` 后访问 8080、走安装→启动→停止链路核对 AC1-AC5。
- Windows：本机 `go build .` 仍通过（AC6）。
