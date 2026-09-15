# toolbox MCP Server

toolbox 以 Model Context Protocol 把全部语音能力直接暴露给 AI Agent——与 CLI、Web 控制台并列的第三个消费方。两种传输，同一工具集、同一 JSON 契约：

- **stdio**：`toolbox mcp`，客户端拉起子进程（本地客户端首选）；
- **HTTP**：`toolbox serve` 内嵌 Streamable HTTP 端点 `/api/mcp`（适合无法拉起子进程的客户端）。工具入参与 CLI 旗标同词表（snake_case），输出与 `--json` 同一契约（见 [json-contract.md](json-contract.md)）。

## 接入配置

凭证复用 `~/.toolbox/config.yaml`，无需为 MCP 单独配置。

**Claude Code**（项目根 `.mcp.json` 或全局配置）：

```json
{
  "mcpServers": {
    "toolbox": {
      "command": "/absolute/path/to/toolbox",
      "args": ["mcp"]
    }
  }
}
```

其他 MCP 客户端：标准 stdio 握手，`command` 指向 toolbox 二进制、参数 `mcp` 即可。

## 工具清单（9 个）

| 工具 | 用途 | 必填参数 | 耗时提示 |
|---|---|---|---|
| `toolbox_tts` | 短文本语音合成（同步秒级） | `text` | 秒级 |
| `toolbox_tts_long` | 长文本合成 ≤10 万字，`timestamps` 出 SRT | `text` | 分钟级 |
| `toolbox_tts_stream` | 流式低延迟，20 语种 8 方言，`context_text` 语音指令、`subtitle` 字级字幕 | `text` | 秒级 |
| `toolbox_asr` | 语音识别：`path`（本地文件）或 `url`（+`version: idle/flash`）二选一 | — | 秒~分钟 |
| `toolbox_podcast` | 双人播客：`text`/`url`/`script` 三选一 | `speakers`（恰好 2 个音色 ID） | 分钟级 |
| `toolbox_separate` | 人声/背景多轨分离 | `url` | 分钟级 |
| `toolbox_translate` | 32 语种翻译，`terms` 固定术语译法 | `text`, `to` | 秒级 |
| `toolbox_minutes` | 音视频 URL → 结构化纪要（总结/待办/章节/翻译） | `url`, `features` | 分钟级 |
| `toolbox_voices` | 音色查询（场景/语种/性别/关键词筛选） | — | 即时 |

## HTTP 端点（serve 内嵌）

`toolbox serve` 启动时同时在 **`http://127.0.0.1:<port>/api/mcp`** 暴露 Streamable HTTP 端点（与 Web 控制台同进程、同端口、共享同一任务引擎与数据目录）。客户端配置示例：

```json
{
  "mcpServers": {
    "toolbox": { "url": "http://127.0.0.1:8080/api/mcp" }
  }
}
```

注意：

- 默认仅监听 `127.0.0.1`，与 `/api` 其余端点同一安全姿态（无独立鉴权）——**不要把该端口暴露到公网**，MCP 工具可触发计费云 API；
- 工具调用与 stdio 模式同样**串行排队**执行；
- `toolbox mcp`（stdio）与 HTTP 端点可同时使用，互不影响。

## 行为约定

- **串行执行**：工具调用在服务端排队串行处理（SQLite 单写者约束）。妙记/播客这类分钟级任务会阻塞后续调用——agent 侧应避免并发发起多个长任务。
- **同步等待**：所有工具同步等到任务终态才返回；没有后台任务句柄，取消 = 中断整个调用。
- **产物路径**：`artifacts[].path` 恒为绝对路径；入参 `out` / `out_dir` 可重定向产物位置（等价 CLI `--out`）。
- **计费**：TTS/长文本/流式按字符、ASR 按时长、妙记按小时、播客按字符、分离按次数计费，工具描述内已标注；`toolbox_voices` 免费无调用次数限制。
- **错误**：参数缺失/互斥在调用前返回错误消息（中文），凭证缺失或上游未开通同样以错误文本返回——agent 可据此向用户索取密钥或建议开通。

## 本地验证

```bash
# 握手 + 工具列表（echo 管道即最小客户端）
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | toolbox mcp
```
