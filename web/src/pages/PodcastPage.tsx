import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiBase, fetchJSON } from "../lib/api";
import TaskProgress from "../components/TaskProgress";
import MiniPlayer from "../components/MiniPlayer";
import DialogFlow, { type DialogFlowItem } from "../components/DialogFlow";
import { useTaskEvents } from "../lib/ws";

interface Task {
  id: string;
  status: string;
  progress: number;
  progress_note: string;
  error?: string;
  summary?: { rounds?: number; duration_s?: number };
}
interface Artifact { id: string; kind: string; filename: string }
interface Voice { id: string; gender: string; category: string }

type Mode = "text" | "url" | "script";
const MODES: { key: Mode; label: string }[] = [
  { key: "text", label: "主题/长文本" },
  { key: "url", label: "网页 URL" },
  { key: "script", label: "对话稿" },
];
const FORMATS = ["mp3", "ogg_opus", "pcm", "aac"];
const SCRIPT_PLACEHOLDER = '{"rounds":[{"speaker":"音色ID","text":"大家好，欢迎收听本期播客…"},{"speaker":"音色ID","text":"主持人好，今天我们聊聊…"}]}';

// 时长摘要展示：秒取整，超过一分钟显示「X 分 Y 秒」。
function fmtDuration(s?: number): string {
  if (s == null) return "—";
  const total = Math.round(s);
  return total >= 60 ? `${Math.floor(total / 60)} 分 ${total % 60} 秒` : `${total} 秒`;
}

