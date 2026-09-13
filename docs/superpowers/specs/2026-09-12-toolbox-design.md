# Toolbox 设计文档

日期：2026-09-12
状态：已与需求方确认的设计稿

## 1. 背景与目标

一套个人自用的多媒体 AI 工具箱：

- **CLI**（cobra）：`toolbox <tool>` 直接执行，适合脚本化与快速调用。
- **Web**（gin + React）：可视化操作台，任务化交互 + WebSocket 进度推送，视觉上是有质感的产品而非 demo。
- 首批 4 个工具，均来自火山引擎，但架构预留接入任意其他平台（阿里云、讯飞等）。

首批工具：

| 工具 | 平台/服务 | 接口形态 | 凭证 |
|---|---|---|---|
| 语音合成 TTS | 豆包语音（openspeech） | HTTP 非流式 V1 / WebSocket 双向流式 V3 | APP ID + Access Token（或新版 API Key） |
| 语音识别 ASR | 豆包语音（openspeech） | 本地文件走 sauc nostream WS 直发；公网 URL 走录音文件识别三版本（标准异步 / 闲时 / 极速同步） | 同上 |
| 语音播客 | 豆包语音播客大模型 | WebSocket V3，流式事件返回 | 同上 |
| 人声背景音分离 | AI MediaKit | REST 异步：提交任务 → 轮询 | 独立 MediaKit API Key（Bearer） |

已确认的关键决策：

1. 定位：**个人自用工具**——无登录体系，单用户。
2. 数据库：**SQLite**（gorm），单文件零依赖。
3. 交互形态：**任务化为主，预留流式**——所有 Web 操作统一为提交任务 → WS 推进度 → 完成后播放/下载；WS 协议预留流式语义。
4. 架构：**单二进制内嵌任务引擎**（方案 A）——goroutine 任务池 + SQLite 状态 + go:embed 前端产物。
5. 前端：**React 19 + Vite + TypeScript**，亮暗**双主题**（暗色默认主打，亮色完整适配）。
6. 可扩展性：**Provider 抽象层**，新平台以实现接口 + 注册方式接入，前端 schema 驱动兜底。

## 2. 火山引擎接口要点（实现依据）

统一域名 `openspeech.bytedance.com`；语音三件套鉴权 headers：`X-Api-App-Key`(APP ID)、`X-Api-Access-Key`(Access Token)、`X-Api-Resource-Id`、`X-Api-Request-Id`(UUID)。新版控制台可改用 `X-Api-Key`。

### 2.1 TTS 语音合成
- HTTP 非流式：`POST /api/v1/tts`，返回完整音频。适合短文本与 CLI。
- WebSocket 双向流式 V3：`wss://.../api/v3/tts/bidirection`，文本流式输入、音频流式输出。首期任务化使用（服务端收完音频落盘），协议封装保留流式能力。
- 关键参数：`voice_type`（音色 ID）、语速/音量、`audio_params.format`（mp3/wav/pcm/ogg_opus）。
- 音色表：内嵌官方在线音色列表（6561/1257544）解析产物 `internal/provider/volcengine/voices.json`——2.0（含外语）+1.0 全量 500+ 条，字段 id/name/gender/scenes/languages/dialects/tags/emotions/generation/note；S2S/SC 端到端实时模型专用音色（jupiter/saturn 前缀）非 TTS 可用、已排除。Web TTS/播客页与 CLI `voices list` 均支持按场景/语种筛选；个别外语音色标注「仅支持单向流，双向流调用会直接报错」（notes 字段）。

