# Implement — Linux 适配与 Docker 运行模式支持

执行顺序自上而下。每个 checkpoint 后运行对应验证命令。

## 0. 预检（只读）
- [ ] 确认是否存在 `go.sum`：`ls go.sum`。若无，Dockerfile backend 阶段用 `COPY go.mod ./ && go mod download`（Go 会据 go.mod 解析）。
- [ ] 确认 `ui/bun.lock` 存在（已确认）。
- 验证：无（信息收集）。

## 1. Go：Steam SDK 软链接（Linux 完整可运行核心）
- [ ] 新增 `internal/steamcmd/steamclient_unix.go`（`//go:build !windows`）：
      导出 `EnsureSteamClientLinks(steamcmdPath string) error`，创建
      `~/.steam/sdk64/steamclient.so`→`<steamcmd>/linux64/steamclient.so`、
      `~/.steam/sdk32/steamclient.so`→`<steamcmd>/linux32/steamclient.so`。
      用 `filepath.Abs` 解析目标；`os.MkdirAll` 建父目录；已是正确链接则跳过，错误目标则 `os.Remove` 后重建。best-effort。
- [ ] 新增 `internal/steamcmd/steamclient_windows.go`（`//go:build windows`）：`EnsureSteamClientLinks` 返回 nil。
- [ ] 在 `internal/steamcmd/steamcmd.go:CheckAndInstall` 成功返回前调用
      `EnsureSteamClientLinks(steamcmdPath)`，失败仅 `fmt.Printf("Warning: ...")` 不阻断。
- 验证：
  - `GOOS=linux go build ./...`
  - `GOOS=windows go build ./...`

## 2. Go：移除应用内浏览器打开逻辑（改由 debug.bat 负责）
> 决策变更：真实工具不打开浏览器；该行为仅调试用。
- [x] 删除 `internal/server/server.go` 的 `openWebUI()` 及 `Start()` 中调用，移除多余 `time` 导入。
- [x] 删除 `internal/server/browser.go`。
- [x] 不新增 `OPEN_BROWSER` 配置。
- [x] `debug.bat`：`go run .` 前后台延时打开 `http://127.0.0.1:8080/`。
- 验证：`GOOS=windows go build .`、`CGO_ENABLED=0 GOOS=linux go build .` 均通过。

## 3. Docker 构建产物
- [ ] 新增 `Dockerfile`（三阶段：oven/bun → golang:1.26-bookworm → debian:bookworm-slim）。
      - backend 阶段 `CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/psm .`
      - runtime 阶段：`dpkg --add-architecture i386` + 安装 `ca-certificates lib32gcc-s1 libstdc++6 locales tzdata`；`useradd steam`；`USER steam`；`WORKDIR /data`；`EXPOSE 8080/tcp 8211/udp 27015/udp`。
- [ ] 新增 `docker/entrypoint.sh`：`set -e`；`mkdir -p /data/steamcmd /data/palworld /data/logs`；`exec "$@"`。置可执行位（`COPY --chmod=0755` 或 Dockerfile `RUN chmod`）。
- [ ] 新增 `.dockerignore`：`.git .trellis ui/node_modules ui/out *.db* logs/ *.exe psm config.yaml dist/ bin/`。
- 验证：`docker build -t psm:local .`（若本机 Docker 可用）。

## 4. docker-compose
- [ ] 新增 `docker-compose.yml`（见 design 2.5）：build、ports(tcp/udp)、environment、named volume `psm-data:/data`、`restart: unless-stopped`。
- 验证：`docker compose config`（语法校验）；条件允许则 `docker compose up -d --build` 后 `curl -I http://localhost:8080`。

## 5. 文档
- [ ] README（或新增 `docs/DEPLOY.zh.md`，就近选择）补充：
      - Docker 一键部署步骤、必改 `JWT_SECRET`、端口与 `-port/-QueryPort` 对应关系、数据卷位置与备份。
      - Linux 原生部署简述（依赖：glibc、`lib32gcc-s1`）。
- 验证：人工通读。

## 6. 全量检查（Phase 2.2）
- [ ] `gofmt -l` 无输出；`go vet ./...` 通过。
- [ ] `GOOS=windows go build .` 与 `GOOS=linux go build .` 均通过。
- [ ] 对照 PRD 验收 AC1–AC7 逐条核对（动态项受 Docker 可用性限制，说明实际执行到的程度）。

## 回滚点
- 每一节为独立提交单元；Go 改动（§1、§2）与 Docker 产物（§3–§5）解耦。
- 如 Docker 构建受阻，§1/§2 的 Linux 原生改动仍可独立交付并验证。

## 验证命令速查
```bash
gofmt -l internal main.go
go vet ./...
GOOS=linux  go build ./...
GOOS=windows go build ./...
docker build -t psm:local .        # 条件允许
docker compose config
```

## 验证结果（2026-07-14 实测）
- ✅ `GOOS=windows go build .` 通过；`CGO_ENABLED=0 GOOS=linux go build .` 通过；`go vet` 通过；新增 go 文件 gofmt 干净。
- ✅ `docker build` 成功，产出镜像约 147MB；移除 `ENV JWT_SECRET` 后 `SecretsUsedInArgOrEnv` 告警消除。
- ✅ 容器冒烟测试：SteamCMD 自动下载+自更新成功 → 日志 `SteamCMD installed successfully`；
  `~/.steam/sdk64|sdk32/steamclient.so` 软链接由 `EnsureSteamClientLinks` 正确建立（指向 `/data/steamcmd/linux64|linux32/steamclient.so`）；
  Web 服务 `Server starting on http://0.0.0.0:8080`（AC1/AC7 验证；AC2/AC3 因需完整下载游戏服未在冒烟中跑完，逻辑链路与原生一致）。
- ✅ `docker compose config` 语法通过。
- 备注：构建上下文一度达 11GB（仓库根目录存在已安装的 steamcmd/Servers/temp 运行时数据），已通过 `.dockerignore` 排除 `/Servers /steamcmd /temp /palworld /psm-data` 修复。
