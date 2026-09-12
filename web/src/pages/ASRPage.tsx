import { useEffect, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { apiBase, fetchJSON } from "../lib/api";
import TaskProgress from "../components/TaskProgress";
import SegmentList, { type Segment } from "../components/SegmentList";
import { useTaskEvents } from "../lib/ws";

interface Task {
  id: string;
  status: string;
  progress: number;
  progress_note: string;
  error?: string;
  summary?: { segments?: Segment[] };
}
interface Artifact { id: string; kind: string; filename: string }

// 音频格式白名单与后端 ASR Tool 一致（mp3/wav/ogg/pcm）。
const ACCEPT = ".mp3,.wav,.ogg,.pcm";

export default function ASRPage() {
  // 跨工具联动：/asr?artifact=<id>（来自分离页「送 ASR识别」）→ 跳过输入区，
  // 以 artifact_input 直接提交识别任务。
  const [searchParams] = useSearchParams();
  const artifactId = searchParams.get("artifact")?.trim() ?? "";
  const artifactMode = artifactId !== "";
  const [mode, setMode] = useState<"upload" | "url">("upload");
  const [file, setFile] = useState<File | null>(null);
  const [url, setUrl] = useState("");
  const [language, setLanguage] = useState("zh-CN");
  const [hotwords, setHotwords] = useState("");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<Task | null>(null);
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [segments, setSegments] = useState<Segment[]>([]);
  const [playSrc, setPlaySrc] = useState<string | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const blobRef = useRef<string | null>(null); // 上传回放的对象 URL（换任务时释放）
  const qc = useQueryClient();
  const ev = useTaskEvents();

  // WS 事件驱动当前任务进度；终态拉详情拿产物链接与 summary.segments
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") setTask((t) => ({ ...(t ?? { id: taskId, status: "running" } as Task), progress: ev.progress ?? 0, progress_note: ev.note ?? "" }));
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<{ task: Task; artifacts: Artifact[] }>(`/api/tasks/${taskId}`)
        .then((d) => {
          setTask(d.task);
          setArtifacts(d.artifacts);
          setSegments(d.task.summary?.segments ?? []);
        })
        .catch(() => {});
    }
  }, [ev, taskId]);

  const canSubmit = artifactMode || (mode === "upload" ? file != null : url.trim() !== "");

  const submit = useMutation({
    mutationFn: async () => {
      const params: Record<string, unknown> = { srt: true, language: language.trim() || "zh-CN" };
      if (hotwords.trim()) params.hotwords = hotwords.trim();
      if (artifactMode) {
        // artifact_input 模式：已有人声轨产物直接作为输入（params 仅 srt/language，无 url/file_ids）。
        return fetchJSON<{ task_id: string }>("/api/tasks", {
          method: "POST",
          body: JSON.stringify({ provider: "volcengine", tool: "asr", params, artifact_input: artifactId }),
        });
      }
      if (mode === "url") {
        return fetchJSON<{ task_id: string }>("/api/tasks", {
          method: "POST",
          body: JSON.stringify({ provider: "volcengine", tool: "asr", params }),
        });
      }
      // 本地上传：先 POST /api/uploads 拿 file_id（不设 Content-Type，让浏览器带 multipart boundary），
      // 再建任务走 file_ids 本地文件通道。
      const fd = new FormData();
      fd.append("file", file!);
      const up = await fetchJSON<{ file_id: string }>("/api/uploads", { method: "POST", body: fd, headers: {} });
      return fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({ provider: "volcengine", tool: "asr", params, file_ids: [up.file_id] }),
      });
    },
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setTask({ id: d.task_id, status: "pending", progress: 0, progress_note: "已提交" });
      setSegments([]);
      setArtifacts([]);
      if (artifactMode) {
        setPlaySrc(`${apiBase}/api/artifacts/${artifactId}/stream`);
      } else if (mode === "upload" && file) {
        if (blobRef.current) URL.revokeObjectURL(blobRef.current);
        blobRef.current = URL.createObjectURL(file);
        setPlaySrc(blobRef.current);
      } else {
        setPlaySrc(url.trim());
      }
      qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => alert(e.message),
  });

  // 点击句子 → 回放跳转到该句起始时间并继续播放
  const seek = (ms: number) => {
    const a = audioRef.current;
    if (!a) return;
    a.currentTime = ms / 1000;
    a.play().catch(() => {});
  };

  const downloads = artifacts.filter((a) => a.kind === "transcript" || a.kind === "subtitle");

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">语音识别</h1>
      {artifactMode ? (
        <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
          <p className="text-sm">
            将使用人声分离任务的音轨直接识别（产物 <span className="text-[var(--accent)]">{artifactId.slice(0, 8)}</span>）
          </p>
          <button
            disabled={submit.isPending}
            onClick={() => submit.mutate()}
            className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium disabled:opacity-40"
          >
            {submit.isPending ? "提交中…" : "开始识别"}
          </button>
        </div>
      ) : (
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <div className="flex gap-2 text-sm">
          {(["upload", "url"] as const).map((m) => (
            <button key={m}
              onClick={() => setMode(m)}
              className={`px-4 py-1.5 rounded-lg border transition-colors ${
                mode === m
                  ? "bg-[var(--accent)] text-[var(--accent-fg)] border-[var(--accent)]"
                  : "border-[var(--border)] text-[var(--muted)] hover:text-[var(--fg)]"
              }`}
            >
              {m === "upload" ? "本地上传" : "音频 URL"}
            </button>
          ))}
        </div>
        {mode === "upload" ? (
          <input type="file" accept={ACCEPT}
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            className="w-full text-sm file:mr-3 file:px-3 file:py-1.5 file:rounded-lg file:border-0 file:bg-[var(--surface-hover)] file:text-[var(--fg)]" />
        ) : (
          <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="公网音频 URL…"
            className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] p-3 text-sm focus:outline-none focus:border-[var(--accent)]" />
        )}
        <div className="flex items-center gap-3 text-sm">
          <label className="text-[var(--muted)]">语言</label>
          <input value={language} onChange={(e) => setLanguage(e.target.value)}
            className="flex-1 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2" />
          <label className="text-[var(--muted)]">热词</label>
          <input value={hotwords} onChange={(e) => setHotwords(e.target.value)} placeholder="可选，逗号分隔"
            className="flex-1 rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2" />
        </div>
        <button
          disabled={!canSubmit || submit.isPending}
          onClick={() => submit.mutate()}
          className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium disabled:opacity-40"
        >
          {submit.isPending ? "提交中…" : "开始识别"}
        </button>
      </div>
      )}
      {task && <TaskProgress task={task} />}
      {playSrc && <audio ref={audioRef} controls src={playSrc} className="w-full" />}
      {segments.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-[var(--muted)]">识别结果（点击句子跳播）</h2>
          <SegmentList segments={segments} onSeek={seek} />
        </div>
      )}
      {downloads.length > 0 && (
        <div className="flex flex-wrap gap-4">
          {downloads.map((a) => (
            <a key={a.id} href={`${apiBase}/api/artifacts/${a.id}/download`} className="text-sm text-[var(--accent)]">
              下载 {a.filename}
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