### 2.2 ASR 语音识别
- 录音文件识别（异步 HTTP）：`POST /api/v3/auc/bigmodel/submit`（body 传音频公网 URL）→ `POST /api/v3/auc/bigmodel/query` 轮询。Resource-Id `volc.seedasr.auc`。上限 4 小时。
- 录音文件识别闲时版（6561/2608618 提交、6561/2608619 查询）：`POST /api/v3/auc/bigmodel/idle/submit` → `/api/v3/auc/bigmodel/idle/query`，Resource-Id `volc.bigasr.auc_idle`。仅收 `audio.url`（`audio.format` 必填、按 URL 扩展名推断），闲时算力执行、任务通常 24h 内完成；查询请求体为空 JSON、任务 ID 经 `X-Api-Request-Id` 头回传，`result` 非空即完成，X-Api-Status-Code 4 开头为终态错误、2/5 开头视为中间态继续轮询（工具层 24h 兜底，10s 起步退避至 2min）。
- 录音文件识别极速版（6561/2608628）：`POST /api/v3/auc/bigmodel/recognize/flash`，Resource-Id `volc.bigasr.auc_turbo`。同步返回完整识别结果（≤100MB / 2 小时），无需轮询。
- 三版本统一 `version` 参数：`standard`（默认）/ `idle` / `flash`；本地文件仅标准版可用（闲时/极速协议只收 URL）。闲时/极速版 language 位于 `audio` 对象（与 sauc WS 的 audio.language 一致），热词经 `request.corpus.context`（JSON 字符串 `{"hotwords":[{"word":"..."}]}`）直传。
- 识别（WebSocket，sauc 协议）：**本地文件识别走官方 sauc 协议（vendor 自 sauc_go demo）`bigmodel_nostream` 端点直发音频、全速分片**，绕开「火山访问不到本地文件」的问题，无需公网 URL。
- 产物：全文文本 + 分句（带时间戳），支持导出 TXT / SRT。
- 增值参数：热词（hotwords 直传）、上下文 context。

### 2.3 语音播客
- WebSocket V3：`wss://.../api/v3/sami/podcasttts`，Resource-Id `volc.service_type.10050`。
- 四种输入（对应请求 action / 字段）：
  1. `input_text`：长文本/主题，模型自动提炼为双人对话（默认，≤20000 字符，建议 ≤12000）；
  2. `input_url`：网页链接，联网总结后生成；
  3. `nlp_texts`：自备对话稿（逐轮指定 speaker + text）；
  4. prompt 模式：给定主题由模型生成大纲。
- 事件流：`PodcastRoundStart(360)` → `PodcastRoundResponse(361, 音频分片)` → `PodcastRoundEnd(362, 轮次时长/起止)` → `PodcastEnd(363, 完整音频 audio_url，1h 有效)`。`round_id == -1` 为开头音乐，`9999` 为结尾音频。
- `speaker_info.speakers`：恰好 2 个音色；音频格式 mp3/ogg_opus/pcm/aac，采样率默认 24000。
- 断点重试：连接中断后按已收轮次断点续传（文档原生支持）。
- 服务端在收到 PodcastEnd 后**立即下载临时 audio_url 转存本地**，避免 1h 失效。

### 2.4 AI MediaKit 人声背景音分离
- `POST https://mediakit.cn-beijing.volces.com/api/v1/tools/separate-voice`，`Authorization: Bearer <MediaKit API Key>`，body `{video_url|audio_url, scene: "Audio"|"Music"|"Drama"|"Narrate", output_format?}` → 返回 task_id。媒体字段二选一：按 URL 扩展名推断，常见视频扩展名走 `video_url`，其余走 `audio_url`。
- scene 四场景轨道数不同：Audio（通用，人声+背景）、Music（音乐，人声+伴奏）双轨；Drama（短剧）、Narrate（口播）三轨（人声+音乐+音效）。
- `output_format` 输出格式 aac/mp3/wav/m4a/flac：上游默认 aac，工具箱默认 mp3。
- `GET /api/v1/tasks/{task_id}` 轮询至 completed，输出多轨音频 URL——**24 小时有效的临时直链**，服务端 completed 后立即逐轨下载转存本地（与播客 1h audio_url 转存同策略，播客临时链接才是 1h）。
- 输入支持公网 URL（`https://`）。本地文件首期要求用户先提供可访问 URL；后续可评估 mediakit:// 本地上传通道。

