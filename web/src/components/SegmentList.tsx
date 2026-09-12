// 识别结果句子列表：文本 + [mm:ss] 起始时间，点击回调 onSeek(startMS)。
export interface Segment {
  text: string;
  start_ms: number;
  end_ms: number;
}

// ms → mm:ss（超一小时累积到分钟位，识别分句场景足够）。
function fmt(ms: number): string {
  const total = Math.floor(ms / 1000);
  const mm = String(Math.floor(total / 60)).padStart(2, "0");
  const ss = String(total % 60).padStart(2, "0");
  return `${mm}:${ss}`;
}

export default function SegmentList({ segments, onSeek }: { segments: Segment[]; onSeek: (ms: number) => void }) {
  if (segments.length === 0) return null;
  return (
    <ul className="rounded-xl border border-[var(--border)] bg-[var(--surface)] divide-y divide-[var(--border)] max-h-96 overflow-y-auto">
      {segments.map((seg, i) => (
        <li key={i}>
          <button
            onClick={() => onSeek(seg.start_ms)}
            className="w-full text-left px-4 py-2 text-sm flex gap-3 items-baseline hover:bg-[var(--surface-hover)] transition-colors"
          >
            <span className="text-[var(--accent)] font-mono text-xs shrink-0">[{fmt(seg.start_ms)}]</span>
            <span className="flex-1">{seg.text}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}
