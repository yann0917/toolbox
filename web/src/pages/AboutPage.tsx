import { Link } from "react-router-dom";
import {
  ArrowUpRight,
  AudioLines,
  BookOpenText,
  ClipboardList,
  Coins,
  Cpu,
  ExternalLink,
  Info,
  KeyRound,
  Languages,
  Mic,
  Monitor,
  NotebookPen,
  Podcast,
  Terminal,
  Waves,
} from "lucide-react";
import { PRICE_SNAPSHOT_DATE } from "../lib/pricing";
import { Card, CardBody, CardHeader, MicroLabel, PageHeader } from "../ui";

/** 能力一览：与导航/工作台入口对应，name 为跳转路径 */
const CAPABILITIES = [
  {
    to: "/tts", icon: AudioLines, name: "语音合成", cmd: "toolbox tts / tts-stream / tts-long",
    desc: "文本转语音：同步秒级、流式低延迟（20 语种、8 方言、语音指令）、长文本 10 万字异步；均可出 SRT 字幕",
  },
  {
    to: "/asr", icon: Mic, name: "语音识别", cmd: "toolbox asr",
    desc: "本地文件直发或 URL 转文字，分句时间戳；标准 / 闲时 / 极速三版本，导出 TXT 与 SRT",
  },
  {
    to: "/podcast", icon: Podcast, name: "语音播客", cmd: "toolbox podcast",
    desc: "主题、长文本、网页或对话稿一键生成双人播客，支持断点续传",
  },
  {
    to: "/separate", icon: Waves, name: "人声分离", cmd: "toolbox separate",
    desc: "公网音视频 URL 多轨分离：通用/音乐双轨，短剧/口播三轨，产物立即转存本地",
  },
  {
    to: "/translate", icon: Languages, name: "机器翻译", cmd: "toolbox translate",
    desc: "32 语种互译、自动检测源语言、术语定制，秒级同步返回",
  },
  {
    to: "/minutes", icon: NotebookPen, name: "语音妙记", cmd: "toolbox minutes",
    desc: "音视频 URL 转结构化纪要：转写+说话人、全文总结、待办/问答提取、章节总结、中英翻译",
  },
];

/** 官方文档外链（产品介绍与计费口径来源） */
const OFFICIAL_DOCS = [
  { label: "豆包语音产品简介", url: "https://docs.volcengine.com/docs/6561/163032" },
  { label: "计费概述", url: "https://www.volcengine.com/docs/6561/1359369" },
  { label: "计费说明", url: "https://www.volcengine.com/docs/6561/1359370" },
];

/** 快速上手三步 */
function Step({ n, title, children }: { n: string; title: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-3">
      <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full border border-line bg-raise-2 font-mono text-[11px] tabular-nums text-accent">
        {n}
      </span>
      <div className="min-w-0 space-y-1.5">
        <p className="text-sm font-medium text-fg">{title}</p>
        <div className="space-y-1.5 text-[13px] leading-relaxed text-fg-2">{children}</div>
      </div>
    </div>
  );
}

function Cmd({ children }: { children: string }) {
  return (
    <code className="block overflow-x-auto whitespace-pre rounded-[var(--radius-sm)] border border-line bg-inset px-3 py-2 font-mono text-xs leading-relaxed text-fg-2">
      {children}
    </code>
  );
}