## 3. 总体架构（方案 A：单二进制）

```
┌─────────────── 单个 Go 二进制 ───────────────┐
│  CLI (cobra)          Web (gin)             │
│      │                    │                 │
│      ▼                    ▼                 │
│   service 层（CLI/Web 唯一共享业务入口）      │
│      │                    │                 │
│      ▼                    ▼                 │
│   task 引擎：goroutine 池 + 状态机           │
│      │            （CLI 直调 service 同步执行）│
│      ▼                                      │
│   provider 抽象 → volcengine 实现            │
│      │                                      │
│      ▼                                      │
│   store (gorm+SQLite)   artifacts(文件系统)  │
└─────────────────────────────────────────────┘
```

- `toolbox serve`：启动 HTTP + WS，前端产物 go:embed。
- `toolbox tts/asr/podcast/separate`：CLI 同步执行，产物写 `--out` 或 data 目录，同时入库（历史在 Web 可见）。

## 4. 目录结构

```
toolbox/
├── main.go
├── cmd/toolbox/            # cobra 入口与子命令
├── internal/
│   ├── config/             # viper：~/.toolbox/config.yaml（凭证、端口、数据目录）
│   ├── provider/           # 平台无关抽象
│   │   ├── types.go        # Tool、ParamSpec、TaskInput、TaskOutput、Progress
│   │   ├── registry.go     # 注册表：工具发现、schema 导出
│   │   └── volcengine/     # 火山引擎 Provider
│   │       ├── provider.go # 凭证两组：语音(APP ID+Token) / MediaKit(API Key)
│   │       ├── auth.go     # 语音统一 headers
│   │       ├── tts.go      # HTTP V1 + WS V3 封装
│   │       ├── asr.go      # ASR：sauc nostream WS 直发 + submit/query 异步 + 工具编排
│   │       ├── sauc/       # 官方 sauc 协议包（vendor 自 sauc_go demo：header/payload/编解码）
│   │       ├── podcast.go  # 播客 WS 客户端、事件解析、断点重试
│   │       └── mediakit.go # 人声分离 REST 客户端
│   ├── service/            # 业务编排：CLI 与 Web 共用
│   ├── task/               # goroutine 池、状态机、SQLite 持久化、取消
│   ├── server/             # gin 路由、WS hub、embed 静态资源、Range 音频流
│   └── store/              # gorm 模型与查询
├── web/                    # React 19 + Vite 前端源码
├── skills/toolbox/         # agent skill（SKILL.md + references/cli.md），随仓库交付
└── data/                   # 运行时产物目录（默认 ~/.toolbox/data）
```

## 5. Provider 抽象（多平台扩展核心）

```go
type ParamSpec struct {
    Key         string        // 参数键
    Label       string        // 展示名（中文）
    Type        ParamType     // string | text | int | float | bool | enum | file
    Required    bool
    Default     any
    Options     []ParamOption // enum 的候选（如音色列表、scene）
    Placeholder string
    Group       string        // 表单分组
}

type Tool interface {
    Meta() ToolMeta                 // Provider、Name、标题、描述、图标、所属分组
    ParamSpecs() []ParamSpec        // 参数 schema：前端表单、CLI flag 共用
    Run(ctx context.Context, in TaskInput, p ProgressReporter) (TaskOutput, error)
}

type TaskInput struct {
    Params map[string]any   // 已校验参数
    Files  map[string]string // 上传文件落盘后的本地路径
}
type TaskOutput struct {
    Artifacts []Artifact // kind: audio | transcript | dialog | subtitle；含 format/duration
    Summary   map[string]any // 展示用摘要（如对话轮次数）
}
```

