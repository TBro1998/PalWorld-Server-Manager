# 创意工坊 mod 搜索器

## Goal

在服务器管理的 Mods 区块内提供"创意工坊 mod 搜索器"，让用户无需手动查 Workshop ID 即可发现并添加 mod：
- 通过 Steam 官方 Web API 搜索 Palworld（App `1623730`）创意工坊 mod 列表，支持关键词搜索与分页；
- 列表项可点击跳转到该 mod 的创意工坊介绍网页；
- 列表项可一键添加到当前服务器的 mod 管理列表；
- 添加时若该 mod 存在未添加的前置 mod（依赖），弹窗提示并支持一键添加所有前置 mod。

用户价值：把"手抄 Workshop ID"改为"搜索即点即加"，并自动补全依赖，降低 mod 配置门槛与漏装依赖导致的加载失败。

## Background / Confirmed Facts

### 现有 mod 体系（代码事实）
- 下载走 SteamCMD `+workshop_download_item`（App `1623730`）：`internal/steamcmd/workshop.go:17,38`；落地 `<steamcmdPath>/steamapps/workshop/content/1623730/<id>/`。
- `Mod` 模型：`workshop_id`(业务键，每服务器唯一)、`name`、`enabled`、`package_name`/`mod_name`/`version`/`tags`（下载后从 Info.json 回填）：`internal/models/server.go:33-46`；前端类型 `ui/src/types/server.ts:29-42`。
- Mod API：`GET/POST /api/servers/:id/mods`、`DELETE /:modId`、`PUT /:modId/toggle`、`POST /mods/update`：`internal/api/router.go:101-109`。前端 `modsApi`：`ui/src/lib/api.ts:119-128`。
- **添加与下载分离**：`modsApi.add(id,{workshopId,name})` 只写 DB（同步 CRUD，见 `ModsSection.tsx:73-85`）；实际下载/部署由 `modsApi.update()` 触发（异步 SteamCMD，进度经 steamcmd 日志 SSE + `done` 具名事件，见 `mod-handling.md:60-76`）。
- 当前 UI 仅支持手填 Workshop ID + 名称：`ui/src/components/server-manage/ModsSection.tsx:151-181`；同区块含 `SteamAccountSection`（应用内登录）。
- Steam 全局账号能力与 settings：`GET /api/steam/status`、`POST /api/steam/login`、`GET /api/steam/logs/stream`（`internal/api/steam_handlers.go`）；`settings` 键值表 `internal/settings/settings.go`（`Get/Set`，键 `steam_username`/`steam_session_ready`）。

### Steam Web API（调研确认，来源见 design.md）
- `IPublishedFileService/QueryFiles/v1`（GET `api.steampowered.com`）搜索创意工坊：入参含 `key`(必需，标准 Web API Key)、`appid`、`search_text`、`query_type`、分页、`return_previews`、`return_short_description`、`return_tags`、`return_children` 等。
- `IPublishedFileService/GetDetails/v1` + `return_children=true` 返回子项/依赖 id（合集子项 + `AddDependency` 软依赖），是"添加前"解析前置 mod 的可靠途径。
- **标准 Steam Web API Key 是硬性前提**（QueryFiles 无 key 不可用）。

## Decisions（本轮确认）
- D1 数据源：Steam 官方 Web API（QueryFiles + GetDetails/return_children）。
- D2 API Key 来源：**UI 填写，后端存 `settings` 表**（键 `steam_web_api_key`），放在 Mods 区块 Steam 账号区域附近；未配置时禁用搜索并给出提示。需相应放宽 `internal/settings/settings.go` 顶部"从不存密码"的注释口径（Web API Key 为只读公开数据的低敏个人密钥）。
- D3 UI 落位：**弹窗/对话框**，由 Mods 区块的"浏览创意工坊"按钮打开。
- D4 添加行为：**仅写 DB 列表**（复用 `modsApi.add`），实际下载仍由现有"更新 mod"触发；与现有 add/update 分离设计一致。
- D5 依赖解析：**递归解析全部**前置（带深度上限与 visited 去重防环），并对"已在列表/本次已加"去重。