export default function PodcastPage() {
  const [mode, setMode] = useState<Mode>("text");
  const [text, setText] = useState("");
  const [url, setUrl] = useState("");
  const [script, setScript] = useState("");
  const [voiceA, setVoiceA] = useState("");
  const [voiceB, setVoiceB] = useState("");
  const [format, setFormat] = useState("mp3");
  const [headMusic, setHeadMusic] = useState(false);
  const [formError, setFormError] = useState("");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<Task | null>(null);
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const [dialogArtifact, setDialogArtifact] = useState<Artifact | null>(null);
  const [flow, setFlow] = useState<DialogFlowItem[]>([]);
  const qc = useQueryClient();
  const ev = useTaskEvents();

  // 音色列表：A/B 各默认取列表前两个音色（顺序即说话顺序）
  const { data: voicesData } = useQuery({
    queryKey: ["voices"],
    queryFn: () => fetchJSON<{ voices: Voice[] }>("/api/voices"),
  });
  const voices = voicesData?.voices ?? [];
  const firstVoice = voices[0]?.id ?? "";
  const voiceAVal = voiceA || firstVoice;
  const voiceBVal = voiceB || voices[1]?.id || firstVoice;

  // WS 事件驱动当前任务进度与对话流（detail.text 非空即一轮对话）；终态拉详情拿产物与 summary
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") {
      setTask((t) => ({ ...(t ?? { id: taskId, status: "running" } as Task), progress: ev.progress ?? 0, progress_note: ev.note ?? "" }));
      const d = ev.detail;
      if (d?.text) {
        const item: DialogFlowItem = { round: d.rounds_done ?? 0, speaker: d.speaker ?? "", text: d.text };
        setFlow((items) => [...items, item]);
      }
    }
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<{ task: Task; artifacts: Artifact[] }>(`/api/tasks/${taskId}`)
        .then((d) => {
          setTask(d.task);
          const audio = d.artifacts.find((a) => a.kind === "audio");
          if (audio) setAudioUrl(`${apiBase}/api/artifacts/${audio.id}/stream`);
          setDialogArtifact(d.artifacts.find((a) => a.kind === "dialog") ?? null);
        })
        .catch(() => {});
    }
  }, [ev, taskId]);

  const submit = useMutation({
    mutationFn: () => {
      const params: Record<string, unknown> = {
        speakers: `${voiceAVal},${voiceBVal}`,
        format,
        head_music: headMusic,
      };
      if (mode === "text") params.input_text = text.trim();
      else if (mode === "url") params.url = url.trim();
      else params.script = script; // 对话稿 JSON 原文直传，校验交给后端
      return fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({ provider: "volcengine", tool: "podcast", params }),
      });
    },
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setTask({ id: d.task_id, status: "pending", progress: 0, progress_note: "已提交" });
      setAudioUrl(null);
      setDialogArtifact(null);
      setFlow([]);
      qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => alert(e.message),
  });

  // 提交前本地校验：当前模式内容非空 + 双音色已选（行内提示，不走 alert）
  const start = () => {
    const content = mode === "text" ? text.trim() : mode === "url" ? url.trim() : script.trim();
    if (!content) {
      setFormError(mode === "text" ? "请输入播客主题或长文本" : mode === "url" ? "请输入网页链接" : "请输入对话稿 JSON");
      return;
    }
    if (!voiceAVal || !voiceBVal) {
      setFormError("请选择说话人 A/B 的音色");
      return;
    }
    setFormError("");
    submit.mutate();
  };

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">播客工坊</h1>

      {/* 1. 内容输入 */}
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <h2 className="text-sm font-medium text-[var(--muted)]">① 内容输入</h2>
        <div className="flex flex-wrap gap-2 text-sm">
          {MODES.map((m) => (
            <button key={m.key}
              onClick={() => { setMode(m.key); setFormError(""); }}
              className={`px-4 py-1.5 rounded-lg border transition-colors ${
                mode === m.key
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] border-[var(--accent)]"
                  : "border-[var(--border)] text-[var(--muted)] hover:text-[var(--fg)]"
              }`}
            >
              {m.label}
            </button>
          ))}
        </div>
        {mode === "text" && (
          <textarea value={text} onChange={(e) => setText(e.target.value)}
            placeholder="播客主题或长文本（≤12000 字），如「聊聊身边的 AI 工具」…"
            rows={6}
            className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-sm focus:outline-none focus:border-[var(--accent)]" />
        )}
        {mode === "url" && (
          <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="网页链接，抓取正文后生成播客…"
            className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-sm focus:outline-none focus:border-[var(--accent)]" />
        )}
        {mode === "script" && (
          <textarea value={script} onChange={(e) => setScript(e.target.value)}
            placeholder={SCRIPT_PLACEHOLDER}
            rows={8}
            className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-xs font-mono focus:outline-none focus:border-[var(--accent)]" />
        )}
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <label className="text-[var(--muted)] shrink-0">说话人 A</label>
          <select value={voiceAVal} onChange={(e) => setVoiceA(e.target.value)}
            className="flex-1 min-w-64 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {voices.map((v) => <option key={v.id} value={v.id}>{v.id}（{v.gender}·{v.category}）</option>)}
          </select>
          <label className="text-[var(--muted)] shrink-0">说话人 B</label>
          <select value={voiceBVal} onChange={(e) => setVoiceB(e.target.value)}
            className="flex-1 min-w-64 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {voices.map((v) => <option key={v.id} value={v.id}>{v.id}（{v.gender}·{v.category}）</option>)}
          </select>
        </div>
        <div className="flex flex-wrap items-center gap-4 text-sm">
          <label className="text-[var(--muted)]">格式</label>
          <select value={format} onChange={(e) => setFormat(e.target.value)}
            className="rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {FORMATS.map((f) => <option key={f}>{f}</option>)}
          </select>
          <label className="flex items-center gap-1.5 text-[var(--muted)]">
            <input type="checkbox" checked={headMusic} onChange={(e) => setHeadMusic(e.target.checked)} />
            开头音乐
          </label>
        </div>
        {formError && <p className="text-xs" style={{ color: "var(--danger)" }}>{formError}</p>}
        <button disabled={submit.isPending} onClick={start}
          className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium disabled:opacity-40">
          {submit.isPending ? "提交中…" : "生成播客"}
        </button>
      </div>

      {/* 2. 生成与进度 */}
      {(task || flow.length > 0) && (
        <div className="space-y-4">
          <h2 className="text-sm font-medium text-[var(--muted)]">② 生成进度</h2>
          {task && <TaskProgress task={task} />}
          {flow.length > 0 && <DialogFlow items={flow} />}
        </div>
      )}

      {/* 3. 成品：summary + 音频播放 + 对话稿下载 */}
      {(audioUrl || dialogArtifact || task?.summary != null) && (
        <div className="space-y-4">
          <h2 className="text-sm font-medium text-[var(--muted)]">③ 成品</h2>
          {task?.summary != null && (
            <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 text-sm">
              <p>
                共 <span className="text-[var(--accent)]">{task.summary.rounds ?? 0}</span> 轮对话
                · 总时长 <span className="text-[var(--accent)]">{fmtDuration(task.summary.duration_s)}</span>
              </p>
            </div>
          )}
          {audioUrl && <MiniPlayer src={audioUrl} title="播客音频" />}
          {dialogArtifact && (
            <a href={`${apiBase}/api/artifacts/${dialogArtifact.id}/download`} className="text-sm text-[var(--accent)]">
              下载对话稿 {dialogArtifact.filename}
            </a>
          )}
        </div>
      )}
    </div>
  );
}
