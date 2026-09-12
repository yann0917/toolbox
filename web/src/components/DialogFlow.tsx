import { useEffect, useRef } from "react";

export interface DialogFlowItem {
  round: number;
  speaker: string;
  text: string;
}

// 音色短名：剥去「语种_性别_」前缀后取第一段（如 zh_female_cancan_mars_bigtts → cancan；
// 对话稿自定义说话人如「张三」原样保留）。
function speakerShort(s: string): string {
  const head = s.replace(/^[a-z]+_(?:male|female)_/, "").split("_")[0];
  return head || s;
}

// 对话流：逐轮追加的播客对话条目，容器限高滚动、新条目自动滚到底。
export default function DialogFlow({ items }: { items: DialogFlowItem[] }) {
  const boxRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const el = boxRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [items]);

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 space-y-2">
      <h2 className="text-sm font-medium text-[var(--muted)]">对话流</h2>
      <div ref={boxRef} className="max-h-64 overflow-y-auto space-y-1.5 text-sm">
        {items.map((it, i) => (
          <p key={i} className="leading-relaxed break-words">
            <span className="text-[var(--muted)]">[第 {it.round} 轮]</span>{" "}
            <span className="text-[var(--accent)]">{speakerShort(it.speaker)}</span>
            <span className="text-[var(--muted)]">：</span>
            {it.text}
          </p>
        ))}
      </div>
    </div>
  );
}
