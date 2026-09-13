# 单向流式语音合成（HTTP Chunked）设计

日期：2026-09-13
状态：已实现（本 spec 为随附设计存档）
官方文档：6561/2528925（`POST /api/v3/tts/unidirectional`）

## 背景与定位

TTS 家族第三条通道。与前两条并存、不合并：

| 工具 | 接口 | 定位 |
|---|---|---|
| `tts` | `/api/v1/tts`（旧版 V1 同步） | 短文本秒级出音频（1.0 音色、wav） |
| `tts_stream`（本次） | `/api/v3/tts/unidirectional`（HTTP Chunked 单向流式） | 2.0 模型低延迟流式；**20 语种 + 8 方言 + 字级字幕 + 语音指令** |
| `tts_long` | `/api/v3/tts/submit` + `query`（异步任务） | 10 万字长文本、分句时间戳 SRT |

## 协议要点（官方 Go demo 权威确认）

- 请求头：`X-Api-Key`（或旧版双头，复用 `sauc.NewAuthHeaderFrom`）+ `X-Api-Resource-Id`
  （seed-tts-2.0 / seed-icl-2.0）+ `X-Api-Request-Id`（必选 uuid）；
  另带 `X-Control-Require-Usage-Tokens-Return: *` 换取计费字数（usage.text_words）。
- 请求体：`{req_params:{text, model?, speaker, audio_params{...}, ...}}`（与 tts_long 同构）。
- **响应为按行分隔的 JSON 流**（bufio.Scanner 逐行，buffer 上限 10MB 对齐官方 demo）：
  - 中间行 `code == 0`：`data` 为 base64 音频分片，顺序拼接；
  - 终止行 `code == 20000000`：成功结束（可能携带完整 sentence/usage）；
  - 其他 `code > 0`：错误行（message + X-Tt-Logid 透传）。
  注意与 tts_long 的差异：这里成功码以**终止行 20000000** 判定，中间分片是 0。
- `sentence.words`：字级时间戳（word/startTime/endTime 秒 float + confidence），仅
  `enable_subtitle=true` 且中英语种返回。
- 格式 mp3/pcm/ogg_opus/**wav**（ogg 仅 48000；wav/pcm 不支持 bit_rate；文档流式推荐 pcm）。
- 文档未给文本长度上限，不做长度预检；控制字符规则仅长文本接口明确，这里不做。

## 工具设计（tts_stream_tool.go）

- 鉴权 `SpeechCred.Validate()`；参数校验先行（退出码 2）：
  format 枚举、ogg→48000、wav/pcm+bit_rate 冲突、speech/loudness 范围交服务端。
- ParamSpecs：text / voice（默认 vv_uranus 2.0）/ format / sample_rate / speech_rate /
  loudness_rate / subtitle（enable_subtitle）/ resource / model；
  高级：explicit_language（20 语种）/ explicit_dialect（8 方言）/ pitch / bit_rate /
  silence_duration / context_texts（单条语音指令，非空包数组）/ tone_fidelity（仅复刻）/ aigc_watermark。
- 进度：连接 10 → 每收到一分片缓升（封顶 90，detail 带 chunks）→ 保存 95。
- 产物：audio `tts_stream/<uuid>.<ext>`；subtitle 开启且 words 非空 → **字级聚合出 SRT**
  （按句末标点 `。！？；…!?;` 切句，句起止取首末字时间戳，尾巴残余成句）。
- Summary：char_count / billed_chars（usage.text_words，成本敏感）/ chunks /
  duration_ms（末字 endTime）。`_out` 契约与既有工具一致。

## CLI（tts-stream）

`toolbox tts-stream <text|--file>`：--voice --format --sample-rate --speech-rate
--loudness-rate --subtitle --resource --model --explicit-language --explicit-dialect
--pitch --bit-rate --silence-duration --context-text --tone-fidelity --aigc-watermark
--out --json。

## Web（/tts-stream 流式合成）

独立页 + 导航「流式合成」（Radio 图标），工作台 5→6；VoicePicker（generation=2.0）与
ArtifactRow 复用；高级折叠含语种/方言/语音指令/还原模式等。历史页工具名与筛选同步。

## 不做（YAGNI）

SSML、latex_parser、pronunciation_dict、aigc_metadata（meta 水印四字段）、
section_id（跨包语义）、markdown/emoji filter 开关、disable_default_bit_rate。
边收边播的流式试听不做：任务引擎与产物体系围绕落盘设计，Web 端按分片数推进真实进度即可。

## 测试

- client：mock 按行返回 3×code0 分片 + 20000000 终态（含 usage/words）→ 拼接/累积断言；
  错误行（code 45000201）→ 报错含 code 与 logid；HTTP 500 → 报错；逐片回调计数。
- tool：format/bit_rate/ogg 采样率校验、mock 全流程（subtitle 聚合 SRT 内容断言）、
  _out 重定向、无凭证报错、SRT 聚合函数单测（中英混排、无标点尾巴）。
- service/routes 工具数断言 5→6。
