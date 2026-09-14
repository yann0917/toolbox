# toolbox 产品化规划（M1 任务体验 + M2 能力开放）

> 状态：**已全部实施并推送**。M1+M2 对应 `71436ff`（溯源+重跑）/ `b860ec4`（Web M1）/ `55cb1a8`（MCP+契约）/ `86a8ded`（词典）/ `f7aeb85`（批量）；后续 P2 见 `5a558ad`（录音）/ `2f0c207`（字幕工坊）。
> 实施时的两处修正：①「不动 schema」约束被打破——Task 增加可空 `input` 列（输入溯源不落库则无法重跑/回放，属规划时评估不足）；② 2C 词典存储经讨论定为 config.yaml 而非 DB（配置型资产人可读、随 HOME 走），Web 端管理入口后置。
> 来源：ZCode plan mode 会话规划，按「值得留的计划归档 docs/plans/」约定入库。

依据：EdgeSpeak/Meetily 双调研 + 代码探索。两个探索关键结论已校准预期：**ASR 页同步播放已存在**（ASRPage.tsx:260-288），**任务参数已持久化且已下发前端**（Task.Params，dto.go:19）——M1 是"补齐与泛化"，不是从零建。

---

## M1 任务体验三件套（先实施）

### 1A. 同步播放泛化 —— 把 ASR 页的样板抽成公共能力（借 Meetily 音频同步 + EdgeSpeak 跟读高亮）

1. 新增 `web/src/lib/useTranscriptSync.ts`：从 ASRPage.tsx:260-288 提取 hook，输入 `segments[{start_ms,end_ms}] + playSrc`，返回 `{activeIdx, seekTo(ms)}`。封装两个既有坑：**先 play 再 seek**（player.ts:98 audio 未初始化时 seek 是 no-op）、**仅当前轨高亮**（subscribeTime + idx 变化才 setState）。
2. 同步新增展示组件 `web/src/components/TranscriptList.tsx`（时间码 mono + 说话人前缀 + 当前句 `border-accent bg-raise-2`，样式对齐 ASRPage.tsx:626-653），ASRPage 改用 hook+组件，行为不变。
3. MinutesPage 接入：转写预览（现 MinutesPage.tsx:450-464 纯 `<p>`）换 TranscriptList。音频源解析 `resolvePlaySrc(task, artifacts)`：上传文件→`/api/uploads/:id/stream`；外部 URL→直接用原 URL 播放。**降级路径**：波形峰值靠 WebAudio 解码需 CORS，跨域 URL 大概率失败——`player.ts` loadPeaks 失败时返回 null，WavePlayer 增加无波形降级模式（进度条式，仍可点击 seek），遵守 --wave-idle 令牌。
4. HistoryPage 任务详情整合（1A+1B 汇合点）：展开面板新增「参数」区 + asr/minutes 任务的 TranscriptList 同步回放（playSrc 优先取本任务 audio artifact / 上传源）。

验收：妙记/ASR/历史三处点句播放、跟读高亮一致；无波形降级不报错；`go build`+`tsc` 通过；真实短音频 ASR（flash 版）截图验证。

### 1B. 任务重跑 + 参数中心（借 Meetily Import & Enhance）

1. 后端 `POST /api/tasks/:id/rerun`（routes.go 新 handler）：读原任务 → 校验 tool 仍注册 → 预检可重跑性（file_ids 对应 upload 文件 stat 存在、artifact_input 产物未删，缺失返回 code=2 带明确 message）→ 原样克隆 params/file_ids/artifact_input 走 engine.Submit → 返回新 task_id。复用现有 validate() 与 apierr 分级，无需新 WS 事件（终态 toast 已有）。
2. 前端：`types.ts` Task 补 `params` 字段；HistoryPage 详情渲染参数（micro 标签 + mono 值，file_ids 显示文件数）；「重跑」按钮 → POST rerun → toast + invalidate tasks；`toolNames.ts` 增 tool→路由映射，toast 附「去工具页」跳转（沿用 `/asr?artifact=` 已证的跨页带参模式）。
3. 不做表单预填编辑（?rerun= 预填列为 M2 后增强项）——克隆重跑已覆盖 90% 场景且与页面解耦。

