# toolbox

个人自用的多媒体 AI 工具箱：一套 Go 二进制，既是 CLI 也是 Web 控制台，首批接入火山引擎的四个能力。

| 能力 | 说明 | 接口形态 |
|---|---|---|
| 语音合成 TTS | 文本转语音，多音色、语速音量可调 | HTTP + WebSocket |
| 语音识别 ASR | 本地文件/URL 转文字，分句时间戳、SRT 字幕 | 本地文件直发（官方协议）/ URL 异步 |
| 语音播客 | 主题/长文本/网页/对话稿一键生成双人播客 | WebSocket 事件流，支持断点续传 |
| 人声背景音分离 | 从音视频分离人声与背景音双轨 | AI MediaKit REST |

## 特性

- **双形态**：`toolbox tts/asr/podcast/separate` 命令行直用（脚本/agent 友好，`--json` 机器可读输出）；`toolbox serve` 启动 Web 控制台（React 19 + Vite，亮暗双主题，任务化交互 + WebSocket 进度推送）。
- **单二进制**：前端产物 go:embed 内嵌，goroutine 任务池 + SQLite 状态，零外部依赖部署。
- **可扩展**：Provider 抽象层，新平台/新工具以「实现接口 + 注册」接入，前端表单与 CLI 由参数 schema 驱动。
- **Agent 可调用**：随仓库交付 [skills/toolbox](skills/toolbox/SKILL.md)，其他 agent 可直接通过 CLI 调用全部能力。

## 开发状态

M1-M4 已完成（骨架、语音合成、语音识别、语音播客）；人声分离、产品化打磨待实施。设计文档见 [docs/superpowers/specs/2026-09-12-toolbox-design.md](docs/superpowers/specs/2026-09-12-toolbox-design.md)。

## 快速开始

```bash
# 构建前端 + 编译单二进制（前端产物 go:embed 内嵌）
make all

# 配置火山引擎语音凭证
./bin/toolbox config set volc.speech.app_id <APP ID>
./bin/toolbox config set volc.speech.access_token <Token>

# CLI 合成（机器可读输出，退出码 0 成功）
./bin/toolbox tts "你好，toolbox" --out /tmp/hello.mp3 --json

# 音频转文字（--srt 默认产出 SRT 字幕，--srt=false 关闭；其他格式见 skill 参考）
./bin/toolbox asr /tmp/recording.mp3 --out /tmp/transcript.txt --json

# 主题一键生成双人播客（--speakers 必填：两个音色 ID 逗号分隔，可用 toolbox voices list 查询）
./bin/toolbox podcast "用五分钟聊聊本地大模型" \
  --speakers zh_female_cancan_mars_bigtts,zh_male_dayixiansheng_v2_saturn_bigtts \
  --out /tmp/podcast.mp3 --json
# 播客输入四选一：位置参数文本 / --file 长文本文件 / --url 网页 / --script 对话稿 JSON（互斥）
# 音频格式 --format mp3|ogg_opus|pcm|aac，开头音乐 --head-music

# 启动 Web 控制台（默认端口取配置 server.port，可用 --port 覆盖）
./bin/toolbox serve --port 8080
# 浏览器打开 http://127.0.0.1:8080 → 合成/识别/播客工坊页交互、播放、查看历史
```

未配置凭证时执行 `tts` / `asr` / `podcast` 以退出码 4 结束，stderr 提示 `config set` 命令。

## 技术栈

Go（gin / gorm / cobra / resty / viper / gorilla/websocket）· React 19 + TypeScript + Vite + Tailwind CSS · SQLite

## License

Private.
