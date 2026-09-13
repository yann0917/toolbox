# toolbox

个人自用的多媒体 AI 工具箱：一套 Go 二进制，既是 CLI 也是 Web 控制台，首批接入火山引擎的八个能力。

| 能力 | 说明 | 接口形态 |
|---|---|---|
| 语音合成 TTS | 文本转语音：同步秒级 / 流式低延迟（20 语种、8 方言、字级字幕、语音指令）/ 长文本（≤10 万字）异步合成，均可出 SRT 字幕 | HTTP + Chunked 流式 + 异步任务 + WebSocket |
| 语音识别 ASR | 本地文件/URL 转文字，分句时间戳、SRT 字幕 | 三版本：本地直发（官方协议）/ URL 异步（标准） / 闲时 / 极速同步 |
| 语音播客 | 主题/长文本/网页/对话稿一键生成双人播客 | WebSocket 事件流，支持断点续传 |
| 人声背景音分离 | 公网音视频 URL 多轨分离：Audio/Music 双轨（人声+背景/伴奏），Drama/Narrate 三轨（人声+音乐+音效） | AI MediaKit REST（产物 24h 临时链接，立即转存本地） |
| 机器翻译 | 32 语种互译、自动检测源语言、术语定制（直传术语 / 术语表） | HTTP 同步（matx_translate，需开通 volc.speech.mt） |
| 语音妙记 | 音视频 URL 转结构化纪要：转写+说话人、全文总结、待办/问答提取、章节总结、中英翻译（≤2h、<1G） | HTTP 异步（lark submit/query，结果链接 24h 有效立即转存） |

## 特性

- **双形态**：`toolbox tts/asr/podcast/separate/translate/minutes` 命令行直用（脚本/agent 友好，`--json` 机器可读输出）；`toolbox serve` 启动 Web 控制台。
- **单二进制**：前端产物 go:embed 内嵌，goroutine 任务池 + SQLite 状态，零外部依赖部署；纯 Go sqlite 驱动，可交叉编译（`make dist`）。
- **可扩展**：Provider 抽象层，新平台/新工具以「实现接口 + 注册」接入，前端表单与 CLI 由参数 schema 驱动。
- **Agent 可调用**：随仓库交付 [skills/toolbox](skills/toolbox/SKILL.md)，其他 agent 可直接通过 CLI 调用全部能力。

## Web 控制台

界面按「专业音频设备」的调性设计：近黑面板 + 琥珀信号色、刻印微标签、等宽数字读数，暗色默认且亮色完整适配（可跟随系统）。

- **语音合成三通道**：同步 / 流式 / 长文本以 Tab 切换，每通道附计费说明（按场景与费用选择）
- **计费测算**：火山语音刊例价快照（2026-09-13），三通道同量对比 + ASR/播客/机器翻译/语音妙记估算器，以账单为准
- **工作台**：真实运行统计（任务数/成功率/累计耗时）+ 最近任务
- **试听**：自研波形播放器 + 全局播放条，历史页与各工具页的产物可直接播放、下载
- **工具联动**：人声分离页的人声轨可「送 ASR 识别」——产物直接作为识别输入，无需公网 URL

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

# 机器翻译（32 语种互译，--from 缺省自动检测；--terms 直传术语「原词=译词」，也可 --table-id/--table-name 用术语表）
./bin/toolbox translate "火山引擎是字节跳动旗下的企业级智能技术服务平台" --to en \
  --terms "火山引擎=Volcengine" --out /tmp/translated.txt --json

# 语音妙记（音视频 URL 转结构化纪要，--features 附加功能至少一项，分钟级异步）
./bin/toolbox minutes "https://example.com/meeting.mp4" --features summary,todo,chapter \
  --speakers 0 --out-dir /tmp/minutes --json

# 启动 Web 控制台（默认端口取配置 server.port，可用 --port 覆盖）
./bin/toolbox serve --port 8080
# 浏览器打开 http://127.0.0.1:8080 → 各工具页交互、播放、查看历史
```

未配置凭证时执行 `tts` / `asr` / `podcast` / `translate` / `minutes`（语音凭证；仅播客必须 APP ID + Access Token）或 `separate`（MediaKit API Key，两套凭证独立）以退出码 4 结束，stderr 提示 `config set` 命令。

> 各火山能力需在控制台开通对应服务：语音三件套开「豆包语音」、机器翻译开 `volc.speech.mt`、语音妙记开 `volc.lark.minutes`、人声分离开 AI MediaKit；开通后数分钟内生效。

## 构建与发布

```bash
make all     # 构建前端 + 单二进制到 bin/toolbox（版本号取自 git describe）
make test    # Go 测试全量
make web     # 仅构建前端并同步到 embed 目录
make dist    # 交叉编译五个平台（darwin/linux × amd64/arm64 + windows/amd64）打包到 dist/
```

`make dist` 依赖无 CGO 的纯 Go sqlite 驱动，因此无需交叉编译工具链即可产出各平台可执行文件。darwin 产物已含 ad-hoc 签名（Go 交叉编译 darwin/arm64 时自动签，release workflow 另在 macOS runner 上显式 `codesign` 加固），但未做 Apple 公证。

**macOS 安装说明**（从 Release 下载 zip 解压后）：

```bash
# 浏览器下载的文件带 quarantine 隔离属性，未公证的二进制会被 Gatekeeper 拦
# （提示「Apple could not verify …」）。移除隔离属性即可运行：
xattr -d com.apple.quarantine ./toolbox

# 也可在「系统设置 → 隐私与安全性」中对该文件点「仍要打开」。
# 用 curl/wget 直接下载的文件不带隔离属性，解压即可运行：
curl -LO <release 里的 zip 地址> && unzip toolbox-*-darwin-arm64.zip && ./toolbox-*/toolbox --version
```

## 技术栈

Go（gin / gorm / cobra / resty / viper / gorilla/websocket）· React 19 + TypeScript + Vite + Tailwind CSS v4 · SQLite · Lucide 图标 · Fira Sans / Fira Code（自托管，离线可用）

## 文档

- [设计文档](docs/superpowers/specs/2026-09-12-toolbox-design.md)：架构、接口要点、契约
- [UI 设计规范](design-system/toolbox/MASTER.md)：色彩/字体/间距/组件规格（Web 界面实现依据）
- [agent skill](skills/toolbox/SKILL.md)：CLI 完整参考（[references/cli.md](skills/toolbox/references/cli.md)），供 agent 与脚本调用

## License

Private.
