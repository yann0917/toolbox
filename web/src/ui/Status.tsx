import type { TaskStatus } from "../lib/types";

const statusMeta: Record<TaskStatus, { label: string; color: string; pulse: boolean }> = {
  pending: { label: "排队中", color: "text-muted", pulse: false },
  running: { label: "进行中", color: "text-accent", pulse: true },
  succeeded: { label: "已完成", color: "text-meter", pulse: false },
  failed: { label: "失败", color: "text-danger", pulse: false },
  canceled: { label: "已取消", color: "text-muted", pulse: false },
  interrupted: { label: "已中断", color: "text-warn", pulse: false },
};

export function StatusBadge({ status }: { status: TaskStatus }) {
  const m = statusMeta[status] ?? statusMeta.pending;
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs ${m.color}`}>
      <span className={`signal-dot ${m.pulse ? "signal-dot-pulse" : ""}`} />
      {m.label}
    </span>
  );
}

export function SignalDot({ tone = "accent", pulse = false }: { tone?: "accent" | "meter" | "danger" | "muted"; pulse?: boolean }) {
  const toneClass = { accent: "text-accent", meter: "text-meter", danger: "text-danger", muted: "text-muted" }[tone];
  return <span className={`signal-dot ${toneClass} ${pulse ? "signal-dot-pulse" : ""}`} />;
}

export interface ProgressBarProps {
  value: number;
  /** 运行中叠加流光 */
  active?: boolean;
  className?: string;
}

export function ProgressBar({ value, active = false, className = "" }: ProgressBarProps) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <div
      role="progressbar"
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
      className={`relative h-0.5 w-full overflow-hidden rounded-full bg-raise-2 ${className}`}
    >
      <div
        className={`h-full rounded-full bg-accent transition-[width] duration-300 ease-out ${active ? "sheen relative" : ""}`}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}
