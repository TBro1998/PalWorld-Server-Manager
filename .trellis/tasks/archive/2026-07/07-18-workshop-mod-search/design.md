# Design — 创意工坊 mod 搜索器

## Architecture Overview

```
前端 WorkshopBrowserDialog ──HTTP──> 后端 /api/steam/workshop/*  ──HTTPS──> Steam Web API
        │                                    │                         (QueryFiles / GetDetails)
        │ (添加)                             │ (key from settings 表)
        └──> modsApi.add (现有) ─────────────┘
```

- key 只存/用在服务端；前端永不接触明文 key，永不直连 Steam（避免泄 key + CORS）。
- 搜索/依赖解析是**无副作用的读操作**；添加复用现有 `POST /api/servers/:id/mods`（仅写 DB）。
- 依赖去重（对当前服务器 mod 列表）在**前端**做：后端只返回该 mod 的完整递归 Steam 依赖（纯 Steam 数据、与具体服务器无关），前端用已加载的 `mods` 列表算"缺失前置"。这样后端依赖端点保持无状态、可缓存、易测。

## Backend

### 新增包 `internal/steamworkshop`（纯逻辑，可单测，无 DB 依赖）
与 `palsave`/`palconfig`/`palmod` 同构：HTTP 客户端 + 响应解析 + 递归依赖，不依赖 models/db。

- `const AppID = "1623730"`（与 steamcmd 的 `palworldClientAppID` 同值；注释交叉引用，避免第二处硬编码语义漂移）。
- 类型：
  - `Item{ WorkshopID, Title, Description, PreviewURL, Author(creator), Subscriptions/Favorited, Views, TimeUpdated, Tags []string }`
  - `SearchResult{ Items []Item, NextCursor string, Total int }`
- `Search(ctx, httpClient, key, query string, cursor string, numPerPage int) (SearchResult, error)`
  - GET `https://api.steampowered.com/IPublishedFileService/QueryFiles/v1/`
  - 参数：`key`、`appid=1623730`、`search_text=<query>`、`query_type`（文本搜索排序；候选 `12`=RankedByTextSearch，实现时以真机请求确认）、`numperpage`、`cursor`（首页 `*`，响应回 `next_cursor`；若该 query_type 不支持 cursor 则退回 `page`）、`return_previews=true`、`return_short_description=true`、`return_tags=true`、`return_metadata=true`。
  - 解析 `response.publishedfiledetails[]` → `Item`；`response.next_cursor`/`response.total`。
- `GetDetails(ctx, httpClient, key string, ids []string) ([]DetailItem, error)`
  - GET `IPublishedFileService/GetDetails/v1/`，数组参 `publishedfileids[0]=..&publishedfileids[1]=..`，`return_children=true`（可加 `return_short_description`/`return_previews` 供依赖弹窗展示名称/缩略图）。
  - `DetailItem` 含 `Children []string`（`children[].publishedfileid`）+ 展示字段。
- `ResolveDependencies(ctx, httpClient, key, rootID string, maxDepth int) ([]DepItem, error)`
  - BFS/DFS：从 `rootID` 的 children 出发，`visited` 集合防环，`maxDepth`（如 5）兜底；对每层未见过的 id 批量 `GetDetails(return_children)` 继续下探；聚合成扁平**去重**列表（不含 root 本身），每项带展示信息。
  - 纯逻辑（httpClient 注入）→ 可用 `httptest.Server` 单测递归/去重/防环。

### settings 扩展
- `internal/settings/settings.go` 加 `KeySteamWebAPIKey = "steam_web_api_key"`；放宽顶部注释口径（说明 Web API Key 为低敏只读个人密钥，可存；密码仍绝不存）。

### API handlers（`internal/api`，挂到现有 `steam` 组）
`internal/api/router.go` 的 `steam := protected.Group("/steam")` 下新增：
- `GET  /steam/workshop/search?q=&cursor=&num=` → `WorkshopSearch`
  - 读 key（settings 优先，空则 `config` 兜底若将来加；MVP 只读 settings）；空 key → `400/409 {error:"web_api_key_missing"}`。
  - 调 `steamworkshop.Search`，成功 200 返回 `{items, nextCursor, total}`；Steam 失败 → `502 {error:...}`（规范化，不透传原始 key）。
