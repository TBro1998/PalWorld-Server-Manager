# 服务器目录与配置编辑

## Goal

让服务器卡片支持"编辑"，覆盖两类编辑能力：
1. 修改服务器安装目录（后端检测目标目录是否已安装服务端文件，未安装则返回标记，卡片提示用户手动安装）。
2. 全量编辑服务器启动参数与 `PalWorldSettings.ini` 配置参数（分组结构化表单 + 原始文本兜底）。

参考官方文档：
- 启动参数：https://docs.palworldgame.com/settings-and-operation/arguments
- 配置参数：https://docs.palworldgame.com/settings-and-operation/configuration

## Requirements

### R1: 卡片编辑入口
- `ServerCard` 增加"编辑"入口（编辑基础信息+目录）和"配置"入口（编辑 INI/启动参数）。
- 仅在服务器 `stopped` 状态可编辑目录与配置；`running` / `installing` 时禁用。

### R2: 修改安装目录
- 编辑对话框可修改 `install_path`。
- 保存时后端检测新目录是否已安装服务端可执行文件（`PalServer.exe` / `PalServer.sh`，复用 `serverExecutable` 判定逻辑）。
- `installed` 持久化到数据库，**仅在按需时机更新**：① 后端启动校正一次；② 修改服务器目录后；③ 安装完成后。不做每次请求的频繁探测。
- 未安装：更新 `install_path`，服务器 JSON 返回 `installed: false`；卡片显示"需要安装"并引导用户点击已有的安装按钮（复用现有 SteamCMD 安装流程）。
- 已安装：`installed: true`，卡片正常显示可启动。
- 仅 `stopped` 状态允许改目录。

### R3: 编辑启动参数（全量）
- 覆盖官方全部启动参数：`-port`、`-players`、`-useperfthreads`、`-NoAsyncLoadingThread`、`-UseMultithreadForDS`、`-NumberOfWorkerThreadsServer=X`、`-publiclobby`、`-publicip=x.x.x.x`、`-publicport=xxxx`、`-logformat=text|json`。
- 启动参数无 INI 归属，持久化到数据库；服务器启动时由进程管理器拼接到命令行。

### R4: 编辑配置参数（PalWorldSettings.ini 全量）
- 覆盖官方文档全部 `OptionSettings` 参数（Performances / Server management / Features / Game balances 四大类）。
- 直接读写磁盘 INI 文件：`<installPath>/Pal/Saved/Config/{WindowsServer|LinuxServer}/PalWorldSettings.ini`。
- 读取时若文件不存在：优先从 `<installPath>/DefaultPalWorldSettings.ini` 播种，否则用内置默认模板播种，再解析返回。
- 保存时序列化回 `[/Script/Pal.PalGameWorldSettings]` 段下的 `OptionSettings=(...)` 单行格式。
- UI：按官方分类分组的结构化表单（带类型控件）+ 一个"原始文本"兜底编辑区。
- **每个参数提供三语（zh/en/ja）label 与说明**（英文取官方文档描述，zh/ja 翻译）；参数 Key（写入 INI 的键名）保持英文。

## Constraints

- 改目录 / 改配置均要求服务器处于 `stopped`（运行时写 INI 无效且可能被覆盖）。
- 跨平台配置路径（`WindowsServer` vs `LinuxServer` 子目录，按 `runtime.GOOS`）。
- 保持单二进制架构：内置默认 INI 模板通过 `embed.FS` 打包。
- INI 解析需容忍单行 `OptionSettings` 的引号字符串与嵌套括号（如 `DenyTechnologyList`、`CrossplayPlatforms`），无法结构化的参数以原始字符串透传。
- 复用现有安装流程（`steamcmd.InstallPalworldServer`）与 `serverExecutable` 判定，不重复实现。

## Acceptance Criteria

- [ ] 卡片在 stopped 状态显示"编辑"与"配置"入口；running/installing 时禁用。
- [ ] 修改目录到未安装路径：保存成功，卡片显示"需要安装"，点击安装按钮触发 SteamCMD。
- [ ] 修改目录到已安装路径：`installed:true`，卡片显示可启动。
- [ ] 运行/安装中禁止改目录，后端返回明确错误。
- [ ] 启动参数保存后，服务器启动命令行正确包含所配置的参数（含布尔开关的取舍、带值参数的拼接）。
- [ ] 配置对话框加载出全量参数当前值；首次（无 INI）能从 Default 或内置模板播种。
- [ ] 结构化表单保存后，磁盘 `PalWorldSettings.ini` 的 `OptionSettings` 被正确回写，未修改项保持不变。
- [ ] 原始文本兜底可直接编辑并保存 `OptionSettings` 行。
- [ ] `DenyTechnologyList` / `CrossplayPlatforms` 等复杂值不被解析破坏（round-trip 保持）。
- [ ] 启动校正 / 改目录 / 安装完成三个时机正确更新 `installed`，其余请求不重复探测。
- [ ] 三语（zh/en/ja）文案齐全，含**全部参数的 label+说明**；前端重新构建并被 Go 嵌入。
- [ ] `go build .` 通过；`bun run build` 通过。

## Notes

- 技术设计见 design.md，执行步骤见 implement.md。
- 与既有任务 `steamcmd-server-lifecycle` 解耦：本任务复用其安装/生命周期实现，不改其核心逻辑（仅在 StartServer 拼接启动参数处扩展）。
