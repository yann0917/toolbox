import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiBase, fetchJSON } from "../lib/api";
import TaskProgress from "../components/TaskProgress";
import MiniPlayer from "../components/MiniPlayer";
import { useTaskEvents } from "../lib/ws";

interface Task { id: string; status: string; progress: number; progress_note: string; error?: string }

export default function TTSPage() {
  const [text, setText] = useState("");
  const [voice, setVoice] = useState("zh_female_cancan_mars_bigtts");
  const [format, setFormat] = useState("mp3");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<Task | null>(null);
  const [audioUrl, setAudioUrl] = useState<string | null>(null);
  const qc = useQueryClient();
  const ev = useTaskEvents();

  // WS 事件驱动当前任务进度与完成
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") setTask((t) => ({ ...(t ?? { id: taskId, status: "running" } as Task), progress: ev.progress ?? 0, progress_note: ev.note ?? "" }));
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<{ task: Task; artifacts: { id: string; kind: string; filename: string }[] }>(`/api/tasks/${taskId}`)
        .then((d) => {
          setTask(d.task);
          const audio = d.artifacts.find((a) => a.kind === "audio");
          if (audio) setAudioUrl(`${apiBase}/api/artifacts/${audio.id}/stream`);
        });
    }
  }, [ev, taskId]);

  const submit = useMutation({
    mutationFn: () =>
      fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({ provider: "volcengine", tool: "tts", params: { text, voice, format, speed_ratio: 1, volume_ratio: 1 } }),
      }),
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setTask({ id: d.task_id, status: "pending", progress: 0, progress_note: "已提交" });
      setAudioUrl(null);
      qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => alert(e.message),
  });

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">语音合成</h1>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="输入要合成的文本…"
          rows={8}
          className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-sm focus:outline-none focus:border-[var(--accent)]"
        />
        <div className="flex items-center gap-3 text-sm">
          <label className="text-[var(--muted)]">音色</label>
          <input value={voice} onChange={(e) => setVoice(e.target.value)}
            className="flex-1 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2" />
          <label className="text-[var(--muted)]">格式</label>
          <select value={format} onChange={(e) => setFormat(e.target.value)}
            className="rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {["mp3", "wav", "pcm", "ogg_opus"].map((f) => <option key={f}>{f}</option>)}
          </select>
        </div>
        <button
          disabled={!text.trim() || submit.isPending}
          onClick={() => submit.mutate()}
          className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium disabled:opacity-40"
        >
          {submit.isPending ? "提交中…" : "开始合成"}
        </button>
      </div>
      {task && <TaskProgress task={task} />}
      {audioUrl && <MiniPlayer src={audioUrl} title="合成结果" />}
    </div>
  );
}
