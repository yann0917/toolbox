import type { Task } from "./types";

/**
 * 妙记导出模板：零 LLM，模板只定义章节取舍与排列，
 * 内容全部来自妙记 API 的既有结构化结果（summary_text/todos/chapters/translation/segments）。
 */

export type MinutesSectionKey = "meta" | "summary" | "todos" | "chapters" | "translation" | "transcript";

export interface MinutesTemplate {
  id: string;
  name: string;
  /** 章节按此顺序输出，仅渲染有内容的部分 */
  sections: MinutesSectionKey[];
}

export const MINUTES_TEMPLATES: MinutesTemplate[] = [
  {
    id: "standard",
    name: "通用会议纪要",
    sections: ["meta", "summary", "todos", "chapters", "translation", "transcript"],
  },
  {
    id: "brief",
    name: "简洁速记",
    sections: ["meta", "summary", "todos"],
  },
  {
    id: "actions",
    name: "待办行动清单",
    sections: ["meta", "todos"],
  },
];

export const defaultMinutesTemplate = MINUTES_TEMPLATES[0];

function fmtClock(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = String(Math.floor((total % 3600) / 60)).padStart(2, "0");
  const s = String(total % 60).padStart(2, "0");
  return h > 0 ? `${h}:${m}:${s}` : `${m}:${s}`;
}

interface Summary {
  minutes_title?: string;
  summary_text?: string;
  translation_text?: string;
  features?: string[];
  sentences?: number;
  speakers_count?: number;
  duration_ms?: number;
  segments?: { text: string; start_ms: number; end_ms: number }[];
  todos?: { content: string; executor: string[]; start_time: number }[];
  chapters?: { title: string; summary: string; start_time: number; end_time: number }[];
}

/** 组装 Markdown 文本；章节缺失时跳过，模板声明但无数据的节不输出空标题。 */
export function buildMinutesMarkdown(task: Task, tpl: MinutesTemplate): string {
  const s = (task.summary ?? {}) as Summary;
  const title = s.minutes_title || "会议纪要";
  const out: string[] = [`# ${title}`, ""];

  const meta: string[] = [];
  if (task.created_at) meta.push(`- 时间：${task.created_at}`);
  if (s.duration_ms) meta.push(`- 时长：${fmtClock(s.duration_ms)}`);
  if (s.speakers_count) meta.push(`- 说话人：${s.speakers_count} 人`);
  if (s.sentences) meta.push(`- 句数：${s.sentences}`);
  if (s.features?.length) meta.push(`- 功能：${s.features.join("、")}`);
  if (meta.length) out.push("## 会议信息", ...meta, "");

  if (s.summary_text && tpl.sections.includes("summary")) {
    out.push("## 全文总结", "", s.summary_text.trim(), "");
  }

  if (tpl.sections.includes("todos") && s.todos?.length) {
    out.push("## 待办事项", "", "| # | 待办 | 执行人 | 时间 |", "| --- | --- | --- | --- |");
    s.todos.forEach((td, i) => {
      const exec = td.executor.filter((e) => e && e !== "无").join("、") || "—";
      const at = td.start_time > 0 ? fmtClock(td.start_time) : "—";
      out.push(`| ${i + 1} | ${td.content.replace(/\|/g, "\\|")} | ${exec} | ${at} |`);
    });
    out.push("");
  }

  if (tpl.sections.includes("chapters") && s.chapters?.length) {
    out.push("## 章节总结", "", "| 时间 | 章节 | 摘要 |", "| --- | --- | --- |");
    s.chapters.forEach((ch) => {
      out.push(`| ${fmtClock(ch.start_time)} – ${fmtClock(ch.end_time)} | ${ch.title.replace(/\|/g, "\\|")} | ${(ch.summary || "").replace(/\|\n/g, " ")} |`);
    });
    out.push("");
  }

  if (tpl.sections.includes("translation") && s.translation_text) {
    out.push("## 翻译文本", "", s.translation_text.trim(), "");
  }

  if (tpl.sections.includes("transcript") && s.segments?.length) {
    out.push("## 转写全文", "");
    s.segments.forEach((seg) => {
      out.push(`- \`${fmtClock(seg.start_ms)}\` ${seg.text}`);
    });
    out.push("");
  }

  out.push("---", "", `由 toolbox 语音妙记生成 · 任务 ${task.id}`);
  return out.join("\n");
}

/** 导出文件名：标题 + 任务短 ID，去掉文件系统不安全字符。 */
export function minutesFilename(task: Task): string {
  const s = (task.summary ?? {}) as Summary;
  const title = (s.minutes_title || "妙记纪要").replace(/[\\/:*?"<>|\s]+/g, "_").slice(0, 40);
  return `${title}_${task.id.slice(0, 8)}.md`;
}
