import { useEffect, useRef, useState, type DragEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowUpRight,
  Captions,
  Download,
  FileAudio,
  FileText,
  Link2,
  Mic,
  RefreshCw,
  SlidersHorizontal,
  Upload,
  X,
} from "lucide-react";
import { apiBase, fetchJSON } from "../lib/api";
import { formatTime, subscribeTime, usePlayer } from "../lib/player";
import type { Artifact, TaskDetail, TaskStatus } from "../lib/types";
import { useTaskEvents } from "../lib/ws";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  EmptyState,
  Field,
  IconButton,
  Input,
  PageHeader,
  ProgressBar,
  Select,
  Skeleton,
  StatusBadge,
  Tabs,
  WavePlayer,
  useToast,
} from "../ui";

/** 音频格式白名单与后端 ASR Tool 一致（mp3/wav/ogg/pcm） */
const ACCEPT = ".mp3,.wav,.ogg,.pcm";
const ALLOWED_EXT = ["mp3", "wav", "ogg", "pcm"];

type Mode = "upload" | "url";
/** 识别版本：standard 本地文件/URL 全支持；idle/flash 仅公网 URL（闲时 24h 内完成 / 极速秒级同步） */
type ASRVersion = "standard" | "idle" | "flash";

/** URL 输入提示随版本变化（闲时/极速版 format 由 URL 扩展名推断，与后端 audioFormatFromURL 一致） */
const URL_HINT: Record<ASRVersion, string> = {
  standard: "需公网可访问的 mp3 / wav / ogg / pcm 音频地址",
  idle: "公网音频地址（wav/mp3/ogg/spx/amr/aac/m4a），最大 512MB / 5 小时",
  flash: "公网音频地址（wav/mp3/ogg/spx/amr/aac/m4a），最大 100MB / 2 小时",
};

type Segment = { text: string; start_ms: number; end_ms: number };

/** 录音文件识别支持语种（与后端 ParamSpecs 同词表）；留空 = 自动识别中文/英文及常见方言 */
const ASR_LANGUAGES: { value: string; label: string }[] = [
  { value: "zh-CN", label: "中文普通话" },
  { value: "en-US", label: "英语" },
  { value: "ja-JP", label: "日语" },
  { value: "id-ID", label: "印尼语" },
  { value: "es-MX", label: "西班牙语" },
  { value: "pt-BR", label: "葡萄牙语" },
  { value: "de-DE", label: "德语" },
  { value: "fr-FR", label: "法语" },
  { value: "ko-KR", label: "韩语" },
  { value: "fil-PH", label: "菲律宾语" },
  { value: "ms-MY", label: "马来语" },
  { value: "th-TH", label: "泰语" },
  { value: "ar-SA", label: "阿拉伯语" },
  { value: "it-IT", label: "意大利语" },
  { value: "bn-BD", label: "孟加拉语" },
  { value: "el-GR", label: "希腊语" },
  { value: "nl-NL", label: "荷兰语" },
  { value: "ru-RU", label: "俄语" },
  { value: "tr-TR", label: "土耳其语" },
  { value: "vi-VN", label: "越南语" },
  { value: "pl-PL", label: "波兰语" },
  { value: "ro-RO", label: "罗马尼亚语" },
  { value: "ne-NP", label: "尼泊尔语" },
  { value: "uk-UA", label: "乌克兰语" },
  { value: "yue-CN", label: "粤语" },
];

/** 任务运行态：只保留界面需要的字段，不伪造完整 Task DTO */
interface Run {
  status: TaskStatus;
  progress: number;
  note: string;
  error?: string;
}

function formatSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** 非音频产物行：图标 + 文件名 + 大小 + 下载 */
function DownloadRow({ a }: { a: Artifact }) {
  const Icon = a.kind === "subtitle" ? Captions : FileText;
  return (
    <a
      href={`${apiBase}/api/artifacts/${a.id}/download`}
      className="flex items-center gap-3 rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3 transition-colors duration-150 hover:border-line-strong"
    >
      <span className="shrink-0 text-muted">
        <Icon size={16} strokeWidth={1.75} />
      </span>
      <span className="min-w-0 flex-1 truncate text-sm text-fg">{a.filename}</span>
      <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted">{formatSize(a.size)}</span>
      <Download size={14} strokeWidth={1.75} className="shrink-0 text-muted" />
    </a>
  );
}

export default function ASRPage() {
  /* 跨工具联动：/asr?artifact=<id>（来自人声分离页「送 ASR识别」）→ 跳过输入区，
     以 artifact_input 直接提交识别任务。 */
  const [searchParams] = useSearchParams();
  const artifactId = searchParams.get("artifact")?.trim() ?? "";
  const artifactMode = artifactId !== "";

  const [mode, setMode] = useState<Mode>("upload");
  const [version, setVersion] = useState<ASRVersion>("standard");
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [url, setUrl] = useState("");
  const [language, setLanguage] = useState("");
  const [hotwords, setHotwords] = useState("");
  const [taskId, setTaskId] = useState<string | null>(null);
  const [run, setRun] = useState<Run | null>(null);
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [segments, setSegments] = useState<Segment[]>([]);
  const [playSrc, setPlaySrc] = useState<string | null>(null);
  const [activeIdx, setActiveIdx] = useState(-1);
  const [submitError, setSubmitError] = useState("");
  const [fileError, setFileError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);
  const blobRef = useRef<string | null>(null); // 上传回放的对象 URL（换任务时释放）
  const qc = useQueryClient();
  const { toast } = useToast();
  const ev = useTaskEvents();

  /* WS 事件驱动当前任务进度；终态拉详情拿产物与 summary.segments */
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") {
      setRun({ status: "running", progress: ev.progress ?? 0, note: ev.note ?? "处理中" });
      return;
    }
    if (ev.type === "done" || ev.type === "error" || ev.type === "canceled") {
      fetchJSON<TaskDetail>(`/api/tasks/${taskId}`)
        .then((d) => {
          setRun({
            status: d.task.status,
            progress: d.task.progress,
            note: d.task.progress_note,
            error: d.task.error,
          });
          setDetail(d);
          setSegments(d.task.summary?.segments ?? []);
          if (d.task.status === "failed") {
            toast({ tone: "error", title: "识别失败", description: d.task.error || undefined });
          }
        })
        .catch((e: Error) => {
          setRun((r) => ({ status: "failed", progress: r?.progress ?? 0, note: "读取任务结果失败", error: e.message }));
          toast({ tone: "error", title: "读取任务结果失败", description: e.message });
        });
    }
  }, [ev, taskId, toast]);

  const artifacts = detail?.artifacts ?? [];
  const downloads = artifacts.filter((a) => a.kind !== "audio");
  const durationSec = detail?.task.summary?.duration_ms ? detail.task.summary.duration_ms / 1000 : undefined;

  const playTitle = artifactMode
    ? "人声轨（分离产物）"
    : mode === "upload"
      ? file?.name ?? "本地上传音频"
      : "远程音频 URL";
  const playSub = artifactMode
    ? `产物 ${artifactId.slice(0, 8)}`
    : mode === "upload"
      ? file
        ? formatSize(file.size)
        : undefined
      : url.trim() || undefined;

  /** 切换识别版本：闲时/极速仅收公网 URL，从本地上传自动切到 URL 模式。 */
  const changeVersion = (v: ASRVersion) => {
    setVersion(v);
    if (v !== "standard") {
      setMode((m) => (m === "upload" ? "url" : m));
      setFileError("");
    }
  };

  const submit = useMutation({
    mutationFn: async () => {
      const params: Record<string, unknown> = {
        srt: true,
        language: language.trim(), // 留空 = 服务端自动识别语种/方言
      };
      if (!artifactMode) params.version = version;
      if (hotwords.trim()) params.hotwords = hotwords.trim();
      if (artifactMode) {
        // artifact_input 模式：已有人声轨产物直接作为输入（params 仅 srt/language，无 url/file_ids）。
        return fetchJSON<{ task_id: string }>("/api/tasks", {
          method: "POST",
          body: JSON.stringify({ provider: "volcengine", tool: "asr", params, artifact_input: artifactId }),
        });
      }
      if (mode === "url") {
        params.url = url.trim();
        return fetchJSON<{ task_id: string }>("/api/tasks", {
          method: "POST",
          body: JSON.stringify({ provider: "volcengine", tool: "asr", params }),
        });
      }
      // 本地上传：先 POST /api/uploads 拿 file_id（不设 Content-Type，让浏览器带 multipart boundary），
      // 再建任务走 file_ids 本地文件通道。
      const fd = new FormData();
      fd.append("file", file!);
      const up = await fetchJSON<{ file_id: string }>("/api/uploads", { method: "POST", body: fd, headers: {} });
      return fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({ provider: "volcengine", tool: "asr", params, file_ids: [up.file_id] }),
      });
    },
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setRun({ status: "pending", progress: 0, note: "已提交" });
      setDetail(null);
      setSegments([]);
      setSubmitError("");
      if (artifactMode) {
        setPlaySrc(`${apiBase}/api/artifacts/${artifactId}/stream`);
      } else if (mode === "upload" && file) {
        if (blobRef.current) URL.revokeObjectURL(blobRef.current);
        blobRef.current = URL.createObjectURL(file);
        setPlaySrc(blobRef.current);
      } else {
        setPlaySrc(url.trim());
      }
      void qc.invalidateQueries({ queryKey: ["tasks"] });
    },
    onError: (e: Error) => {
      setSubmitError(e.message);
      toast({ tone: "error", title: "提交失败", description: e.message });
    },
  });

  /* 分句高亮：跟随全局播放通道的时间推进（rAF 通道，不触发 60fps 重渲染），
     仅当当前句变化时才 setState。 */
  useEffect(() => {
    if (segments.length === 0) return;
    const unsub = subscribeTime((t) => {
      if (usePlayer.getState().track?.src !== playSrc) {
        setActiveIdx((prev) => (prev === -1 ? prev : -1));
        return;
      }
      const ms = t * 1000;
      let idx = -1;
      for (let i = 0; i < segments.length; i++) {
        if (ms >= segments[i].start_ms && ms < segments[i].end_ms) {
          idx = i;
          break;
        }
      }
      setActiveIdx((prev) => (prev === idx ? prev : idx));
    });
    return () => {
      unsub();
    };
  }, [segments, playSrc]);

  // 点击句子 → 回放跳转到该句起始时间；若当前播放的不是本任务的音频，先切过去
  const seekTo = (ms: number) => {
    if (!playSrc) return;
    const st = usePlayer.getState();
    if (st.track?.src !== playSrc) st.play({ id: playSrc, src: playSrc, title: playTitle, sub: playSub }, true);
    st.seek(ms / 1000);
  };

  const pickFile = (f: File) => {
    const ext = f.name.split(".").pop()?.toLowerCase() ?? "";
    if (!ALLOWED_EXT.includes(ext)) {
      setFileError(`不支持的格式 .${ext || "未知"}：仅支持 mp3 / wav / ogg / pcm`);
      return;
    }
    setFileError("");
    setFile(f);
  };

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(false);
    const f = e.dataTransfer.files?.[0];
    if (f) pickFile(f);
  };

  const canSubmit = artifactMode || (mode === "upload" ? file != null : url.trim() !== "");
  const isCurrentTrack = usePlayer((s) => s.track?.src === playSrc && playSrc !== null);
  // 只有当前播放的正是本任务的音频、且分句非空时，才标记「当前句」
  const shownActive = isCurrentTrack && segments.length > 0 ? activeIdx : -1;

  return (
    <>
      <PageHeader
        title="语音识别"
        description="音频转文字，输出分句时间戳与 SRT 字幕"
        actions={
          <Link
            to="/history"
            className="inline-flex items-center gap-1 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
          >
            历史产物
            <ArrowUpRight size={13} strokeWidth={1.75} />
          </Link>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        {/* 左：音频输入（artifact 联动时为来源横幅） */}
        <Card className="min-w-0">
          <CardHeader
            title={artifactMode ? "输入来源" : "音频输入"}
            icon={<FileAudio size={15} strokeWidth={1.75} />}
            aside={<span className="micro">{artifactMode ? "artifact_input" : mode === "upload" ? "本地文件" : "公网 URL"}</span>}
          />
          <CardBody className="space-y-4">
            {artifactMode ? (
              <div className="space-y-3">
                <div className="flex flex-wrap items-center gap-3 rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3">
                  <span className="flex size-7 shrink-0 items-center justify-center rounded-[var(--radius-sm)] border border-line text-accent">
                    <FileAudio size={14} strokeWidth={1.75} />
                  </span>
                  <span className="min-w-0 flex-1 text-xs text-fg-2">
                    使用人声分离产物{" "}
                    <span className="font-mono text-accent">{artifactId.slice(0, 8)}</span> 直接识别，无需再上传
                  </span>
                  <Link
                    to="/asr"
                    className="shrink-0 text-xs text-fg-2 transition-colors duration-150 hover:text-accent"
                  >
                    改用其他音频
                  </Link>
                </div>
                <p className="text-[11px] text-muted">
                  提交时以 artifact_input 通道传入该产物，提交后可在下方试听源音轨。
                </p>
              </div>
            ) : (
              <>
                <Field
                  label="识别版本"
                  hint={
                    version === "idle"
                      ? "闲时任务在服务端持续等待结果，可关闭页面，完成后在历史产物查看"
                      : undefined
                  }
                >
                  {() => (
                    <Tabs<ASRVersion>
                      items={[
                        { value: "standard", label: "标准版" },
                        { value: "idle", label: "闲时版" },
                        { value: "flash", label: "极速版" },
                      ]}
                      value={version}
                      onChange={changeVersion}
                    />
                  )}
                </Field>

                <Tabs<Mode>
                  items={[
                    ...(version === "standard"
                      ? [{ value: "upload" as const, label: "本地上传", icon: <Upload size={13} strokeWidth={1.75} /> }]
                      : []),
                    { value: "url" as const, label: "音频 URL", icon: <Link2 size={13} strokeWidth={1.75} /> },
                  ]}
                  value={mode}
                  onChange={(m) => {
                    setMode(m);
                    setFileError("");
                  }}
                />

                {mode === "upload" ? (
                  <div className="space-y-2">
                    <div
                      role="button"
                      tabIndex={0}
                      aria-label="选择或拖入音频文件"
                      onClick={() => fileInputRef.current?.click()}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          fileInputRef.current?.click();
                        }
                      }}
                      onDragOver={(e) => {
                        e.preventDefault();
                        setDragging(true);
                      }}
                      onDragLeave={() => setDragging(false)}
                      onDrop={onDrop}
                      className={`flex cursor-pointer flex-col items-center justify-center gap-2 rounded-[var(--radius-md)] border border-dashed px-4 py-8 text-center transition-colors duration-150 ${
                        dragging ? "border-accent bg-raise-2" : "border-line-strong bg-raise-2/40 hover:border-accent"
                      }`}
                    >
                      <span className={`flex size-9 items-center justify-center rounded-full border border-line bg-raise ${dragging ? "text-accent" : "text-muted"}`}>
                        <Upload size={16} strokeWidth={1.75} />
                      </span>
                      {file ? (
                        <>
                          <p className="max-w-full truncate text-sm text-fg">{file.name}</p>
                          <p className="font-mono text-[11px] tabular-nums text-muted">{formatSize(file.size)}</p>
                        </>
                      ) : (
                        <>
                          <p className="text-sm text-fg-2">拖拽音频到此处，或点击选择文件</p>
                          <p className="text-[11px] text-muted">支持 mp3 / wav / ogg / pcm</p>
                        </>
                      )}
                      <input
                        ref={fileInputRef}
                        type="file"
                        accept={ACCEPT}
                        aria-label="选择要识别的音频文件"
                        className="hidden"
                        onChange={(e) => {
                          const f = e.target.files?.[0];
                          if (f) pickFile(f);
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
                            setFile(null);
                            setFileError("");
                          }}
                        >
                          <X size={14} strokeWidth={1.75} />
                        </IconButton>
                      </div>
                    )}
                    {fileError && (
                      <p className="flex items-start gap-1.5 text-[11px] text-danger">
                        <AlertTriangle size={12} strokeWidth={1.75} className="mt-0.5 shrink-0" />
                        {fileError}
                      </p>
                    )}
                  </div>
                ) : (
                  <Field label="音频 URL" hint={URL_HINT[version]}>
                    {({ id, ...rest }) => (
                      <Input
                        id={id}
                        value={url}
                        onChange={(e) => setUrl(e.target.value)}
                        placeholder="https://example.com/audio.mp3"
                        {...rest}
                      />
                    )}
                  </Field>
                )}
              </>
            )}
          </CardBody>
        </Card>

        {/* 右：参数面板 */}
        <Card className="lg:sticky lg:top-4 lg:self-start">
          <CardHeader
            title="识别参数"
            icon={<SlidersHorizontal size={15} strokeWidth={1.75} />}
            aside={<span className="micro">volcengine · asr</span>}
          />
          <CardBody className="space-y-4">
            <Field label="语言" hint="留空自动识别：中文、英文及上海/闽南/四川/陕西/粤语方言">
              {({ id, ...rest }) => (
                <Select
                  id={id}
                  value={language}
                  onChange={(e) => setLanguage(e.target.value)}
                  {...rest}
                >
                  <option value="">自动识别</option>
                  {ASR_LANGUAGES.map((l) => (
                    <option key={l.value} value={l.value}>
                      {l.label} {l.value}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <Field label="热词" aside="可选" hint="逗号分隔，用于提升专有名词识别率">
              {({ id, ...rest }) => (
                <Input
                  id={id}
                  value={hotwords}
                  onChange={(e) => setHotwords(e.target.value)}
                  placeholder="火山引擎,语音合成"
                  {...rest}
                />
              )}
            </Field>

            <div className="border-t border-line pt-3">
              <Button
                variant="primary"
                className="w-full"
                icon={<Mic size={15} strokeWidth={1.75} />}
                loading={submit.isPending}
                disabled={!canSubmit}
                onClick={() => submit.mutate()}
              >
                开始识别
              </Button>
              {!canSubmit && (
                <p className="mt-2 text-[11px] text-muted">
                  {mode === "upload" ? "请先选择音频文件" : "请先填写音频 URL"}
                </p>
              )}
              {submitError && (
                <p className="mt-2 flex items-start gap-1.5 text-[11px] text-danger">
                  <AlertTriangle size={12} strokeWidth={1.75} className="mt-0.5 shrink-0" />
                  {submitError}
                </p>
              )}
            </div>
          </CardBody>
        </Card>
      </div>

      {/* 结果区 */}
      <Card className="mt-4">
        <CardHeader
          title="识别结果"
          icon={<FileText size={15} strokeWidth={1.75} />}
          aside={
            run ? (
              <StatusBadge status={run.status} />
            ) : segments.length > 0 ? (
              <span className="font-mono text-[11px] tabular-nums text-muted">{segments.length} 句</span>
            ) : undefined
          }
        />

        {!run ? (
          <EmptyState
            icon={<Mic size={18} strokeWidth={1.75} />}
            title="还没有识别结果"
            description="上传本地音频或填写音频 URL 后开始识别，分句时间戳与字幕会显示在这里。"
            action={
              artifactMode ? (
                <Button variant="primary" size="sm" loading={submit.isPending} onClick={() => submit.mutate()}>
                  开始识别该音轨
                </Button>
              ) : (
                <Button variant="secondary" size="sm" icon={<Upload size={13} strokeWidth={1.75} />} onClick={() => fileInputRef.current?.click()}>
                  选择音频文件
                </Button>
              )
            }
          />
        ) : run.status === "failed" ? (
          <CardBody className="space-y-3">
            <p className="flex items-start gap-2 text-sm text-danger">
              <AlertTriangle size={15} strokeWidth={1.75} className="mt-0.5 shrink-0" />
              <span className="min-w-0 break-words">{run.error || "任务失败，请重试"}</span>
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                icon={<RefreshCw size={13} strokeWidth={1.75} />}
                loading={submit.isPending}
                onClick={() => submit.mutate()}
              >
                重试
              </Button>
              <span className="font-mono text-[11px] text-muted">{taskId?.slice(0, 8)}</span>
            </div>
          </CardBody>
        ) : (
          <CardBody className="space-y-3">
            {playSrc && (
              <div className="flex items-center gap-3 rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3">
                <span className="w-14 shrink-0 text-xs text-fg-2">音频源</span>
                <WavePlayer
                  src={playSrc}
                  title={playTitle}
                  sub={playSub}
                  durationSec={durationSec}
                  className="min-w-0 flex-1"
                />
              </div>
            )}

            {run.status !== "succeeded" ? (
              <div className="space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="text-xs text-muted">{run.note || "处理中"}</span>
                  <span className="font-mono text-[11px] tabular-nums text-muted">{run.progress}%</span>
                </div>
                <ProgressBar value={run.progress} active={run.status === "running" || run.status === "pending"} />
                {[0, 1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-9 w-full" />
                ))}
              </div>
            ) : segments.length > 0 ? (
              <div className="overflow-hidden rounded-[var(--radius-sm)] border border-line">
                <ul className="max-h-96 divide-y divide-line overflow-y-auto">
                  {segments.map((seg, i) => (
                    <li key={i}>
                      <button
                        type="button"
                        onClick={() => seekTo(seg.start_ms)}
                        aria-current={shownActive === i ? "true" : undefined}
                        className={`flex w-full cursor-pointer items-baseline gap-3 border-l-2 px-4 py-2.5 text-left transition-colors duration-150 ${
                          shownActive === i
                            ? "border-accent bg-raise-2"
                            : "border-transparent hover:bg-raise-2"
                        }`}
                      >
                        <span
                          className={`shrink-0 font-mono text-[11px] tabular-nums ${
                            shownActive === i ? "text-accent" : "text-muted"
                          }`}
                        >
                          [{formatTime(seg.start_ms / 1000)}]
                        </span>
                        <span className="min-w-0 flex-1 text-sm leading-relaxed">{seg.text}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ) : (
              <p className="py-2 text-xs text-muted">未识别到分句内容，可直接下载转写文本查看。</p>
            )}
          </CardBody>
        )}
      </Card>

      {/* 产物下载：非音频产物行式列出 */}
      {downloads.length > 0 && (
        <Card className="mt-4">
          <CardHeader
            title="产物"
            icon={<Download size={15} strokeWidth={1.75} />}
            aside={<span className="font-mono text-[11px] tabular-nums text-muted">{downloads.length} 个文件</span>}
          />
          <CardBody className="space-y-2">
            {downloads.map((a) => (
              <DownloadRow key={a.id} a={a} />
            ))}
          </CardBody>
        </Card>
      )}
    </>
  );
}
