# 长文本语音合成（异步 submit/query）设计

日期：2026-09-13
状态：已实现（本 spec 为随附设计存档）

## 背景与定位

现有 `tts` 工具对接的是火山**旧版 HTTP V1 同步接口**（`/api/v1/tts`，`volcano_tts` 集群，
Bearer Token 鉴权，音频 base64 随响应返回），单次请求文本量小，超 1000 字符由本地
`splitText` 分段循环合成后拼接。

本次新增**异步长文本语音合成**（官方文档 `/api/v3/tts/submit` + `/api/v3/tts/query`）：
- 异步任务模式：submit 提交任务 → 轮询 query → task_status=2 时返回 `audio_url`（1 小时有效）
- 最大 10 万字符；非法 ASCII 控制字符（不含 \t \n）占比 >10% 拒绝执行
- 鉴权：`X-Api-Key`（新版）或旧版 `X-Api-App-Key` + `X-Api-Access-Key`（与 ASR v3 异步同构）
- 资源：`seed-tts-2.0`（豆包合成大模型 2.0 音色）/ `seed-icl-2.0`（声音复刻音色）
- 查询结果含分句与字级时间戳，可产出 SRT

两个接口是不同代际（参数语义、格式集、鉴权、模型系都不同），因此**新增独立工具
`tts_long`（长文本语音合成）**，与 `tts` 并存，而不是给 `tts` 加版本参数。
（ASR 的 version 参数先例适用场景是「同一批参数、不同后端」；此处参数集不可共用。）

## 架构

沿用既有分层：volcengine Client（纯协议）→ Tool（参数/轮询/产物）→ Registry →
Engine（任务池/落库/事件）→ REST 统一包络 + CLI。无新端点、无新表、无新凭证。

```
toolbox tts-long / Web /tts-long
  └─ Engine.Submit("volcengine", "tts_long")
       └─ TTSLongTool.Run
            ├─ TTSLongClient.Submit  → task_id
            ├─ poll（2s 起步指数退避，上限 15s，总超时 30min）
            ├─ TTSLongClient.Query   → audio_url + sentences
            ├─ TTSLongClient.Download → data/tts_long/<uuid>.<ext>
            └─ （开启时间戳且分句非空）BuildSRT → <uuid>.srt
```

### TTSLongClient（tts_long_client.go）

- 复用 `sauc.NewAuthHeaderFrom`：APIKey 优先，否则 AppID+AccessToken；带
  `X-Api-Resource-Id` 与 `X-Api-Request-Id`（UUID）。
- `Submit(ctx, TTSLongSubmitReq)`：POST `/api/v3/tts/submit`。
  body：`user.uid="toolbox"`、`user.unique_id`（=X-Api-Request-Id，响应 task_id 即取该值）、
  `req_params{text, model?, speaker, audio_params{format, sample_rate, bit_rate?,
  speech_rate, loudness_rate, enable_timestamp}, explicit_language?, aigc_watermark?,
  post_process{pitch?}}`。
  成功判定：body `code == 20000000`；错误消息附响应头 `X-Tt-Logid` 便于反馈定位。
- `Query(ctx, taskID)`：POST `/api/v3/tts/query`，body `{"task_id":...}`。
  返回归一化状态 Running(1)/Success(2)/Failure(3)，data 含 audio_url、
  sentences[{text,startTime,endTime,words…}]（秒，float64）、req_text_length、
  synthesize_text_length、url_expire_time。
- `Download(ctx, url)`：GET audio_url（1h 有效，查到即取）。

### TTSLongTool（tts_long_tool.go）

ParamSpecs（CLI flag 与 /api/tools schema 同源）：

| 参数 | 类型 | 默认 | 说明 |
|---|---|---|---|
| text | text | 必填 | ≤100000 字符（rune 计）；控制字符（除 \t\n）占比 >10% 报参数错误 |
| voice | string | zh_female_vv_uranus_bigtts | 2.0/复刻音色 ID（voices list，generation=2.0） |
| format | enum | mp3 | mp3/pcm/ogg_opus（无 wav） |
| sample_rate | enum | 24000 | 8000/16000/22050/24000/32000/44100/48000；ogg_opus 强制 48000 |
| speech_rate | int | 0 | -50~100（100=2 倍速） |
| loudness_rate | int | 0 | -50~100 |
| timestamps | bool | false | 开启后 query 返回分句/字级时间戳，产出 SRT 字幕 |
| resource | enum | seed-tts-2.0 | seed-icl-2.0（复刻音色） |
| model | string | 空 | 复刻音色时透传 req_params.model |
| explicit_language | enum | 空 | zh-cn/en/es-mx/id/pt-br |
| pitch | int | 0 | -12~12（post_process） |
| bit_rate | enum | 空(服务端默认) | 64000/160000；pcm 不支持 |
| aigc_watermark | bool | false | 音频结尾 AIGC 节奏标识 |

预检（参数错误，退出码 2）：text 空 / 超 10 万字 / 控制字符占比 >10% / ogg_opus+非 48000 /
bit_rate+pcm。凭证校验 `SpeechCred.Validate()`（APIKey 或 AppID+Token 双轨均可）。

产物：`tts_long/<uuid>.<ext>`（audio）；timestamps 且 sentences 非空 → 同名 `.srt`
（复用 BuildSRT，秒→ms 换算）。Summary：char_count(req_text_length)、
synthesized_chars(synthesize_text_length)、upstream_task_id、sentences 数。
`_out` 重定向契约与既有工具一致。

轮询节奏（var 注入测试）：起步 2s、退避上限 15s、总超时 30min；
每次上报 progress（30→90 缓升）与 detail{task_id, poll, status}。
Success 但无 audio_url 视为失败；Failure 原样透传 message。

### CLI（tts-long）

`toolbox tts-long <text>`：--file --voice --format --sample-rate --speech-rate
--loudness-rate --timestamps --resource --model --explicit-language --pitch
--bit-rate --aigc-watermark --out --json。复用 runToolSync（进度走 stderr、--json 纯 stdout）。

### Web（/tts-long 长文本合成）

- 独立页 + 导航项「长文本合成」（ScrollText 图标）；工作台入口 4→5。
- 音色选择逻辑与 TTSPage 重复度高 → 抽 `components/VoicePicker.tsx`
  （场景/语种筛选 + generation 过滤 prop + 分组渲染 + 自定义音色 ID 输入），
  TTSPage 同步改用（行为等价重构）。
- 表单：文本区（10 万字计数、超限/非法字符行内拦截）、参数面板（核心参数 +
  高级折叠：resource/model/explicit_language/pitch/bit_rate/aigc_watermark）、
  结果区（进度含上游 task_id 展示；成功后 WavePlayer 试听 + 音频/SRT 下载行）。
- 结果区/进度骨架沿用页面级实现（ProgressBody/AudioRow 语义与 TTSPage 对齐）。

### 不做（YAGNI，记录在案）

SSML 开关（与 disable_markdown_filter 联动有坑）、latex_parser、pronunciation_dict
（需结构化编辑 UI）、max_length_to_filter_parenthesis、silence_duration、
markdown/emoji filter 开关。需要时按同模式增量加 ParamSpec 即可。

## 测试

- client：httptest mock —— submit 头/体断言、错误码透传（含 logid）、query 三态、
  audio_url 下载。
- tool：预检各分支、mock 全流程（running→success）落盘 audio+SRT、_out 重定向、
  ogg_opus 采样率强制、无凭证报错。
- routes 不变：工具注册后自动出现在 /api/tools，包络与既有任务端点复用。