验收：URL 任务一键重跑成功并出现于历史；缺上传文件的重跑返回可读错误；取消新任务正常（engine 取消链路现成）。

### 1C. 妙记模板导出（零 LLM，借 Meetily 模板 JSON + item_format 表格思路）

1. `web/src/lib/minutesExport.ts`：模板定义 `{id, name, sections:[{title, include, format}]}`，三种模板——**通用会议纪要**（元信息/全文总结/待办表格（内容×执行人×时间码）/章节表格/翻译附录）、**简洁速记**（总结+待办）、**待办行动清单**（待办为主）。数据全部来自妙记 Summary 现有结构（summary_text/todos/chapters/translation），前端组装 Markdown + blob 下载 + 复制到剪贴板。
2. MinutesPage 结果区头部：自定义 Select（禁原生，MASTER.md）选模板 + 「导出 Markdown」按钮。
3. 明确不做：LLM 重总结（已拍板零 LLM）、富文本编辑器（后续里程碑再议）。

验收：三模板导出内容正确、待办为 Markdown 表格；导出文件可读性人工检查。

**提交粒度**：1A hook/组件重构 → 1B rerun → 1C 导出 → Minutes/History 整合，各 1 commit。

---

## M2 能力开放（M1 后排期，本次一并规划）

### 2A. MCP server（借 EdgeSpeak 12 工具出口 + Meetily llama-helper stdio 模式）
- 新增 `toolbox mcp` 子命令：stdio JSON-RPC MCP server，优先官方 Go SDK（github.com/modelcontextprotocol/go-sdk，实施时确认成熟度，不行则按 spec 手写 initialize/tools.list/tools.call 三件套）。
- 复用 service 层（CLI/Web 之外的第三个消费方，service.go 本就是唯一共享入口）+ SubmitSync，暴露 8 个工具：tts / tts_long / tts_stream / asr（文件路径或 URL 输入）/ translate / minutes（同步等待轮询，注明长耗时）/ podcast / separate + voices 查询；输出复用 `--json` 形状。
- 交付 `docs/mcp.md`（Claude Code 等接入配置片段）+ 更新 skills/toolbox/SKILL.md。

### 2B. JSON 输出契约文档（借 EdgeSpeak 独立契约章节）
- `docs/json-contract.md`：逐工具 `--json` 输出 shape（任务包络/artifacts/summary 键位），标注「契约 v1」，README 与 skills 链接；MCP 工具输出直接引用它。

### 2C. 词典资产（借 EdgeSpeak 词典/固定读法）
- CLI `toolbox dict list/add/rm`：命名热词表 + 术语对，落 `~/.toolbox` 配置（viper）。
- 服务端 `GET/PUT /api/dicts`；ASR/妙记 hotwords、翻译 terms 输入处加「从词典填入」（Select 选择后回填）。实施时核对 asr_tool 是否已有 hotwords 参数（minutes 已有），没有则词典首期只覆盖妙记+翻译。

### 2D. 批量任务队列（借 EdgeSpeak 批量导入）
- WorkbenchPage 增「批量识别」卡：多行 URL 逐行 POST 提交（复用引擎并发=2 天然排队），页面本地批次分组 + WS 进度实时刷新 + 行内取消/重跑（重用 1B）。不改 DB schema，不加新端点。

---

## 横切规范（来自工程调研的直接应用）

- 错误分级复用 apierr code（2=参数/6=不存在），不新造体系；context 取消沿用 engine 既有链路；不新增 WS 事件类型。
- 所有新 UI 走 ui/ 组件库 + 令牌，禁原生 select/alert/emoji 图标；数字读数 mono。
- 不动 SQLite schema（M1/M2 均无新表新列）；不留 `*_old` 文件；ASRPage 重构是迁移不是复制。
- 验证卫生（既有约定）：临时 HOME + 独立端口起服、临时 vite 配置；UI 用真实短任务 + 截图 + 程序化断言；收尾 unrouteAll/停进程/清临时目录。
- 每步 `make build`（go vet + 前端 tsc）再提交，conventional commit 中文风格。

## 实施顺序

1A → 1B → 1C → M1 整合验证（提交）→ 2A → 2B → 2C → 2D（M2 各自独立提交）。M1 完成即交付一次，M2 逐项交付。
