import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { apiBase, fetchJSON } from "../lib/api";
import TaskProgress from "../components/TaskProgress";
import MiniPlayer from "../components/MiniPlayer";
import { useTaskEvents } from "../lib/ws";

interface Task {
  id: string;
  status: string;
  progress: number;
  progress_note: string;
  error?: string;
  summary?: { scene?: string; duration_s?: number; tracks?: string[] };
}
interface Artifact {
  id: string;
  kind: string;
  filename: string;
  format?: string;
  meta?: { track?: string };
}

// 分离场景与输出格式白名单和后端 SeparateTool ParamSpecs 一致。
const SCENES = [
  { value: "Audio", label: "通用场景 · 2 轨" },
  { value: "Music", label: "音乐场景 · 2 轨" },
  { value: "Drama", label: "短剧场景 · 3 轨" },
  { value: "Narrate", label: "口播场景 · 3 轨" },
];
const FORMATS = ["aac", "mp3", "wav", "m4a", "flac"];
const TRACK_LABELS: Record<string, string> = {
  voice: "人声",
  background: "背景音",
  music: "音乐",
  sfx: "音效",
};
const SCENE_LABELS: Record<string, string> = {
  Audio: "通用",
  Music: "音乐",
  Drama: "短剧",
  Narrate: "口播",
};
const trackLabel = (t?: string) => (t ? TRACK_LABELS[t] ?? t : "音轨");

export default function SeparatePage() {
  const [url, setUrl] = useState("");
  const [scene, setScene] = useState("Audio");
  const [format, setFormat] = useState("mp3");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<Task | null>(null);
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const ev = useTaskEvents();

  // WS 事件驱动当前任务进度；终态拉详情拿产物列表（每轨一个 audio artifact）与 summary
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") setTask((t) => ({ ...(t ?? { id: taskId, status: "running" } as Task), progress: ev.progress ?? 0, progress_note: ev.note ?? "" }));
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<{ task: Task; artifacts: Artifact[] }>(`/api/tasks/${taskId}`)
        .then((d) => {
          setTask(d.task);
          setArtifacts(d.artifacts);
        })
        .catch(() => {});
    }
  }, [ev, taskId]);

  const submit = useMutation({
    mutationFn: () =>
      fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({
          provider: "volcengine",
          tool: "separate",
          params: { url: url.trim(), scene, output_format: format },
        }),
      }),
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setTask({ id: d.task_id, status: "pending", progress: 0, progress_note: "已提交" });
      setArtifacts([]);
      qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => alert(e.message),
  });

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">人声分离</h1>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <div className="space-y-1">
          <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="公网可访问的音视频/音频 URL，本地文件请先上传至对象存储"
            className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-sm focus:outline-none focus:border-[var(--accent)]" />
          <p className="text-xs text-[var(--muted)]">MediaKit 需公网可访问地址，本地文件请先上传至对象存储</p>
        </div>
        <div className="flex items-center gap-3 text-sm">
          <label className="text-[var(--muted)]">场景</label>
          <select value={scene} onChange={(e) => setScene(e.target.value)}
            className="flex-1 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {SCENES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
          </select>
          <label className="text-[var(--muted)]">格式</label>
          <select value={format} onChange={(e) => setFormat(e.target.value)}
            className="rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2">
            {FORMATS.map((f) => <option key={f}>{f}</option>)}
          </select>
        </div>
        <button
          disabled={!url.trim() || submit.isPending}
          onClick={() => submit.mutate()}
          className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium disabled:opacity-40"
        >
          {submit.isPending ? "提交中…" : "开始分离"}
        </button>
      </div>
      {task && <TaskProgress task={task} />}
      {task?.summary && (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 text-sm space-y-1">
          <p>场景：<span className="text-[var(--muted)]">{SCENE_LABELS[task.summary.scene ?? ""] ?? task.summary.scene ?? "-"}</span></p>
          <p>时长：<span className="text-[var(--muted)]">{task.summary.duration_s != null ? `${task.summary.duration_s}s` : "-"}</span></p>
          <p>轨道：<span className="text-[var(--muted)]">{task.summary.tracks?.map(trackLabel).join("、") || "-"}</span></p>
        </div>
      )}
      {artifacts.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-[var(--muted)]">分离结果（每轨一行）</h2>
          {artifacts.map((a) => {
            const track = a.meta?.track;
            return (
              <div key={a.id} className="flex items-center gap-3">
                <span className="shrink-0 w-16 text-sm text-[var(--fg)]">{trackLabel(track)}</span>
                <div className="flex-1 min-w-0">
                  <MiniPlayer src={`${apiBase}/api/artifacts/${a.id}/stream`} title={a.filename} />
                </div>
                <a href={`${apiBase}/api/artifacts/${a.id}/download`} className="shrink-0 text-sm text-[var(--accent)]">下载</a>
                {track === "voice" && (
                  <button onClick={() => navigate(`/asr?artifact=${a.id}`)}
                    className="shrink-0 px-3 py-1.5 rounded-lg border border-[var(--border)] text-sm hover:border-[var(--accent)]">
                    送 ASR识别
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
