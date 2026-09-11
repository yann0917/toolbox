export default function MiniPlayer({ src, title }: { src: string; title: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 flex items-center gap-3">
      <audio controls src={src} className="flex-1" />
      <span className="text-sm text-[var(--muted)] shrink-0">{title}</span>
    </div>
  );
}