## Requirements

- R1（后端搜索代理）：新增后端端点代理 QueryFiles，key 留服务端，前端不直接请求 Steam。返回规范化结果（含 `workshop_id`、标题、简介、缩略图 URL、作者/热度、更新时间、tags）+ 分页游标。
- R2（前端搜索 UI）：弹窗内提供关键词输入、结果列表（缩略图/标题/简介/统计）、分页/加载更多；未配置 key 时禁用并提示。
- R3（跳转介绍页）：列表项可打开 `https://steamcommunity.com/sharedfiles/filedetails/?id=<workshopID>`（新标签页）。
- R4（一键添加）：列表项"添加"复用 `modsApi.add`（仅写 DB）；已在列表的项显示为"已添加"并禁用重复添加。
- R5（前置 mod 检测与一键补全）：添加时经后端递归解析该 mod 的 Workshop 依赖，前端对当前服务器 mod 列表去重，得出"缺失前置"集合；若非空，弹出确认框列出缺失前置，支持"一键添加全部"（逐个 `modsApi.add`）。无缺失则不弹窗。
- R6（Key 配置）：UI 输入框保存/清除 `steam_web_api_key`；`GET /api/steam/status` 增补 `webApiKeyConfigured` 布尔（不回显明文 key）；新增保存端点。
- R7（i18n）：所有新文案补齐 zh/en/ja 三语（沿用 `serverConfig`/`serverManage` 命名空间约定）。
- R8（错误与降级）：key 无效/配额/网络错误在 UI 明确提示；依赖无声明时优雅降级（无弹窗，正常添加）。

## Acceptance Criteria

- [ ] AC1：配置有效 Web API Key 后，在弹窗输入关键词能返回 Palworld 工坊结果并分页；未配置 key 时"浏览创意工坊"禁用且有提示。
- [ ] AC2：结果项点击外链能在新标签页打开对应 `filedetails` 页面。
- [ ] AC3：点结果项"添加"后，该 mod 出现在服务器 mod 列表（DB 持久化），弹窗内该项变为"已添加"。
- [ ] AC4：添加一个"在 Steam 层声明了依赖且依赖未添加"的 mod 时，弹出缺失前置列表；"一键添加全部"后所有缺失前置进入 mod 列表；已存在的依赖不重复添加；无缺失依赖时不弹窗。
- [ ] AC5：后端搜索/依赖端点：key 缺失返回明确错误码/信息；Steam 报错被规范化返回，前端可展示；key 明文不出现在 `GET /api/steam/status` 响应。
- [ ] AC6：递归依赖解析对环/深度有防护（visited + 深度上限），不产生无限递归或重复项。
- [ ] AC7：`go build .` 通过；后端新增纯逻辑（Steam 客户端解析/依赖递归）有单测；前端 `bun run lint` 通过。
- [ ] AC8：新增 UI 文案 zh/en/ja 三语齐全，无缺键。

## Out of Scope
- 从 Info.json 的 `Dependencies` 字段解析依赖（仅下载后可读，且多为 PackageName；本任务只做"添加前"的 Steam 层依赖）。
- 依赖的下载/部署本身（仍由现有"更新 mod"流程负责，本任务只负责把前置加入列表）。
- 收藏/合集（collection）作为整体导入（可后续迭代）。
- 服务端结果缓存/限流优化（MVP 直连，必要时后续加）。

## Risks / Assumptions
- 前置检测有效性取决于**mod 作者是否在 Steam 层声明了依赖**（`AddDependency`/合集 children）。Palworld 生态里若普遍改用 Info.json `Dependencies`（PackageName）而不声明 Steam 依赖，则弹窗可能很少触发——属数据可用性限制，非实现缺陷；以 Windows 真机对真实 mod 验证为准。
- QueryFiles 的 `query_type` 精确枚举（文本搜索排序）与 cursor 分页取值在实现时以一次小调研/真机请求最终敲定（design.md 记录候选值）。
