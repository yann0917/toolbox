import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { Download, SplitSquareHorizontal, Waves } from "lucide-react";
import { apiBase, fetchJSON } from "../lib/api";
import type { TaskStatus } from "../lib/types";
import { useTaskEvents } from "../lib/ws";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  EmptyState,
  Field,
  Input,
  PageHeader,
  ProgressBar,
  Select,
  StatusBadge,
  WavePlayer,
  useToast,
} from "../ui";

interface SepTask {
  id: string;
  status: TaskStatus;
  progress: number;
  progress_note: string;
  error?: string;
  cost_ms?: number;
  summary?: { scene?: string; duration_s?: number; tracks?: string[] };
}
interface SepArtifact {
  id: string;
  kind: string;
  filename: string;
  format?: string;
  size?: number;
  duration_ms?: number;
  meta?: { track?: string };
}

/** 场景与格式白名单与后端 SeparateTool 的 ParamSpecs 保持一致 */
const SCENES = [
  { value: "Audio", name: "通用", tracks: 2, desc: "通用音视频，输出人声与背景音" },
  { value: "Music", name: "音乐", tracks: 2, desc: "含背景音乐的素材，输出人声与背景音" },
  { value: "Drama", name: "短剧", tracks: 3, desc: "影视剧对白，输出人声 / 音乐 / 音效" },
  { value: "Narrate", name: "口播", tracks: 3, desc: "口播讲解，输出人声 / 音乐 / 音效" },
];
const FORMATS = ["mp3", "aac", "wav", "m4a", "flac"];
const TRACK_LABELS: Record<string, string> = {
  voice: "人声",
  background: "背景音",
  music: "音乐",
  sfx: "音效",
};
const trackLabel = (t?: string) => (t ? TRACK_LABELS[t] ?? t : "音轨");
/** 语音识别仅接受 mp3/wav，其他格式不提供「送 ASR」入口 */
const asrCompatible = (fmt?: string) => fmt === "mp3" || fmt === "wav";