- 约束：provider 包不 import service/task/server；依赖单向（service→task→provider）。
- 新平台接入 = 新建 `internal/provider/<platform>/` 实现 Tool + 在 registry 注册；任务历史、进度推送、文件管理、历史页零改动；前端自动出现在通用工具页（schema 驱动表单），可后续升级定制页。
- 工具联动（跨工具传值）在 service 层实现：产物可「发送到」另一工具作为输入（如分离人声 → ASR）。

## 6. 数据模型（SQLite + gorm）

- `tasks`：`id`（UUID）、`provider`、`tool`、`status`（pending/running/succeeded/failed/canceled/interrupted）、`params`(JSON)、`progress`（0-100）、`progress_note`、`error`（映射后的中文消息 + 原始码）、`cost_ms`、`created_at/updated_at`。服务启动时将 running 置为 interrupted。
- `artifacts`：`id`、`task_id`(索引)、`kind`（audio/transcript/dialog/subtitle）、`path`（data 目录相对路径）、`filename`、`format`、`size`、`duration_ms`、`meta`(JSON，如 ASR 分句、播客轮次索引)。
- `settings`：`key`、`value`(JSON)——运行时可改项：默认音色、默认播客音色对、并发任务数。

凭证存 viper 配置文件 `~/.toolbox/config.yaml`（0600）：`volc.speech.app_id / access_token / api_key(可选)`、`volc.mediakit.api_key`、`server.port`、`data_dir`。

## 7. API 与 WebSocket 协议

REST（前缀 `/api`，响应统一包络：HTTP 一律 200，body `{"code":N,"data":...,"message":"..."}`；业务码与 CLI 退出码同一语义：0 成功、2 参数错误、3 任务/上游失败、4 凭证、5 内部、6 资源不存在。例外：产物流/下载端点为二进制流，不套包络，按真实 HTTP 语义）：

- `GET /api/tools` → 工具列表 + ParamSpec schema（前端渲染表单/CLI 生成共用）
- `POST /api/tasks` `{provider, tool, params, file_ids?, artifact_input?}` → `{task_id}`（上传走 `POST /api/uploads`，返回 file_id；上传仅用于服务端直发官方 nostream 端点的工具，如 ASR 本地文件）。`artifact_input`：既有产物 id，跨工具联动输入（如分离人声轨送 ASR）——服务端解析产物文件注入 `files["audio"]`，与 `file_ids` 互斥，产物文件须真实存在
- `GET /api/tasks?provider=&status=` 分页列表；`GET /api/tasks/:id`（含 artifacts）；`DELETE /api/tasks/:id`；`POST /api/tasks/:id/cancel`
- `GET /api/artifacts/:id/stream` 音频流（支持 HTTP Range，供播放器拖动）
- `GET /api/artifacts/:id/download` 附件下载（Content-Disposition）
- `GET /api/health`、`GET /api/settings` / `PUT /api/settings`、`POST /api/settings/test-connection`（凭证连通性测试）

WebSocket `GET /api/ws`，单一通道，JSON 消息：

```json
{"type": "task.progress", "task_id": "...", "progress": 42, "note": "第 3 轮对话生成中", "detail": {"round_text": "..."}}
{"type": "task.done",     "task_id": "...", "artifacts": [...]}
{"type": "task.error",    "task_id": "...", "error": "..."}
{"type": "task.canceled", "task_id": "..."}
```

协议按「消息类型可扩展」设计：未来流式试听新增 `task.chunk` 类型即可，不改既有语义。连接建立时服务端补发所有非终态任务的最新状态（防漏消息）。

## 8. CLI 设计

```
toolbox serve [--port 8080]
toolbox tts <text|--file> [--voice <id>] [--format mp3|wav] [--speed-ratio 1.0] [--volume-ratio 1.0] [--out path]
toolbox asr <file|--url> [--version standard|idle|flash] [--out text.txt] [--srt] [--hotwords "词1,词2"]
toolbox podcast <text|--file|--url> [--mode auto|script] [--script dialog.json]
                [--speakers id1,id2] [--format mp3] [--out path]
toolbox separate <url> [--scene audio|drama] [--out dir]
toolbox run <provider>.<tool> [--param key=value ...]   # schema 驱动的通用入口
toolbox config set <key> <value> / toolbox config list
toolbox voices list                                      # 音色列表查询与缓存
```

