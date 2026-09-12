# toolbox CLI 完整参考

目录：

- [通用约定](#通用约定)
- [tts 语音合成](#tts-语音合成)
- [asr 语音识别](#asr-语音识别)
- [podcast 播客生成](#podcast-播客生成)
- [separate 人声背景音分离](#separate-人声背景音分离)
- [run 通用工具入口](#run-通用工具入口)
- [voices 音色查询](#voices-音色查询)
- [config 配置管理](#config-配置管理)
- [serve Web 服务](#serve-web-服务)
- [错误码映射](#错误码映射)

## 通用约定

- `--json`：stdout 输出单个 JSON 对象；进度、警告等人类可读信息一律走 stderr。不加 `--json` 时输出人类可读进度条与摘要。
- JSON 结果统一结构：

```json
{
  "task_id": "uuid",
  "provider": "volcengine",
  "tool": "tts",
  "status": "succeeded",
  "cost_ms": 3210,
  "artifacts": [
    {"kind": "audio", "path": "/abs/path/speech.mp3", "format": "mp3", "size": 245760, "duration_ms": 8400}
  ],
  "summary": {"char_count": 42}
}
```

- `artifacts[].kind` 取值：`audio`（音频产物）、`transcript`（纯文本转写）、`subtitle`（SRT 字幕）、`dialog`（播客对话稿 JSON）。
- 退出码：`0` 成功；`2` 用法/参数错误（不会消耗配额）；`3` 任务失败（上游报错，stderr 给中文原因）；`4` 凭证缺失或无效。
- `artifacts[].path` 一律为绝对路径（含 `--out` 重定向与默认数据目录两种来源），可直接交给下游工具使用。
- 未显式传 `--out` 时产物写入默认数据目录（`~/.toolbox/data`，可用 `config set data_dir` 修改）。
- `--out` 传相对路径时按数据目录（默认 `~/.toolbox/data`）解析；产物 JSON 中仍返回绝对路径。

## tts 语音合成

```bash
toolbox tts <text | --file path> [flags]
```

| flag | 默认 | 说明 |
|---|---|---|
| `--voice` | `zh_female_cancan_mars_bigtts` | 音色 ID，用 `toolbox voices list` 查询 |
| `--format` | `mp3` | `mp3` / `wav` / `pcm` / `ogg_opus` |
| `--speed-ratio` | `1.0` | 语速倍率，0.2 ~ 3.0 |
| `--volume-ratio` | `1.0` | 音量倍率，0.2 ~ 3.0 |
| `--file` | — | 从文件读文本（与位置参数二选一） |
| `--out` | 数据目录自动命名 | 产物路径 |
| `--json` | 关 | 机器可读输出 |

超过 1000 字的长文本自动改走分段合成后拼接（仅支持 mp3，其他格式长文本会报参数错误），无需手动处理。

## asr 语音识别

```bash
toolbox asr <file> [--url audio_url] [flags]
```

位置参数 file 与 `--url` 二选一；同传报参数错误，都不传也报参数错误。

| flag | 默认 | 说明 |
|---|---|---|
| `--out` | 数据目录自动命名 | 转写文本输出路径 |
| `--srt` | 开 | 额外产出 `.srt` 字幕（artifacts 中 kind=`subtitle`）；关闭传 `--srt=false` |
| `--hotwords` | 空 | 逗号分隔热词，提升专有名词准确率 |
| `--language` | `zh-CN` | 识别语言，可传其他语言代码 |
| `--url` | — | 公网音频 URL，走异步批量通道 |
| `--json` | 关 | 机器可读输出 |

- 本地文件：官方协议直发识别端点（全速分片），无需公网可达。
- `artifacts` 中 `transcript` 为全文文本；分句与时间戳在 JSON `summary.segments` 中（每句 `{text, start_ms, end_ms}`）。
- 支持 mp3/wav/ogg/pcm（wav/pcm 内部需 pcm_s16le），其他格式报参数错误。

## podcast 播客生成

```bash
toolbox podcast <text | --file path | --url page_url | --script dialog.json> [flags]
```

| flag | 默认 | 说明 |
|---|---|---|
| `--speakers` | settings 中默认搭配 | 两个音色 ID，逗号分隔，顺序为说话人 A、B |
| `--format` | `mp3` | `mp3` / `ogg_opus` / `pcm` / `aac` |
| `--head-music` | 关 | 是否加开头音乐 |
| `--out` | 数据目录自动命名 | 播客音频输出路径 |
| `--json` | 关 | 机器可读输出 |

四种输入模式：

1. 位置参数 / `--file`：主题或长文本，模型自动提炼为双人对话（建议 ≤12000 字）。
2. `--url`：网页链接，服务端联网总结后生成。
3. `--script`：自备对话稿，内容完全可控。格式：

```json
{
  "rounds": [
    {"speaker": "zh_male_dayixiansheng_v2_saturn_bigtts", "text": "开场白……"},
    {"speaker": "zh_female_mizaitongxue_v2_saturn_bigtts", "text": "回应……"}
  ]
}
```

- 产物：`audio`（播客成品）+ `dialog`（实际使用的对话稿 JSON，含每轮起止时间）。
- 注意：生成耗时数分钟，长文本更久；上游断线会自动断点续传。

## separate 人声背景音分离

```bash
toolbox separate <url> [flags]
```

| flag | 默认 | 说明 |
|---|---|---|
| `--scene` | `audio` | `audio` 通用音视频 / `drama` 影视剧 |
| `--out-dir` | 数据目录 | 双轨输出目录（人声、背景音各一个文件） |
| `--json` | 关 | 机器可读输出 |

- 输入必须为公网可访问的音视频 URL（本地文件请先自行上传到可访问的存储）。
- 产物：两个 `audio` artifact，`summary` 中以 `voice` / `background` 标注对应 artifact id。

## run 通用工具入口

```bash
toolbox run <provider>.<tool> [--param key=value ...] [--json]
toolbox run <provider>.<tool> --help   # 动态查看该工具的参数 schema
```

按注册表调用任意已注册工具（含后续新接入的平台），`--param` 按工具 schema 传参。上面四个快捷命令等价于 `toolbox run volcengine.tts` 等。

## voices 音色查询

```bash
toolbox voices list [--json]           # 列出可用音色（分类、性别）
toolbox voices preview <voice_id>      # 播放音色试听样本（规划中，当前未实现）
```

音色表内置于程序，离线可用。

## config 配置管理

```bash
toolbox config set <key> <value>       # 写入 ~/.toolbox/config.yaml（0600）
toolbox config list                    # 查看配置（密钥打码显示）
```

| key | 说明 |
|---|---|
| `volc.speech.app_id` | 火山引擎语音 APP ID（TTS/ASR/播客共用） |
| `volc.speech.access_token` | 语音 Access Token |
| `volc.speech.api_key` | 新版控制台 API Key（与上面二选一） |
| `volc.mediakit.api_key` | AI MediaKit API Key（人声分离） |
| `server.port` | Web 端口，默认 8080 |
| `data_dir` | 产物数据目录，默认 `~/.toolbox/data` |

## serve Web 服务

```bash
toolbox serve [--port 8080]
```

启动 Web 控制台（含 REST API 与 WebSocket）。CLI 与 Web 共用同一数据目录与任务历史。

## 错误码映射

| 上游情形 | 退出码 | stderr 提示方向 |
|---|---|---|
| 鉴权失败（App ID / Token 错误） | 4 | 「凭证无效，请检查 config」 |
| 未配置凭证 | 4 | 「先执行 toolbox config set …」 |
| 参数错误 / 长度超限 | 2 | 具体参数与限制 |
| 内容审核拦截 | 3 | 「内容触发安全审核」 |
| 额度/余额不足 | 3 | 「资源包额度不足」 |
| 限流 | 3 | 「触发限流，请稍后重试」 |
| 网络中断 | 3 | 「连接中断（已尝试断点续传）」 |