- `GET  /steam/workshop/mods/:workshopId/dependencies` → `WorkshopDependencies`
  - 调 `steamworkshop.ResolveDependencies`，返回 `{dependencies: DepItem[]}`（完整递归、去重、不含自身）。
- `POST /steam/webapi-key`（body `{key}`）→ `SetWebAPIKey`：`settings.Set(KeySteamWebAPIKey, trim(key))`；空串=清除。返回 `{configured:bool}`。
- 扩展 `SteamStatus`：响应加 `webApiKeyConfigured: <settings 有非空 key>`（**不回显明文**）。

超时：给 Steam 请求 `context.WithTimeout`（如 15s），复用 `net/http` client（可包内单例或注入）。

## Frontend

### `ui/src/lib/api.ts`（`steamApi` 扩展）
- `status()` 返回类型加 `webApiKeyConfigured: boolean`。
- `workshopSearch(params:{q:string; cursor?:string; num?:number})` → `{items: WorkshopItem[]; nextCursor: string; total: number}`。
- `workshopDependencies(workshopId:string)` → `{dependencies: WorkshopDep[]}`。
- `setWebApiKey(key:string)` → `{configured:boolean}`。

### 类型（`ui/src/types/server.ts`）
- `WorkshopItem{ workshop_id, title, description, preview_url, author, subscriptions, views, time_updated, tags }`
- `WorkshopDep{ workshop_id, title, preview_url }`（弹窗展示用最小集）

### 组件
- `ui/src/components/server-manage/WorkshopBrowserDialog.tsx`（新）：
  - 受控 `open`；搜索输入（防抖）+ 结果列表（缩略图/标题/简介/统计）+ 外链按钮 + "添加"按钮 + 分页/加载更多（cursor）。
  - "添加"流程（复用 R4/R5）：
    1. `modsApi.add(serverId, {workshopId, name: title})`；
    2. `steamApi.workshopDependencies(workshopId)` → 与当前 `mods`（父组件传入或 query）算 `missing = deps 去除 已在列表/刚添加`；
    3. `missing.length>0` → 打开 `MissingDepsDialog` 列出缺失前置，"一键添加全部" 逐个 `modsApi.add`；
    4. 全程 `invalidateQueries(['mods', serverId])` 刷新列表；错误 toast/inline。
  - 未配置 key：整个入口按钮禁用 + 提示（在 `ModsSection` 层判断 `webApiKeyConfigured`）。
- Web API Key 配置：在 `SteamAccountSection`（或其下方新增一行）加"Web API Key"输入 + 保存/清除，调 `setWebApiKey`，用 `webApiKeyConfigured` 显示已配置态。key 为敏感值 → 用 `PasswordInput`（现有）遮挡；不预填明文（后端不回显）。
- `ModsSection.tsx`：在 add 表单区加"浏览创意工坊"按钮打开 `WorkshopBrowserDialog`，透传 `server` 与 `mods`。

### i18n（`ui/messages/{zh,en,ja}.json`）
在 `serverConfig`（或 `serverManage`）下新增：`workshop.browse/searchPlaceholder/add/added/openPage/empty/loading/loadMore/noKey/keyLabel/keySave/keyCleared/deps.title/deps.hint/deps.addAll/deps.none/errors.*` 等。三语齐全。

## Cross-Layer 一致性（对照 guides/cross-layer）
- 新增字段贯穿：Go 响应 struct（json snake_case）→ 前端 `types/server.ts` 接口 → i18n 文案，四处对齐（mod-handling.md:93 的既有约定）。
- Steam 返回字段命名/大小写不稳 → 后端**在 `steamworkshop` 内规范化**为固定 struct，前端只吃规范化结果（不在 UI 里 cast 原始 Steam JSON）。

## 关键取舍
- 依赖去重放前端：后端依赖端点无状态、可复用/可缓存；前端本就有 `mods` 列表，去重成本低。
- 添加=仅写 DB：严格对齐现有 add/update 分离（mod-handling.md:60），避免在搜索器里引入下载编排的并发/错误复杂度。
- 递归解析在后端一次算完：减少前端多次往返；`visited`+`maxDepth` 防环。

## 参考来源
- IPublishedFileService（QueryFiles/GetDetails 参数、return_children）：Steamworks 官方文档 `partner.steamgames.com/doc/webapi/IPublishedFileService`；社区参考 `steamapi.xpaw.me`。
- 依赖解析范式：社区 SteamWorkshopDependenciesResolver（GetDetails + return_children 递归）。
