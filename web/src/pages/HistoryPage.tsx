import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Clock, Download, RefreshCw, Trash2 } from "lucide-react";
import { apiBase, fetchJSON } from "../lib/api";
import type { Artifact, Task, TaskDetail } from "../lib/types";
import {
  Card,
  CardHeader,
  ConfirmDialog,
  EmptyState,
  IconButton,
  PageHeader,
  Skeleton,
  StatusBadge,
  WavePlayer,
  useToast,
} from "../ui";

const toolName: Record<string, string> = {
  tts: "语音合成",
  asr: "语音识别",
  podcast: "播客工坊",
  separate: "人声分离",
};

const artifactLabel: Record<string, string> = {
  audio: "音频",
  transcript: "转写文本",
  subtitle: "字幕",
  dialog: "对话稿",
};

const trackLabel: Record<string, string> = {
  voice: "人声轨",
  background: "背景音轨",
  music: "音乐轨",
  sfx: "音效轨",
};

function formatSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function ArtifactRow({ a }: { a: Artifact }) {
  const isAudio = a.kind === "audio";
  const label = a.meta?.track ? trackLabel[a.meta.track] : artifactLabel[a.kind] ?? a.kind;
  return (
    <div className="flex items-center gap-3 rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3">
      <span className="w-16 shrink-0 text-xs text-fg-2">{label}</span>
      {isAudio ? (
        <WavePlayer
          src={`${apiBase}/api/artifacts/${a.id}/stream`}
          title={a.filename}
          sub={label}
          durationSec={a.duration_ms ? a.duration_ms / 1000 : undefined}
          className="min-w-0 flex-1"
        />
      ) : (
        <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted">{a.filename}</span>
      )}
      <span className="hidden shrink-0 font-mono text-[11px] text-muted sm:inline">{formatSize(a.size)}</span>
      <a
        href={`${apiBase}/api/artifacts/${a.id}/download`}
        className="inline-flex shrink-0 items-center gap-1 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
      >
        <Download size={13} strokeWidth={1.75} />
        下载
      </a>
    </div>
  );
}

const filters = [
  { value: "", label: "全部" },
  { value: "tts", label: "语音合成" },
  { value: "asr", label: "语音识别" },
  { value: "podcast", label: "播客工坊" },
  { value: "separate", label: "人声分离" },
];

export default function HistoryPage() {
  const qc = useQueryClient();
  const { toast } = useToast();
  const [selected, setSelected] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Task | null>(null);
  const [toolFilter, setToolFilter] = useState("");

  const list = useQuery({
    queryKey: ["tasks", "history"],
    queryFn: () => fetchJSON<{ items: Task[]; total: number }>("/api/tasks?size=50"),
  });

  const detail = useQuery({
    queryKey: ["task", selected],
    enabled: Boolean(selected),
    queryFn: () => fetchJSON<TaskDetail>(`/api/tasks/${selected}`),
  });

  const del = useMutation({
    mutationFn: (id: string) => fetchJSON(`/api/tasks/${id}`, { method: "DELETE" }),
    onSuccess: (_d, id) => {
      toast({ tone: "ok", title: "任务已删除" });
      if (selected === id) setSelected(null);
      setPendingDelete(null);
      void qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => toast({ tone: "error", title: "删除失败", description: e.message }),
  });

  const items = (list.data?.items ?? []).filter((t) => !toolFilter || t.tool === toolFilter);

  return (
    <>
      <PageHeader
        title="历史"
        description="全部任务与产物，可回放、下载、删除"
        actions={
          <>
            <div className="hidden items-center gap-0.5 rounded-[var(--radius-sm)] border border-line bg-raise-2 p-0.5 sm:flex">
              {filters.map((f) => (
                <button
                  key={f.value || "all"}
                  onClick={() => setToolFilter(f.value)}
                  className={`cursor-pointer rounded-[5px] px-2.5 py-1 text-xs transition-colors duration-150 ${
                    toolFilter === f.value ? "bg-raise text-fg" : "text-muted hover:text-fg-2"
                  }`}
                >
                  {f.label}
                </button>
              ))}
            </div>
            <IconButton label="刷新" onClick={() => void qc.invalidateQueries({ queryKey: ["tasks"] })}>
              <RefreshCw size={15} strokeWidth={1.75} />
            </IconButton>
          </>
        }
      />

      <Card>
        <CardHeader
          title="任务列表"
          icon={<Clock size={15} strokeWidth={1.75} />}
          aside={<span className="micro">{items.length} 条</span>}
        />
        {list.isLoading ? (
          <div className="space-y-2 p-4">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            icon={<Clock size={18} strokeWidth={1.75} />}
            title="没有匹配的任务"
            description="换个筛选条件，或先去工具页发起一个任务。"
          />
        ) : (
          <ul className="divide-y divide-line">
            {items.map((t) => (
              <li key={t.id}>
                <div className="flex items-center gap-3 px-4 py-2.5 text-sm transition-colors duration-150 hover:bg-raise-2">
                  <button
                    onClick={() => setSelected(selected === t.id ? null : t.id)}
                    className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 text-left"
                  >
                    <span className="w-20 shrink-0 text-fg-2">{toolName[t.tool] ?? t.tool}</span>
                    <StatusBadge status={t.status} />
                    <span className="min-w-0 flex-1 truncate text-xs text-muted">
                      {t.progress_note || t.error || "—"}
                    </span>
                  </button>
                  <span className="hidden shrink-0 font-mono text-[11px] text-muted md:inline">
                    {t.cost_ms > 0 ? `${(t.cost_ms / 1000).toFixed(1)}s` : "—"}
                  </span>
                  <span className="shrink-0 font-mono text-[11px] text-muted">{t.created_at}</span>
                  <IconButton
                    label="删除任务"
                    size="sm"
                    variant="ghost"
                    className="hover:text-danger"
                    onClick={() => setPendingDelete(t)}
                  >
                    <Trash2 size={14} strokeWidth={1.75} />
                  </IconButton>
                </div>

                {selected === t.id && (
                  <div className="rise space-y-2 border-t border-line bg-panel px-4 py-3">
                    {detail.isLoading ? (
                      <Skeleton className="h-12 w-full" />
                    ) : detail.data && detail.data.artifacts.length > 0 ? (
                      detail.data.artifacts.map((a) => <ArtifactRow key={a.id} a={a} />)
                    ) : (
                      <p className="py-2 text-xs text-muted">
                        {t.error ? `失败原因：${t.error}` : "该任务没有产物"}
                      </p>
                    )}
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </Card>

      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title="删除任务"
        description={`将删除「${
          pendingDelete ? toolName[pendingDelete.tool] ?? pendingDelete.tool : ""
        }」任务记录及其产物文件，且不可恢复。`}
        confirmLabel="删除"
        loading={del.isPending}
        onConfirm={() => pendingDelete && del.mutate(pendingDelete.id)}
        onCancel={() => setPendingDelete(null)}
      />
    </>
  );
}
