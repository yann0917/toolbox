# toolbox

个人自用的多媒体 AI 工具箱：一套 Go 二进制，既是 CLI 也是 Web 控制台，首批接入火山引擎的六个能力。

| 能力 | 说明 | 接口形态 |
|---|---|---|
| 语音合成 TTS | 文本转语音：同步秒级 / 流式低延迟（20 语种、8 方言、字级字幕、语音指令）/ 长文本（≤10 万字）异步合成，均可出 SRT 字幕 | HTTP + Chunked 流式 + 异步任务 + WebSocket |
| 语音识别 ASR | 本地文件/URL 转文字，分句时间戳、SRT 字幕 | 三版本：本地直发（官方协议）/ URL 异步（标准） / 闲时 / 极速同步 |
| 语音播客 | 主题/长文本/网页/对话稿一键生成双人播客 | WebSocket 事件流，支持断点续传 |
| 人声背景音分离 | 公网音视频 URL 多轨分离：Audio/Music 双轨（人声+背景/伴奏），Drama/Narrate 三轨（人声+音乐+音效） | AI MediaKit REST（产物 24h 临时链接，立即转存本地） |

## 特性

- **双形态**：`toolbox tts/asr/podcast/separate` 命令行直用（脚本/agent 友好，`--json` 机器可读输出）；`toolbox serve` 启动 Web 控制台。
- **单二进制**：前端产物 go:embed 内嵌，goroutine 任务池 + SQLite 状态，零外部依赖部署；纯 Go sqlite 驱动，可交叉编译（`make dist`）。
- **可扩展**：Provider 抽象层，新平台/新工具以「实现接口 + 注册」接入，前端表单与 CLI 由参数 schema 驱动。
- **Agent 可调用**：随仓库交付 [skills/toolbox](skills/toolbox/SKILL.md)，其他 agent 可直接通过 CLI 调用全部能力。

## Web 控制台

界面按「专业音频设备」的调性设计：近黑面板 + 琥珀信号色、刻印微标签、等宽数字读数，暗色默认且亮色完整适配（可跟随系统）。

- **工作台**：六个工具入口 + 真实运行统计（任务数/成功率/累计耗时）+ 最近任务
- **波形播放器**：自研 `WavePlayer` —— WebAudio 解码真实峰值、canvas 波形、已播段着色、点击/拖拽定位、mono 时间码
- **全局播放条**：跨页面常驻，同一时刻只播一路音频；历史页与各工具页的结果都可直接试听
- **工具联动**：人声分离页的人声轨可「送 ASR 识别」——产物直接作为识别输入，无需公网 URL
- **组件库**：`web/src/ui/` 统一组件（按钮/字段/卡片/徽标/进度/骨架/空态/Toast/对话框/分段控件/波形播放器），页面不再手写样式
- **设计规范**：[design-system/toolbox/MASTER.md](design-system/toolbox/MASTER.md)（色彩/字体/间距/组件/反模式/交付检查表），基于 ui-ux-pro-max 校正定稿

## 开发状态

M1-M6 已完成：骨架、语音合成、语音识别、语音播客、人声分离，以及 Web 产品化重做（设计系统 + 组件库 + 全站页面 + 音频播放体验）。设计文档见 [docs/superpowers/specs/2026-09-12-toolbox-design.md](docs/superpowers/specs/2026-09-12-toolbox-design.md)。

## 快速开始

```bash
# 构建前端 + 编译单二进制（前端产物 go:embed 内嵌）
make all

# 配置火山引擎语音凭证
./bin/toolbox config set volc.speech.app_id <APP ID>
./bin/toolbox config set volc.speech.access_token <Token>

# CLI 合成（机器可读输出，退出码 0 成功）
./bin/toolbox tts "你好，toolbox" --out /tmp/hello.mp3 --json

# 长文本异步合成（≤10 万字，seed-tts-2.0；--timestamps 额外产出 SRT 字幕）
./bin/toolbox tts-long --file book.txt --timestamps --out /tmp/audiobook.mp3 --json

# 流式合成（低延迟；20 语种、8 方言、语音指令 --context-text、字级字幕 --subtitle）
./bin/toolbox tts-stream "用粤语说一段开场白" --explicit-dialect yue --subtitle --out /tmp/intro.mp3 --json

# 音频转文字（--srt 默认产出 SRT 字幕，--srt=false 关闭；其他格式见 skill 参考）
./bin/toolbox asr /tmp/recording.mp3 --out /tmp/transcript.txt --json

# 录音文件识别版本 --version standard|idle|flash（闲时/极速仅收公网 URL）
./bin/toolbox asr --url "https://example.com/talk.mp3" --version flash --json   # 极速版，同步秒级返回
./bin/toolbox asr --url "https://example.com/talk.mp3" --version idle  --json   # 闲时版，低价、24h 内完成

# 主题一键生成双人播客（--speakers 必填：两个音色 ID 逗号分隔，用 toolbox voices list 查询，支持 --scene/--lang 筛选）
./bin/toolbox podcast "用五分钟聊聊本地大模型" \
  --speakers zh_female_cancan_mars_bigtts,zh_male_dayixiansheng_v2_saturn_bigtts \
  --out /tmp/podcast.mp3 --json
# 播客输入四选一：位置参数文本 / --file 长文本文件 / --url 网页 / --script 对话稿 JSON（互斥）
# 音频格式 --format mp3|ogg_opus|pcm|aac，开头音乐 --head-music

# 人声背景音分离（四场景 audio/music 双轨、drama/narrate 三轨；需独立的 MediaKit API Key）
./bin/toolbox config set volc.mediakit.api_key <MediaKit API Key>
./bin/toolbox separate "https://example.com/media.mp4" --scene audio --format mp3 --json
# 音视频公网 URL 必填（MediaKit 不支持本地文件，本地文件请先上传至对象存储）；
# 输出格式 --format aac|mp3|wav|m4a|flac，产物音轨落盘 --out-dir 指定目录

# 启动 Web 控制台（默认端口取配置 server.port，可用 --port 覆盖）
./bin/toolbox serve --port 8080
# 浏览器打开 http://127.0.0.1:8080 → 合成/识别/播客/分离页交互、播放、查看历史
```

未配置凭证时执行 `tts` / `asr` / `podcast`（语音凭证）或 `separate`（MediaKit API Key，两套凭证独立）以退出码 4 结束，stderr 提示 `config set` 命令。

## 构建与发布

```bash
make all     # 构建前端 + 单二进制到 bin/toolbox（版本号取自 git describe）
make test    # Go 测试全量
make web     # 仅构建前端并同步到 embed 目录
make dist    # 交叉编译五个平台（darwin/linux × amd64/arm64 + windows/amd64）打包到 dist/
```

`make dist` 依赖无 CGO 的纯 Go sqlite 驱动，因此无需交叉编译工具链即可产出各平台可执行文件。

## 技术栈

Go（gin / gorm / cobra / resty / viper / gorilla/websocket）· React 19 + TypeScript + Vite + Tailwind CSS v4 · SQLite · Lucide 图标 · Fira Sans / Fira Code（自托管，离线可用）

## License

Private.