- 快捷命令与 `toolbox run` 共享 service；CLI 输出人类可读进度（TUI 进度条）。
- **机器可读契约（agent 集成，供 skill 调用）**：`--json` 时 stdout 只输出单个 JSON 结果对象（`task_id/provider/tool/status/cost_ms/artifacts[]/summary`），进度与告警一律走 stderr；退出码 `0` 成功、`2` 参数错误、`3` 任务失败、`4` 凭证缺失或无效。该契约由 `skills/toolbox`（随仓库交付的 agent skill，含 `references/cli.md` 完整命令参考）消费。
- 未配置凭证时给出明确指引（config set 命令示例）。
- 音色查询：`toolbox voices list` 拉取并本地缓存音色列表（分类/性别/语言），供 CLI 提示与 Web 音色选择器共用。

## 9. 前端设计（React 19 + Vite）

技术栈：React 19、TypeScript、Vite、React Router、Tailwind CSS、shadcn/ui（双主题定制）、TanStack Query（任务/列表/工具 schema）、Zustand（播放器与主题状态）、原生 WebSocket hook（自动重连）。

信息架构（左侧固定导航 + 顶栏）：

1. **工作台**：工具入口卡片（含累计用量统计）+ 最近任务动态。
2. **语音合成**：文本编辑区（字数、长文本提示）+ 参数面板（音色分组选择器、试听样本；语速/音量/格式）。生成 → 进度态 → 内嵌播放器 + 下载。
3. **语音识别**：拖拽上传或粘贴 URL，可选识别版本（标准/闲时/极速，后两者仅 URL）；结果按句展示（时间戳可点击跳播），导出 TXT/SRT。
4. **播客工坊**：三步向导——内容输入（主题/长文本/网页/对话稿 四模式）→ 双人音色搭配（预设组合）→ 生成页「对话流」逐轮滚动 + 进度环 + 已生成时长；成品播放器。
5. **人声分离**：输入音频/视频公网 URL（表单明确提示 MediaKit 需公网可访问地址，本地文件先上传对象存储）→ 四场景选择（通用/音乐双轨，短剧/口播三轨）+ 输出格式 → 多轨结果（每轨一行播放器、分别下载）；人声轨一键「送 ASR」（`artifact_input` 跨工具联动）。
6. **历史**：任务表格（类型/状态/耗时筛选），行内重播、下载、删除、同参重跑；产物均有下载入口。
7. **设置**：凭证配置 + 连接测试、默认参数、数据目录。

视觉规范（M6 已落地，实现细节以 [design-system/toolbox/MASTER.md](../../../design-system/toolbox/MASTER.md) 为准）：