function formatSize(bytes?: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export default function SeparatePage() {
  const [url, setUrl] = useState("");
  const [scene, setScene] = useState("Audio");
  const [format, setFormat] = useState("mp3");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [task, setTask] = useState<SepTask | null>(null);
  const [artifacts, setArtifacts] = useState<SepArtifact[]>([]);

  const qc = useQueryClient();
  const navigate = useNavigate();
  const { toast } = useToast();
  const ev = useTaskEvents();

  // WS 事件驱动进度；终态拉详情取每轨产物与 summary
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") {
      setTask((t) => ({
        ...(t ?? ({ id: taskId, status: "running" } as SepTask)),
        progress: ev.progress ?? 0,
        progress_note: ev.note ?? "",
      }));
    }
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<{ task: SepTask; artifacts: SepArtifact[] }>(`/api/tasks/${taskId}`)
        .then((d) => {
          setTask(d.task);
          setArtifacts(d.artifacts);
        })
        .catch((e: Error) => toast({ tone: "error", title: "获取任务详情失败", description: e.message }));
    }
  }, [ev, taskId, toast]);

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
      void qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => toast({ tone: "error", title: "提交失败", description: e.message }),
  });

  const running = task?.status === "running" || task?.status === "pending";

  return (
    <>
      <PageHeader
        title="人声分离"
        description="从公网音视频中分离人声与背景音（AI MediaKit，独立凭证）"
      />

      <Card>
        <CardHeader title="输入与参数" icon={<Waves size={15} strokeWidth={1.75} />} />
        <CardBody className="space-y-5 p-4">
          <Field
            label="音视频 URL"
            required
            hint="MediaKit 仅接受公网可访问地址；本地文件请先上传至对象存储（或火山 TOS）。"
          >
            {({ id, ...rest }) => (
              <Input
                id={id}
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://example.com/media.mp4"
                {...rest}
              />
            )}
          </Field>

          <div className="space-y-2">
            <span className="micro">分离场景</span>
            <div role="radiogroup" aria-label="分离场景" className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
              {SCENES.map((s) => {
                const active = scene === s.value;
                return (
                  <button
                    key={s.value}
                    role="radio"
                    aria-checked={active}
                    onClick={() => setScene(s.value)}
                    className={`cursor-pointer rounded-[var(--radius-md)] border p-3 text-left transition-colors duration-150 ${
                      active
                        ? "border-accent bg-raise-2"
                        : "border-line bg-raise hover:border-line-strong hover:bg-raise-2"
                    }`}
                  >
                    <div className="flex items-baseline justify-between gap-2">
                      <span className={`text-sm font-medium ${active ? "text-accent" : "text-fg"}`}>{s.name}</span>
                      <span className="font-mono text-[11px] text-muted">{s.tracks} 轨</span>
                    </div>
                    <p className="mt-1 text-[11px] leading-relaxed text-muted">{s.desc}</p>
                  </button>
                );
              })}
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="输出格式" hint="默认 mp3；语音识别仅支持 mp3 / wav">
              {({ id, ...rest }) => (
                <Select id={id} value={format} onChange={(e) => setFormat(e.target.value)} {...rest}>
                  {FORMATS.map((f) => (
                    <option key={f} value={f}>
                      {f.toUpperCase()}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <div className="flex items-end">
              <Button
                variant="primary"
                className="w-full"
                loading={submit.isPending}
                disabled={!url.trim() || running}
                onClick={() => submit.mutate()}
                icon={<SplitSquareHorizontal size={14} strokeWidth={1.75} />}
              >
                {running ? "分离中…" : "开始分离"}
              </Button>
            </div>
          </div>
        </CardBody>
      </Card>

      {task && (
        <Card className="mt-4">
          <CardHeader
            title="处理进度"
            aside={
              <>
                {task.cost_ms ? <span className="micro">{`${(task.cost_ms / 1000).toFixed(1)}s`}</span> : null}
                <StatusBadge status={task.status} />
              </>
            }
          />
          <CardBody className="space-y-2.5 p-4">
            <ProgressBar value={task.progress} active={running} />
            <p className="text-xs text-muted">{task.progress_note || "—"}</p>
            {task.error && <p className="text-xs text-danger break-words">{task.error}</p>}
            {task.summary && (
              <div className="flex flex-wrap gap-x-6 gap-y-1 pt-1 text-xs">
                <span className="text-muted">
                  场景 <span className="font-mono text-fg-2">{SCENES.find((s) => s.value === task.summary?.scene)?.name ?? task.summary.scene ?? "—"}</span>
                </span>
                <span className="text-muted">
                  时长{" "}
                  <span className="font-mono text-fg-2">
                    {task.summary.duration_s != null ? `${task.summary.duration_s.toFixed(1)}s` : "—"}
                  </span>
                </span>
                <span className="text-muted">
                  轨道{" "}
                  <span className="font-mono text-fg-2">
                    {task.summary.tracks?.map(trackLabel).join(" / ") || "—"}
                  </span>
                </span>
              </div>
            )}
          </CardBody>
        </Card>
      )}

      {artifacts.length > 0 && (
        <Card className="mt-4">
          <CardHeader
            title="分离结果"
            icon={<Waves size={15} strokeWidth={1.75} />}
            aside={<span className="micro">{artifacts.length} 轨</span>}
          />
          <CardBody className="space-y-2 p-4">
            {artifacts.map((a) => {
              const track = a.meta?.track;
              return (
                <div
                  key={a.id}
                  className="flex flex-wrap items-center gap-3 rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3"
                >
                  <span className="w-14 shrink-0 text-xs text-fg">{trackLabel(track)}</span>
                  <WavePlayer
                    src={`${apiBase}/api/artifacts/${a.id}/stream`}
                    title={a.filename}
                    sub={trackLabel(track)}
                    durationSec={a.duration_ms ? a.duration_ms / 1000 : undefined}
                    className="min-w-0 flex-1 basis-64"
                  />
                  <span className="hidden shrink-0 font-mono text-[11px] text-muted sm:inline">
                    {formatSize(a.size)}
                  </span>
                  <a
                    href={`${apiBase}/api/artifacts/${a.id}/download`}
                    className="inline-flex shrink-0 items-center gap-1 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
                  >
                    <Download size={13} strokeWidth={1.75} />
                    下载
                  </a>
                  {track === "voice" &&
                    (asrCompatible(a.format) ? (
                      <Button
                        size="sm"
                        onClick={() => navigate(`/asr?artifact=${a.id}`)}
                        className="shrink-0"
                      >
                        送 ASR 识别
                      </Button>
                    ) : (
                      <span
                        className="shrink-0 text-[11px] text-muted"
                        title="语音识别仅支持 mp3 / wav，可重新分离并选择对应格式"
                      >
                        格式暂不支持识别
                      </span>
                    ))}
                </div>
              );
            })}
          </CardBody>
        </Card>
      )}

      {!task && (
        <Card className="mt-4">
          <EmptyState
            icon={<Waves size={18} strokeWidth={1.75} />}
            title="还没有分离结果"
            description="填入公网音视频地址、选择场景后点击「开始分离」，双轨或三轨音频会出现在这里，可试听、下载，人声轨可一键送语音识别。"
          />
        </Card>
      )}
    </>
  );
}
