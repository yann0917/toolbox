import { useEffect, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowUpRight,
  Clock,
  ListChecks,
  NotebookPen,
  RefreshCw,
  SlidersHorizontal,
} from "lucide-react";
import { Link } from "react-router-dom";
import { fetchJSON } from "../lib/api";
import type { TaskDetail, TaskStatus } from "../lib/types";
import { useTaskEvents } from "../lib/ws";
import { ArtifactRow } from "../components/ArtifactRow";
import { MINUTES_PRICE, PRICE_SNAPSHOT_DATE } from "../lib/pricing";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  EmptyState,
  Field,
  Input,
  MicroLabel,
  PageHeader,
  ProgressBar,
  Skeleton,
  StatusBadge,
  useToast,
} from "../ui";

/** 附加功能（官方约束：至少一项，否则上游提交失败） */
const FEATURES = [
  { key: "summary", label: "全文总结", desc: "生成会议主题与纪要段落" },
  { key: "todo", label: "待办提取", desc: "识别待办事项与执行人" },
  { key: "qa", label: "问答提取", desc: "标记重要问句" },
  { key: "chapter", label: "章节总结", desc: "按时间轴分段总结" },
  { key: "translation", label: "翻译", desc: "转写文本中英互译" },
] as const;

/** 毫秒 → mm:ss / h:mm:ss（待办与章节时间轴展示） */
function fmtClock(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = String(m).padStart(2, "0");
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

interface Run {
  status: TaskStatus;
  progress: number;
  note: string;
  error?: string;
}

export default function MinutesPage() {
  const [url, setUrl] = useState("");
  const [features, setFeatures] = useState<string[]>(["summary"]);
  const [sourceLang, setSourceLang] = useState("zh_cn");
  const [targetLang, setTargetLang] = useState("en_us");
  const [speakers, setSpeakers] = useState(0);
  const [hotwords, setHotwords] = useState("");
  const [allActivate, setAllActivate] = useState(true);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [upstreamTaskId, setUpstreamTaskId] = useState("");
  const [run, setRun] = useState<Run | null>(null);
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [submitError, setSubmitError] = useState("");
  const urlRef = useRef<HTMLDivElement>(null);
  const focusUrl = () => urlRef.current?.querySelector("input")?.focus();
  const { toast } = useToast();
  const ev = useTaskEvents();

  /* WS 事件驱动进度；终态回读任务详情取纪要结果 */
  useEffect(() => {
    if (!ev || !taskId || ev.task_id !== taskId) return;
    if (ev.type === "progress") {
      if (ev.detail?.task_id) setUpstreamTaskId(ev.detail.task_id);
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
          if (d.task.status === "failed") {
            toast({ tone: "error", title: "妙记任务失败", description: d.task.error || undefined });
          }
        })
        .catch((e: Error) => {
          setRun((r) => ({ status: "failed", progress: r?.progress ?? 0, note: "读取任务结果失败", error: e.message }));
          toast({ tone: "error", title: "读取任务结果失败", description: e.message });
        });
    }
  }, [ev, taskId, toast]);

  const toggleFeature = (key: string) => {
    setFeatures((prev) => (prev.includes(key) ? prev.filter((f) => f !== key) : [...prev, key]));
  };

  const submit = useMutation({
    mutationFn: () =>
      fetchJSON<{ task_id: string }>("/api/tasks", {
        method: "POST",
        body: JSON.stringify({
          provider: "volcengine",
          tool: "minutes",
          params: {
            url,
            features: features.join(","),
            source_lang: sourceLang,
            target_lang: targetLang,
            speakers,
            hotwords,
            all_activate: allActivate,
          },
        }),
      }),
    onSuccess: (d) => {
      setTaskId(d.task_id);
      setRun({ status: "pending", progress: 0, note: "已提交" });
      setDetail(null);
      setSubmitError("");
    },
    onError: (e: Error) => {
      setSubmitError(e.message);
      toast({ tone: "error", title: "提交失败", description: e.message });
    },
  });

  const urlValid = /^https?:\/\//i.test(url.trim());
  const canSubmit = urlValid && features.length > 0;

  const artifacts = detail?.artifacts ?? [];
  const task = detail?.task;
  const summary = task?.summary;
  const transcriptPreview = (summary?.segments ?? []).slice(0, 200);

  return (
    <>
      <PageHeader
        title="语音妙记"
        description="音视频 URL 转结构化纪要：转写+说话人、总结、待办、章节、翻译（≤2 小时、<1G，需公网 URL）"
        icon={<NotebookPen size={16} strokeWidth={1.75} />}
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
        {/* 左：输入区 */}
        <Card className="min-w-0">
          <CardHeader title="音视频输入" icon={<NotebookPen size={15} strokeWidth={1.75} />} />
          <CardBody className="space-y-3">
            <div ref={urlRef}>
              <Field
                label="音视频 URL"
                hint="公网可访问地址（音频 MP3/WAV/AAC/FLAC/OGG，视频 MP4/AVI/MKV/MOV/FLV/WMV）；本地文件请先上传至对象存储。"
                error={url.trim() !== "" && !urlValid ? "请输入 http(s):// 开头的公网 URL" : undefined}
              >
                {({ id, ...rest }) => (
                  <Input
                    id={id}
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    placeholder="https://example.com/meeting.mp4"
                    {...rest}
                  />
                )}
              </Field>
            </div>
            <p className="text-[11px] leading-relaxed text-muted">
              妙记与 ASR 的区别：ASR 只做转写（本地文件可直发）；妙记额外生成说话人分离、总结、待办、章节等结构化纪要，
              但仅收 URL，按小时计费（转写 1.8 元/小时 + 结构费），适合会议/访谈/讲座。同量对比见
              <Link to="/pricing" className="mx-0.5 text-accent transition-colors duration-150 hover:opacity-80">
                计费测算
              </Link>
              。
            </p>
          </CardBody>
        </Card>

        {/* 右：参数面板 */}
        <Card className="lg:sticky lg:top-4 lg:self-start">
          <CardHeader
            title="纪要参数"
            icon={<SlidersHorizontal size={15} strokeWidth={1.75} />}
            aside={<span className="micro">volcengine · minutes</span>}
          />
          <CardBody className="space-y-4">
            <div className="space-y-2">
              <MicroLabel>附加功能（至少一项）</MicroLabel>
              {FEATURES.map((f) => (
                <label key={f.key} className="flex cursor-pointer items-start gap-2 rounded-[var(--radius-sm)] px-1 py-1 text-sm text-fg-2 transition-colors duration-150 hover:bg-raise-2">
                  <input
                    type="checkbox"
                    checked={features.includes(f.key)}
                    onChange={() => toggleFeature(f.key)}
                    className="mt-0.5 size-4 cursor-pointer accent-accent"
                  />
                  <span className="min-w-0">
                    <span className="text-fg">{f.label}</span>
                    <span className="block text-[11px] text-muted">{f.desc}</span>
                  </span>
                </label>
              ))}
              {features.length === 0 && (
                <p className="text-[11px] text-danger">至少选择一项附加功能，否则上游提交失败。</p>
              )}
            </div>

            {features.includes("translation") && (
              <div className="grid grid-cols-2 gap-2">
                <Field label="源语种">
                  {({ id, ...rest }) => (
                    <select
                      id={id}
                      value={sourceLang}
                      onChange={(e) => setSourceLang(e.target.value)}
                      className="h-9 w-full cursor-pointer appearance-none rounded-[var(--radius-sm)] border border-line bg-inset px-3 font-mono text-sm text-fg transition-colors duration-150 focus:border-accent"
                      {...rest}
                    >
                      <option value="zh_cn">中文</option>
                      <option value="en_us">英语</option>
                    </select>
                  )}
                </Field>
                <Field label="翻译目标语">
                  {({ id, ...rest }) => (
                    <select
                      id={id}
                      value={targetLang}
                      onChange={(e) => setTargetLang(e.target.value)}
                      className="h-9 w-full cursor-pointer appearance-none rounded-[var(--radius-sm)] border border-line bg-inset px-3 font-mono text-sm text-fg transition-colors duration-150 focus:border-accent"
                      {...rest}
                    >
                      <option value="en_us">英语</option>
                      <option value="zh_cn">中文</option>
                    </select>
                  )}
                </Field>
              </div>
            )}

            <div className="grid grid-cols-2 gap-2">
              <Field label="说话人数" hint="0 = 自动识别">
                {({ id, ...rest }) => (
                  <Input
                    id={id}
                    type="number"
                    min={0}
                    max={10}
                    value={String(speakers)}
                    onChange={(e) => setSpeakers(Number(e.target.value || 0))}
                    {...rest}
                  />
                )}
              </Field>
              <Field label="打包计费" hint="关 = 按功能数计费">
                {({ id }) => (
                  <label htmlFor={id} className="flex h-9 cursor-pointer items-center gap-2 text-sm text-fg-2">
                    <input
                      id={id}
                      type="checkbox"
                      checked={allActivate}
                      onChange={(e) => setAllActivate(e.target.checked)}
                      className="size-4 cursor-pointer accent-accent"
                    />
                    {allActivate ? "按打包价" : "按功能汇总"}
                  </label>
                )}
              </Field>
            </div>

            <Field label="热词" hint="逗号分隔，提升专有名词准确率">
              {({ id, ...rest }) => (
                <Input
                  id={id}
                  value={hotwords}
                  onChange={(e) => setHotwords(e.target.value)}
                  placeholder="如：火山引擎, 大模型"
                  {...rest}
                />
              )}
            </Field>

            <div className="border-t border-line pt-3">
              <Button
                variant="primary"
                className="w-full"
                icon={<NotebookPen size={15} strokeWidth={1.75} />}
                loading={submit.isPending}
                disabled={!canSubmit}
                onClick={() => submit.mutate()}
              >
                提交妙记任务
              </Button>
              {submitError && (
                <p className="mt-2 flex items-start gap-1.5 text-[11px] text-danger">
                  <AlertTriangle size={12} strokeWidth={1.75} className="mt-0.5 shrink-0" />
                  {submitError}
                </p>
              )}
              <p className="mt-2 text-[11px] leading-relaxed text-muted">
                计费：转写 {MINUTES_PRICE.transcriptionPerHour} 元/小时 + 结构
                {allActivate ? `（打包 ${MINUTES_PRICE.structureBundlePerHour} 元/小时）` : `（单功能 ${MINUTES_PRICE.structureSinglePerHour} 元/小时 × ${features.length}）`}
                ，刊例快照 {PRICE_SNAPSHOT_DATE}，以账单为准。生成耗时与音视频时长正相关（分钟级）。
              </p>
            </div>
          </CardBody>
        </Card>
      </div>

      {/* 结果区 */}
      <Card className="mt-4">
        <CardHeader
          title="纪要结果"
          icon={<NotebookPen size={15} strokeWidth={1.75} />}
          aside={run ? <StatusBadge status={run.status} /> : undefined}
        />
        {!run ? (
          <EmptyState
            icon={<NotebookPen size={18} strokeWidth={1.75} />}
            title="还没有妙记任务"
            description="粘贴音视频公网 URL、选择附加功能后提交，转写、总结、待办与章节会出现在这里。"
            action={
              <Button variant="secondary" size="sm" onClick={focusUrl}>
                去输入 URL
              </Button>
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
                重新提交
              </Button>
              <span className="font-mono text-[11px] text-muted">{taskId?.slice(0, 8)}</span>
            </div>
          </CardBody>
        ) : run.status !== "succeeded" ? (
          <CardBody className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <StatusBadge status={run.status} />
              <span className="font-mono text-[11px] tabular-nums text-muted">{run.progress}%</span>
            </div>
            <ProgressBar value={run.progress} active={run.status === "running" || run.status === "pending"} />
            <p className="text-xs text-muted">{run.note || "处理中"}</p>
            {upstreamTaskId && (
              <p className="font-mono text-[11px] tabular-nums text-muted" title="上游妙记任务 ID">
                任务 {upstreamTaskId}
              </p>
            )}
            <Skeleton className="h-24 w-full" />
          </CardBody>
        ) : (
          <CardBody className="space-y-4">
            {/* 全文总结 */}
            {summary?.summary_text && (
              <section className="space-y-1.5">
                <MicroLabel>全文总结</MicroLabel>
                {summary.minutes_title && <p className="text-sm font-medium text-fg">{summary.minutes_title}</p>}
                <p className="whitespace-pre-wrap break-words text-sm leading-relaxed text-fg-2">{summary.summary_text}</p>
              </section>
            )}

            {/* 待办 */}
            {summary?.todos && summary.todos.length > 0 && (
              <section className="space-y-1.5">
                <MicroLabel className="inline-flex items-center gap-1">
                  <ListChecks size={12} strokeWidth={1.75} />
                  待办事项 · {summary.todos.length}
                </MicroLabel>
                <ul className="space-y-1.5">
                  {summary.todos.map((td, i) => (
                    <li key={i} className="flex items-start gap-2 rounded-[var(--radius-sm)] border border-line bg-raise-2 px-3 py-2 text-sm">
                      <span className="font-mono text-[11px] tabular-nums text-muted">{i + 1}</span>
                      <span className="min-w-0 flex-1 break-words text-fg-2">{td.content}</span>
                      {td.executor.filter((e) => e && e !== "无").length > 0 && (
                        <span className="shrink-0 text-[11px] text-muted">{td.executor.filter((e) => e && e !== "无").join("、")}</span>
                      )}
                      {td.start_time > 0 && (
                        <span className="shrink-0 font-mono text-[11px] tabular-nums text-muted">{fmtClock(td.start_time)}</span>
                      )}
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {/* 章节 */}
            {summary?.chapters && summary.chapters.length > 0 && (
              <section className="space-y-1.5">
                <MicroLabel className="inline-flex items-center gap-1">
                  <Clock size={12} strokeWidth={1.75} />
                  章节总结 · {summary.chapters.length}
                </MicroLabel>
                <ul className="space-y-1.5">
                  {summary.chapters.map((ch, i) => (
                    <li key={i} className="rounded-[var(--radius-sm)] border border-line bg-raise-2 px-3 py-2">
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="font-mono text-[11px] tabular-nums text-muted">
                          {fmtClock(ch.start_time)} – {fmtClock(ch.end_time)}
                        </span>
                        <span className="text-sm font-medium text-fg">{ch.title}</span>
                      </div>
                      {ch.summary && <p className="mt-1 text-xs leading-relaxed text-muted">{ch.summary}</p>}
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {/* 翻译 */}
            {summary?.translation_text && (
              <section className="space-y-1.5">
                <MicroLabel>翻译文本</MicroLabel>
                <p className="max-h-48 overflow-y-auto whitespace-pre-wrap break-words rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3 text-sm leading-relaxed text-fg-2">
                  {summary.translation_text}
                </p>
              </section>
            )}

            {/* 产物 */}
            {artifacts.length > 0 && (
              <section className="space-y-2">
                {artifacts.map((a) => (
                  <ArtifactRow key={a.id} a={a} />
                ))}
              </section>
            )}

            {/* 转写预览 */}
            {transcriptPreview.length > 0 && (
              <section className="space-y-1.5">
                <MicroLabel>
                  转写预览 · {summary?.sentences ?? transcriptPreview.length} 句 · {summary?.speakers_count ?? "?"} 个说话人
                </MicroLabel>
                <div className="max-h-72 space-y-1 overflow-y-auto rounded-[var(--radius-sm)] border border-line bg-raise-2 p-3">
                  {transcriptPreview.map((seg, i) => (
                    <p key={i} className="text-[13px] leading-relaxed text-fg-2">
                      <span className="mr-2 font-mono text-[11px] tabular-nums text-muted">{fmtClock(seg.start_ms)}</span>
                      {seg.text}
                    </p>
                  ))}
                </div>
              </section>
            )}

            {/* 摘要行 */}
            <p className="pt-1 font-mono text-[11px] tabular-nums text-muted">
              {summary?.duration_ms ? `时长 ${(summary.duration_ms / 60000).toFixed(1)} 分钟` : ""}
              {summary?.sentences ? ` · ${summary.sentences} 句` : ""}
              {summary?.speakers_count ? ` · ${summary.speakers_count} 个说话人` : ""}
              {summary?.features?.length ? ` · ${summary.features.join("/")}` : ""}
              {task && task.cost_ms > 0 ? ` · ${(task.cost_ms / 1000).toFixed(1)}s` : ""}
              {upstreamTaskId ? ` · ${upstreamTaskId}` : ""}
            </p>
          </CardBody>
        )}
      </Card>
    </>
  );
}
