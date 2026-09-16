# 对象存储中转（TOS 优先，S3 兼容预留）设计

日期：2026-09-16
状态：已实现（本 spec 为随附设计存档）
官方 SDK：`github.com/volcengine/ve-tos-golang-sdk/v2`（V4 签名，ClientV2）

## 背景与定位

语音识别（闲时/极速版）、人声分离（AI MediaKit）、语音妙记的上游接口**只收公网 URL**，
此前本地文件必须手动上传到对象存储再贴 URL。本设计在任务链路里内置这条中转：
**本地文件在任务执行时自动 Put 到对象桶，换取预签名 GET URL 提交上游**；
公网 URL 输入保持原样直用。桶内对象按生命周期规则（默认 3 天）到期自动清理。

## 架构决策

- **服务端中转而非 Web 直传**：本工具是本机自用形态（浏览器与服务同机），浏览器直传省不下
  带宽，却要求用户给桶配 CORS；服务端中转复用既有 `/api/uploads` 通道，Web/CLI/MCP 三入口
  同享。官方 Web 直传（PostObject 预签名）留作公网部署时的优化路径。
- **桥接在工具层而非引擎层**：`ensureURLInput`（internal/provider/volcengine/storage_bridge.go）
  由各 URL-only 工具在 Run 内调用——一句话识别继续本地直发、标准版+文件保持旧的
  「降级为一句话」语义，桥接只作用于真正需要 URL 的分支；凭证校验先于转存，无凭证不白传大文件。
- **通道注入在引擎层**：`provider.TaskInput` 增加 `Storage StorageClient`（窄接口
  Put/PresignGet），引擎每任务启动时经 getter 热取（`Engine.SetStorageClient`），
  Web 保存配置后下一任务即用新通道，进行中任务不受影响。

## 包结构

- `internal/objectstorage`：Provider 无关抽象 + TOS 实现
  - `Client` 接口：Put / PresignGet / Ping（HeadBucket 探活）/ ApplyLifecycle
    （**合并语义**：GetBucketLifecycle → 换掉本工具规则 ID `toolbox-auto-clean` → Put 回，
    桶上用户自建规则原样保留；无规则时 TOS 404 按空集处理）
  - 对象 key：`<prefix>/<yyyyMMdd>/<uuid>.<ext>`——扩展名保留是硬约束（上游 format 由
    URL 扩展名推断）；预签名 TTL 72h 对齐 3 天生命周期（SDK 上限 7 天）
  - endpoint 原样交 SDK（其按 `http://`/`https://` 前缀推断协议，裸 host 默认 HTTPS）；
    `InsecureSkipTLSVerify` 仅给自签私有化部署（MinIO 类），不入 config
  - `New()` 的 provider 分派已预留 `"s3"`（OSS/腾讯 COS 均兼容 S3 协议，接实现即可）
- `internal/config`：`storage` 段（provider/endpoint/region/bucket/access_key/secret_key/
  prefix/lifecycle_days），维持 config.yaml 单一事实来源（与词典资产同理不迁 DB）
- `internal/service`：客户端重建与热更新（SaveStorage / ReloadDiskConfig）、探活、
  生命周期应用；**坑**：SaveStorage 除重建客户端外必须把新值写回 cfg 原子快照，
  否则设置页 GET 回显为空（E2E 抓出过）

## API

- `GET /api/settings`：新增 `storage` 段（secret 不回传，回传 `has_secret_key`；
  `enabled` = 客户端就绪，前端各工具页据此开关「本地上传」通道）
- `PUT /api/settings`：可选 `storage` 段整表保存（secret_key 留空=不修改；
  provider 留空=停用；校验 tos 必填字段）；与凭证字段同请求时存储先行、失败不改凭证
- `POST /api/settings/test-connection`：新增 `storage` 段（HeadBucket，不计费）
- `POST /api/storage/lifecycle`：按「前缀 + 天数」应用桶规则（days≤0 回落配置值）

## 工具侧行为变化

| 工具 | 变化 |
|---|---|
| asr | 标准/闲时/极速版均接受本地文件，三版本格式白名单一致（wav/mp3/ogg/spx/amr/aac/m4a，转存前拦截；大小上限极速 100MB、闲时 512MB、标准交服务端裁决）；标准版+文件在配置存储后走**真标准版异步**，未配置存储保持降级一句话（历史兼容）；ParamSpecs 的 url 转「URL 或文件」语义 |
| separate | url 参数 Required 取消（引擎校验放行，Run 内合并校验）；凭证校验先于转存 |
| minutes | 同上；文件大小前端预校验 <1G |
| 工作台批量识别 | 输入区双通道（URL 列表 / 本地文件 ≤20 个，FileDrop multiple 模式），逐文件上传建任务 |
| CLI | `separate`/`minutes` 新增 `--file`（与 URL 参数互斥）；`asr` 放开 file+version 组合限制（裁决交给工具层按存储可用性给出） |

## 前端

- 设置页：独立「对象存储 · 本地文件中转」表单（provider=未启用/TOS、endpoint/region/bucket/
  AK/SK/前缀/生命周期天数）+ 保存、应用自动清理规则两个动作；连通性测试卡加对象存储徽标。
  独立 form 是刻意的：凭证字段是「留空不修改」语义，存储字段是「表单即真相」语义，混合会互相污染。
- separate / minutes：输入区 Tabs（音视频 URL / 本地上传，后者仅在存储 enabled 时出现），
  复用 `components/FileDrop`；ASR：闲时/极速版增加本地上传 Tab（扩展名白名单按版本区分），
  标准版维持 URL-only（避免静默降级困惑），版本切换清空已选文件。

## 验证

- 单测：TOS 客户端对 httptest（Put 内容/Content-Type、预签名 URL 形态、生命周期合并/空集、
  403 中文翻译）；`ensureURLInput` 六分支；ASR 闲时+文件经 fake storage 全链（断言提交上游的
  URL 与 source/version）；引擎注入三态（nil→注入→getter 返回 nil）。
- E2E（临时 HOME + 18111 端口 + 假 TOS httprv）：设置保存/回显/重启加载、test-connection
  storage 段、生命周期应用（真 SDK 协议）、`/api/uploads` → separate 任务 → 假 TOS 收到对象
  → 进度「转存完成，已取得签名 URL」→ MediaKit 假凭证 403 报错清晰；UI 三页 Tabs 与
  版本联动截图核对。

## 已知边界

- 真 TOS 通道的端到端上传需用户真实 AK/SK 与已开通服务（与 ASR WS 403 同理，账号侧待确认）。
- 转存进度为心跳式（每 5s 一跳），不做字节级进度（SDK PutObjectV2 无进度回调，UploadFile
  才有；500MB 内可接受）。
- 上游拉取签名 URL 的时效按 72h 计，闲时任务 24h 排队 + 处理完全覆盖；对象 3 天后随
  生命周期清理，与「处理完即无用」对齐。
