export default function TaskProgress({ task }: { task: { status: string; progress: number; progress_note: string; error?: string } }) {
  const color = task.status === "succeeded" ? "var(--ok)" : task.status === "failed" ? "var(--danger)" : "var(--accent)";
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 space-y-2">
      <div className="flex justify-between text-sm">
        <span>{task.progress_note || "处理中"}</span>
        <span style={{ color }}>{task.status}</span>
      </div>
      <div className="h-1.5 rounded-full bg-[var(--surface-hover)] overflow-hidden">
        <div className="h-full rounded-full transition-all"
          style={{ width: `${task.progress}%`, background: color }} />
      </div>
      {task.error && <p className="text-xs" style={{ color: "var(--danger)" }}>{task.error}</p>}
    </div>
  );
}
