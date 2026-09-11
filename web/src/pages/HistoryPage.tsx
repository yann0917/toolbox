import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiBase, fetchJSON } from "../lib/api";
import MiniPlayer from "../components/MiniPlayer";

interface Task { id: string; provider: string; tool: string; status: string; cost_ms: number; created_at: string }
interface Artifact { id: string; kind: string; filename: string }
interface TaskDetail { task: Task; artifacts: Artifact[] }

export default function HistoryPage() {
  const qc = useQueryClient();
  const [playing, setPlaying] = useState<string | null>(null);
  const { data } = useQuery({
    queryKey: ["tasks"],
    queryFn: () => fetchJSON<{ items: Task[]; total: number }>("/api/tasks"),
  });
  const del = useMutation({
    mutationFn: (id: string) => fetchJSON(`/api/tasks/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),
  });
  const detail = useQuery({
    queryKey: ["task", playing],
    enabled: !!playing,
    queryFn: () => fetchJSON<TaskDetail>(`/api/tasks/${playing}`),
  });

  return (
    <div className="max-w-4xl mx-auto space-y-4">
      <h1 className="text-xl font-semibold">历史任务</h1>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] divide-y divide-[var(--border)]">
        {(data?.items ?? []).map((t) => (
          <div key={t.id} className="p-4 flex items-center gap-3 text-sm">
            <span className="w-20 text-[var(--muted)]">{t.tool}</span>
            <span className="w-24">{t.status}</span>
            <span className="flex-1 text-[var(--muted)]">{t.created_at}</span>
            <button onClick={() => setPlaying(t.id)} className="text-[var(--accent)]">查看</button>
            <button onClick={() => del.mutate(t.id)} className="text-[var(--danger)]">删除</button>
          </div>
        ))}
      </div>
      {detail.data && (
        <div className="space-y-2">
          {detail.data.artifacts.map((a) => (
            <div key={a.id} className="flex items-center gap-3">
              {a.kind === "audio" && <MiniPlayer src={`${apiBase}/api/artifacts/${a.id}/stream`} title={a.filename} />}
              <a href={`${apiBase}/api/artifacts/${a.id}/download`} className="text-sm text-[var(--accent)]">下载 {a.filename}</a>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
