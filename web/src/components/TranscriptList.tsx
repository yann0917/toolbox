import { formatTime } from "../lib/player";
import type { SyncSegment } from "../lib/useTranscriptSync";

interface TranscriptListProps {
  segments: SyncSegment[];
  /** 当前播放句下标（-1 = 无），由 useTranscriptSync 计算 */
  activeIdx?: number;
  /** 传入即可点击跳播；不传为只读预览 */
  onSeek?: (ms: number) => void;
  /** 时间码格式：默认 mm:ss.d；长音频可传 h:mm:ss */
  formatTimecode?: (ms: number) => string;
  maxHeightClass?: string;
  className?: string;
}

/** 分句文稿列表：时间码 + 文本，当前播放句高亮（ASR / 妙记 / 历史回放共用）。 */
export function TranscriptList({
  segments,
  activeIdx = -1,
  onSeek,
  formatTimecode = (ms) => formatTime(ms / 1000),
  maxHeightClass = "max-h-96",
  className = "",
}: TranscriptListProps) {
  if (segments.length === 0) return null;
  return (
    <div className={`overflow-hidden rounded-[var(--radius-sm)] border border-line ${className}`}>
      <ul className={`${maxHeightClass} divide-y divide-line overflow-y-auto`}>
        {segments.map((seg, i) => {
          const active = i === activeIdx;
          const inner = (
            <>
              <span
                className={`shrink-0 font-mono text-[11px] tabular-nums ${active ? "text-accent" : "text-muted"}`}
              >
                [{formatTimecode(seg.start_ms)}]
              </span>
              <span className="min-w-0 flex-1 text-sm leading-relaxed">{seg.text}</span>
            </>
          );
          return (
            <li key={i}>
              {onSeek ? (
                <button
                  type="button"
                  onClick={() => onSeek(seg.start_ms)}
                  aria-current={active || undefined}
                  className={`flex w-full cursor-pointer items-baseline gap-3 border-l-2 px-4 py-2.5 text-left transition-colors duration-150 ${
                    active ? "border-accent bg-raise-2" : "border-transparent hover:bg-raise-2"
                  }`}
                >
                  {inner}
                </button>
              ) : (
                <div
                  className={`flex items-baseline gap-3 border-l-2 px-4 py-2.5 ${
                    active ? "border-accent bg-raise-2" : "border-transparent"
                  }`}
                >
                  {inner}
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
