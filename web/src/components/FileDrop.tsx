import { useRef, useState, type DragEvent } from "react";
import { AlertTriangle, Upload, X } from "lucide-react";
import { IconButton } from "../ui";

interface FileDropProps {
  file: File | null;
  onFile: (f: File | null) => void;
  /** 文件选择器的 accept 值（如 ".mp3,.wav"，须与页面文案/后端白名单一致） */
  accept: string;
  /** 空态提示（支持的格式说明） */
  emptyHint: string;
  /** 选择区域标签（无障碍） */
  label: string;
  /** 校验失败等错误信息 */
  error?: string;
}

/** 本地文件拖放/点选区：人声分离与妙记的「本地上传」输入通道（文件经服务端中转对象存储）。 */
export function FileDrop({ file, onFile, accept, emptyHint, label, error }: FileDropProps) {
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(false);
    const f = e.dataTransfer.files?.[0];
    if (f) onFile(f);
  };

  return (
    <div className="space-y-2">
      <div
        role="button"
        tabIndex={0}
        aria-label={label}
        onClick={() => inputRef.current?.click()}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            inputRef.current?.click();
          }
        }}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={`flex cursor-pointer flex-col items-center justify-center gap-2 rounded-[var(--radius-md)] border border-dashed px-4 py-7 text-center transition-colors duration-150 ${
          dragging ? "border-accent bg-raise-2" : "border-line-strong bg-raise-2/40 hover:border-accent"
        }`}
      >
        <span className={`flex size-9 items-center justify-center rounded-full border border-line bg-raise ${dragging ? "text-accent" : "text-muted"}`}>
          <Upload size={16} strokeWidth={1.75} />
        </span>
        {file ? (
          <>
            <p className="max-w-full truncate text-sm text-fg">{file.name}</p>
            <p className="font-mono text-[11px] tabular-nums text-muted">{formatBytes(file.size)}</p>
          </>
        ) : (
          <>
            <p className="text-sm text-fg-2">拖拽文件到此处，或点击选择文件</p>
            <p className="text-[11px] text-muted">{emptyHint}</p>
          </>
        )}
        <input
          ref={inputRef}
          type="file"
          accept={accept}
          aria-label={label}
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) onFile(f);
            e.target.value = "";
          }}
        />
      </div>
      {file && (
        <div className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-muted">{file.name}</span>
          <IconButton
            label="清除已选文件"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              onFile(null);
            }}
          >
            <X size={14} strokeWidth={1.75} />
          </IconButton>
        </div>
      )}
      {error && (
        <p className="flex items-start gap-1.5 text-[11px] text-danger">
          <AlertTriangle size={12} strokeWidth={1.75} className="mt-0.5 shrink-0" />
          {error}
        </p>
      )}
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
