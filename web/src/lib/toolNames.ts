/** 工具名 → 中文名（HistoryPage 筛选器与全局任务通知共用）。 */
export const toolName: Record<string, string> = {
  tts: "语音合成",
  tts_long: "长文本合成",
  tts_stream: "流式合成",
  asr: "语音识别",
  podcast: "播客工坊",
  separate: "人声分离",
  translate: "机器翻译",
  minutes: "语音妙记",
};

export const toolLabel = (t: string): string => toolName[t] ?? t;

/** 工具名 → 控制台路由（历史页重跑后跳转等跨页导航用）。 */
export const toolRoute: Record<string, string> = {
  tts: "/tts",
  tts_long: "/tts?tab=long",
  tts_stream: "/tts?tab=stream",
  asr: "/asr",
  podcast: "/podcast",
  separate: "/separate",
  translate: "/translate",
  minutes: "/minutes",
};
