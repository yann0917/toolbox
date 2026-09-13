---
name: toolbox
description: 多媒体 AI 工具箱 CLI（toolbox），通过火山引擎提供语音合成 TTS、流式语音合成（低延迟多语种方言）、长文本语音合成（10 万字异步+字幕）、语音识别 ASR、AI 双人播客生成、人声背景音分离六大能力。Use when the user asks to 文字转语音 / 配音 / TTS / 长文本转语音 / 有声书 / 方言配音 / 多语种配音、音频或视频转文字 / 出字幕 / ASR 转写、生成播客 / 播客音频、分离人声与背景音 / 提取人声，or otherwise needs speech/audio processing that the `toolbox` command provides.
---

# toolbox

通过 `toolbox` CLI 完成语音类媒体处理任务。所有命令同步执行：进程退出即任务完成，产物路径直接可用。

## 前置检查

1. 确认已安装：`command -v toolbox`。未安装时从仓库构建：`go build -o toolbox ./cmd/toolbox`（toolbox 仓库内），或告知用户安装 toolbox。
2. 确认凭证已配置：`toolbox config list`。若 `volc.speech.*` 为空，TTS/ASR/播客不可用；若 `volc.mediakit.api_key` 为空，人声分离不可用。请用户提供对应密钥后执行 `toolbox config set <key> <value>` 配置，不要猜测或编造密钥。

## 调用规则

- **始终加 `--json`**：stdout 只输出一个 JSON 结果对象（含产物路径、时长等），人类可读进度走 stderr。
- **显式指定产物路径**：`--out`（separate 为 `--out-dir`），避免依赖默认数据目录。
- 从 JSON 的 `artifacts[].path` / `artifacts[].kind` 读取产物（`audio` / `transcript` / `subtitle` / `dialog`）。
- 退出码：`0` 成功；`2` 参数错误；`3` 任务失败（火山侧报错，stderr 有中文原因）；`4` 凭证缺失或无效。
- 播客生成耗时数分钟（长文本更久），用后台方式执行并轮询进程退出；TTS 短文本秒级返回。

## 六个能力的最小用法

> 当前构建提供 `tts` / `tts-long` / `tts-stream` / `asr` / `podcast` / `separate` / `config` / `voices` / `serve` 命令；`run` 随后续里程碑交付，调用前先 `toolbox --help` 确认可用。

```bash
# 文字转语音（返回 mp3 路径）
toolbox tts "今天天气不错" --voice zh_male_dayixiansheng_v2_saturn_bigtts --out speech.mp3 --json

# 长文本转语音（≤10 万字异步合成，分钟级；--timestamps 额外产出 SRT 字幕，大段文本用 --file）
toolbox tts-long --file book.txt --timestamps --out audiobook.mp3 --json

# 流式合成（低延迟，20 语种/8 方言/语音指令；--subtitle 出字级 SRT 字幕）
toolbox tts-stream "用粤语说一段开场白" --explicit-dialect yue --subtitle --out intro.mp3 --json

# 音频转文字（本地文件直发；--srt 默认开启，额外产出带时间戳 SRT 字幕）
toolbox asr recording.mp3 --out transcript.txt --json

# 生成双人播客（--speakers 必填：两个音色 ID，用 voices list 查询；也可 --url 传网页链接、--script 传自备对话稿）
toolbox podcast "介绍下大模型在语音方向的应用" --speakers zh_male_dayixiansheng_v2_saturn_bigtts,zh_female_mizaitongxue_v2_saturn_bigtts --out podcast.mp3 --json

# 人声背景音分离（输入需公网可访问的音视频 URL）
toolbox separate "https://example.com/video.mp4" --scene audio --out-dir ./sep --json
```

## 参数选择要点

- 音色 ID 通过 `toolbox voices list --json` 查询（含音色分类与性别），不要凭空编造音色 ID。
- 播客默认自动把文本提炼为双人对话；已写好对话稿时用 `--script dialog.json`（格式见参考文档），生成内容更可控。
- ASR 识别带明显背景音的音频前，可先用人声分离提纯人声再转写，准确率更高（两个命令串联）。
- 短文本配音用 `tts`（秒级）或 `tts-stream`（2.0 模型，20 语种/8 方言/语音指令/字级字幕）；书籍/长文等 ≤10 万字用 `tts-long`（分钟级异步，可出字幕），超 10 万字再分段。
- 播客（建议 ≤12000 字）注意长度限制，超出时先分段。

## 完整参考

所有命令、flag、JSON 输出结构、错误码映射见 [references/cli.md](references/cli.md)。工具 schema 也可运行 `toolbox run <provider>.<tool> --help` 动态查看。