/** 关于页：产品介绍、能力一览、使用方法与底层模型说明（纯内容页）。 */
export default function AboutPage() {
  return (
    <>
      <PageHeader
        title="关于"
        description="toolbox · 个人自用的多媒体 AI 工作台"
        icon={<Info size={16} strokeWidth={1.75} />}
        actions={
          <Link
            to="/history"
            className="inline-flex items-center gap-1 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
          >
            历史产物
            <ArrowUpRight size={13} strokeWidth={1.75} />
          </Link>
        }
      />

      {/* 产品介绍 */}
      <Card className="mb-4">
        <CardBody className="space-y-3">
          <p className="text-sm leading-relaxed text-fg">
            toolbox 是一套个人自用的多媒体 AI 工具箱：单个 Go 二进制，既是命令行工具也是 Web 控制台。
            它把火山引擎豆包语音的八项 AI 能力装进同一个任务引擎——提交任务、实时进度、产物落盘、历史可溯，
            面向配音、转写、播客、会议纪要、翻译等日常内容生产场景。
          </p>
          <p className="text-sm leading-relaxed text-fg-2">
            命令行面向脚本与 agent（<code className="rounded bg-inset px-1.5 py-0.5 font-mono text-xs">--json</code> 输出机器可读结果，
            退出码区分成功 / 参数 / 失败 / 凭证）；Web 控制台按「专业音频设备」的调性设计，
            提供波形试听、计费测算与任务历史。两者共享同一份数据目录与任务记录。
          </p>
        </CardBody>
      </Card>

      {/* 能力一览 */}
      <Card className="mb-4">
        <CardHeader
          title="能力一览"
          icon={<ClipboardList size={15} strokeWidth={1.75} />}
          aside={<span className="micro">六个入口 · 八项能力</span>}
        />
        <CardBody className="space-y-2">
          {CAPABILITIES.map(({ to, icon: Icon, name, cmd, desc }) => (
            <Link
              key={to}
              to={to}
              className="group flex items-start gap-3 rounded-[var(--radius-sm)] border border-line bg-raise-2 px-3 py-2.5 transition-colors duration-150 hover:border-line-strong"
            >
              <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-[var(--radius-sm)] border border-line bg-raise text-accent">
                <Icon size={16} strokeWidth={1.75} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                  <span className="text-sm font-medium text-fg">{name}</span>
                  <span className="font-mono text-[11px] text-muted">{cmd}</span>
                </span>
                <span className="mt-0.5 block text-xs leading-relaxed text-muted">{desc}</span>
              </span>
              <ArrowUpRight
                size={13}
                strokeWidth={1.75}
                className="mt-1 shrink-0 text-muted transition-colors duration-150 group-hover:text-accent"
              />
            </Link>
          ))}
        </CardBody>
      </Card>

      {/* 快速上手 */}
      <Card className="mb-4">
        <CardHeader title="快速上手" icon={<Terminal size={15} strokeWidth={1.75} />} aside={<span className="micro">CLI 与 Web 同源</span>} />
        <CardBody className="space-y-5">
          <Step n="1" title="配置凭证">
            <p>
              在 <Link to="/settings" className="text-accent transition-colors duration-150 hover:opacity-80">设置页</Link> 填写凭证，
              或执行（配置文件位于 <code className="rounded bg-inset px-1.5 py-0.5 font-mono text-xs">~/.toolbox/config.yaml</code>）：
            </p>
            <Cmd>toolbox config set volc.speech.app_id &lt;APP ID&gt;&#10;toolbox config set volc.speech.access_token &lt;Token&gt;</Cmd>
            <p className="text-xs text-muted">
              <KeyRound size={12} strokeWidth={1.75} className="mr-1 inline" />
              仅播客必须 APP ID + Access Token，其余能力支持新版 API Key 单键；人声分离使用独立的 MediaKit API Key。
            </p>
          </Step>
          <Step n="2" title="命令行调用">
            <p>
              所有命令同步执行、进程退出即完成；加 <code className="rounded bg-inset px-1.5 py-0.5 font-mono text-xs">--json</code> 获得机器可读产物路径。
            </p>
            <Cmd>toolbox tts "你好，toolbox" --out hello.mp3 --json&#10;toolbox minutes "https://example.com/meeting.mp4" --features summary,todo --json</Cmd>
            <p className="text-xs text-muted">
              完整命令与参数见
              <a
                href="https://github.com/yann0917/toolbox/blob/main/skills/toolbox/references/cli.md"
                target="_blank"
                rel="noreferrer"
                className="mx-1 text-accent transition-colors duration-150 hover:opacity-80"
              >
                CLI 完整参考
              </a>
              ，或 <code className="rounded bg-inset px-1.5 py-0.5 font-mono text-xs">toolbox &lt;命令&gt; --help</code>。
            </p>
          </Step>
          <Step n="3" title="Web 控制台">
            <Cmd>toolbox serve --port 8080</Cmd>
            <p className="flex flex-wrap items-center gap-1.5 text-xs text-muted">
              <Monitor size={12} strokeWidth={1.75} className="mr-1 inline" />
              浏览器打开 http://127.0.0.1:8080 —— 各工具页交互、试听与历史；同量费用对比见
              <Link to="/pricing" className="text-accent transition-colors duration-150 hover:opacity-80">
                计费测算
              </Link>
              （刊例快照 {PRICE_SNAPSHOT_DATE}，以账单为准）。
            </p>
          </Step>
        </CardBody>
      </Card>

      {/* 底层模型（产品介绍） */}
      <Card>
        <CardHeader
          title="底层能力：火山引擎豆包语音"
          icon={<Cpu size={15} strokeWidth={1.75} />}
          aside={<span className="micro">大模型体系</span>}
        />
        <CardBody className="space-y-3">
          <p className="text-sm leading-relaxed text-fg-2">
            全部能力由火山引擎豆包语音大模型体系驱动。官方产品线覆盖音频创作（Seed-Audio，单条 Prompt 生成影视级多轨音频）、
            语音合成（多情感高表现力 TTS）、声音复刻（少量样本克隆音色）、语音识别（高准确率转写）、
            语音播客（多角色对谈生成）、语音同传、语音妙记（会议转写与智能纪要）与机器翻译——
            toolbox 按个人工作流挑选并组合了其中八项，统一封装为本地工具；音色列表、能力边界与计费口径以官方文档为准。
          </p>
          <div className="space-y-1.5">
            <MicroLabel className="inline-flex items-center gap-1">
              <BookOpenText size={12} strokeWidth={1.75} />
              官方文档
            </MicroLabel>
            <div className="flex flex-wrap gap-x-4 gap-y-1.5">
              {OFFICIAL_DOCS.map((d) => (
                <a
                  key={d.url}
                  href={d.url}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
                >
                  {d.label}
                  <ExternalLink size={12} strokeWidth={1.75} />
                </a>
              ))}
              <Link
                to="/pricing"
                className="inline-flex items-center gap-1 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
              >
                <Coins size={12} strokeWidth={1.75} />
                本地计费测算
              </Link>
            </div>
          </div>
        </CardBody>
      </Card>
    </>
  );
}