- **设计方向**：把界面做成一台专业音频设备——近黑阳极面板、机架刻度线、琥珀信号色、等宽数字读数、刻印感微标签。判据：任何视觉选择都要能回答「这像不像录音棚里的设备」。
- 主题：CSS variables + Tailwind v4 `@theme` 语义工具类（`bg-panel`/`text-fg-2`/`border-line`，页面不再散写 `var()`）；暗色默认（`#0A0A0C` 画布 + `#0F0F12` 面板 + `#141418` 卡片 + 发丝分隔线），亮色完整适配；顶栏三态切换（跟随系统/暗/亮），localStorage 持久化。
- 色彩纪律：**琥珀 `#FF8A3D` 是唯一主操作色**（主按钮/激活态/焦点环）；绿 `#45D483` 只表示信号与成功/电平；红只表示错误与破坏性操作；禁止第二个装饰性强调色。
- 字体：**Fira Sans**（界面）+ **Fira Code**（全部数字读数：时间码/时长/耗时/计数），中文逐字形回退 PingFang SC / 微软雅黑；以 `@fontsource` 自托管（离线可用，不依赖 Google CDN）；数字一律 tabular-nums；字重仅 400/500/600。
- 图标：**Lucide** 统一 stroke 1.75，尺寸 16/20 两档；**禁止 emoji 作图标**（M1-M5 的 emoji 图标已全部替换）。
- 组件库：`web/src/ui/` 统一实现 Button/Field/Input/Select/Textarea/Card/Skeleton/EmptyState/StatusBadge/ProgressBar/Toast/Modal/ConfirmDialog/Tabs/PageHeader/WavePlayer；页面禁止手写卡片/按钮样式。
- 音频体验：自研 **WavePlayer**（WebAudio 解码真实峰值 + canvas 波形 + 已播段着色 + 点击拖拽定位 + mono 时间码）替代浏览器默认控件；**全局播放条**跨页面常驻，单实例音频通道保证同一时刻只播一路。
- 反馈与状态：Skeleton 同构骨架（内容区不用 spinner）、EmptyState 引导、Toast 替代全部 `alert()`、破坏性操作走 ConfirmDialog；仅「运行中」允许呼吸/流光动画，且尊重 `prefers-reduced-motion`。
- 布局：左侧固定导轨（激活态琥珀左标记）+ 顶栏（面包屑/API 健康灯/主题切换）；主区 `max-w-[1100px]` 居中；`<1024` 导轨收为图标态、`<768` 转抽屉；表单页 `≥1024` 两栏（编辑区 + 320px 参数面板）。

## 10. 错误处理

- 火山错误码统一映射为中文提示（鉴权失败/额度不足/内容审核/参数错误/限流），保留原始码入库。
- 播客 WS 断线按官方断点重试机制续传；任务级 context 取消贯穿 provider。
- ASR 异步轮询指数退避 + 上限；MediaKit 轮询同策略。
- 火山侧要求公网 URL 的输入（ASR 异步、MediaKit）：Web 表单明确提示，优先引导本地上传通道（ASR 服务端直发官方 nostream 端点）。
- 任务并发默认 2，可配置；上传文件大小限制可配置。

## 11. 测试策略

- provider 层：接口客户端单测（httptest / websocket mock）；真实凭证的冒烟用例隔离在 `integration` tag 后。
- task 引擎：状态机流转、并发、取消、interrupted 恢复的单测。
- server：路由与 WS 消息协议测试。
- 前端：核心交互组件测试 + `vite build` 通过即门禁；视觉以人工验收为准。

## 12. 里程碑

1. **M1 骨架** ✅：cobra + gin + gorm + config + embed 前端空壳跑通；tasks/artifacts 模型与任务引擎。
2. **M2 TTS 端到端** ✅：provider TTS（HTTP V1）→ service → REST/WS → 前端合成页 + 播放器 + 历史。CLI `toolbox tts`。
3. **M3 ASR** ✅：本地文件直发（官方 sauc nostream 协议）+ 异步 URL 通道；识别结果页（分句/时间戳）、TXT/SRT 导出。
4. **M4 播客** ✅：播客 WS 客户端（事件流、断点重试、audio_url 转存）；播客工坊页面。
5. **M5 MediaKit 人声分离** ✅ + 工具联动（分离→ASR，`artifact_input` 通道）。
6. **M6 产品化** ✅：设计系统（design-system/MASTER.md）、组件库、全站页面重做、WavePlayer 与全局播放条、双主题与响应式审计、`make dist` 交叉编译发布。

依赖技术清单（Go）：gin、gorm(+sqlite driver)、cobra、resty（HTTP 客户端）、viper、gorilla/websocket。
前端：React 19、Vite、Tailwind CSS v4、TanStack Query、Zustand、Lucide、Fira Sans/Code（@fontsource 自托管）。
