import { useState, type FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { CheckCircle2, Eye, EyeOff, FolderOpen, HardDrive, KeyRound, PlugZap, XCircle } from "lucide-react";
import { fetchJSON } from "../lib/api";
import {
  Button,
  Card,
  CardBody,
  CardHeader,
  Field,
  IconButton,
  Input,
  MicroLabel,
  PageHeader,
  Skeleton,
  useToast,
} from "../ui";

interface Settings {
  volc: {
    speech: { app_id: string; has_access_token: boolean; api_key: string };
    mediakit: { has_api_key: boolean };
  };
  data_dir: string;
}
interface ConnResult {
  ok: boolean;
  message: string;
  mediakit?: { ok: boolean; message: string };
}

/** 敏感输入：默认隐藏 + 显示切换 */
function SecretInput({
  name,
  placeholder,
  id,
  ...rest
}: { name: string; placeholder: string; id?: string } & Record<string, unknown>) {
  const [show, setShow] = useState(false);
  return (
    <div className="relative">
      <Input
        id={id}
        name={name}
        type={show ? "text" : "password"}
        placeholder={placeholder}
        autoComplete="off"
        className="pr-10"
        {...rest}
      />
      <div className="absolute right-1 top-1/2 -translate-y-1/2">
        <IconButton
          label={show ? "隐藏内容" : "显示内容"}
          size="sm"
          type="button"
          onClick={() => setShow((v) => !v)}
        >
          {show ? <EyeOff size={14} strokeWidth={1.75} /> : <Eye size={14} strokeWidth={1.75} />}
        </IconButton>
      </div>
    </div>
  );
}

function ConnBadge({ result }: { result?: { ok: boolean; message: string } }) {
  if (!result) return null;
  const Icon = result.ok ? CheckCircle2 : XCircle;
  return (
    <span
      className={`inline-flex items-center gap-1.5 text-xs ${result.ok ? "text-meter" : "text-danger"}`}
      role="status"
    >
      <Icon size={14} strokeWidth={1.75} />
      <span className="break-all">{result.message}</span>
    </span>
  );
}

export default function SettingsPage() {
  const { toast } = useToast();
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["settings"],
    queryFn: () => fetchJSON<Settings>("/api/settings"),
  });

  const save = useMutation({
    mutationFn: (body: Record<string, string>) =>
      fetchJSON("/api/settings", { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => {
      toast({ tone: "ok", title: "凭证已保存", description: "重启 Web 服务后生效。" });
      void refetch();
    },
    onError: (e: Error) => toast({ tone: "error", title: "保存失败", description: e.message }),
  });

  const test = useMutation({
    mutationFn: () => fetchJSON<ConnResult>("/api/settings/test-connection", { method: "POST" }),
    onError: (e: Error) => toast({ tone: "error", title: "连通性测试失败", description: e.message }),
  });

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const fd = new FormData(form);
    save.mutate(
      {
        app_id: String(fd.get("app_id") ?? ""),
        access_token: String(fd.get("access_token") ?? ""),
        api_key: String(fd.get("api_key") ?? ""),
        mediakit_api_key: String(fd.get("mediakit_api_key") ?? ""),
      },
      { onSuccess: () => form.reset() }
    );
  };

  const speech = data?.volc.speech;
  const mediakit = data?.volc.mediakit;

  return (
    <>
      <PageHeader title="设置" description="两套独立凭证体系：火山语音与 AI MediaKit，各自配置与测试" />

      <form onSubmit={onSubmit} className="space-y-4">
        <Card>
          <CardHeader
            title="火山语音 · TTS / ASR / 播客"
            icon={<KeyRound size={15} strokeWidth={1.75} />}
            aside={
              isLoading ? (
                <Skeleton className="h-4 w-24" />
              ) : (
                <span className="micro">
                  {speech?.app_id ? (
                    <>
                      APP ID <span className="font-mono text-fg-2">{speech.app_id}</span>
                    </>
                  ) : (
                    "未配置"
                  )}
                </span>
              )
            }
          />
          <CardBody className="space-y-4 p-4">
            <p className="text-xs text-muted">
              用于语音合成、语音识别与播客生成。配置项：<code className="font-mono text-fg-2">volc.speech.app_id</code>、
              <code className="font-mono text-fg-2">volc.speech.access_token</code>（或新版
              <code className="font-mono text-fg-2">volc.speech.api_key</code>）。
            </p>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="APP ID" hint="留空表示不修改">
                {({ id, ...rest }) => <Input id={id} name="app_id" placeholder="火山控制台获取" {...rest} />}
              </Field>
              <Field label="Access Token" hint={speech?.has_access_token ? "当前已配置" : "当前未配置"}>
                {({ id, ...rest }) => (
                  <SecretInput id={id} name="access_token" placeholder="留空表示不修改" {...rest} />
                )}
              </Field>
            </div>
            <Field label="新版 API Key（与上面二选一）" hint="新版控制台可只配此项">
              {({ id, ...rest }) => <SecretInput id={id} name="api_key" placeholder="留空表示不修改" {...rest} />}
            </Field>
          </CardBody>
        </Card>

        <Card>
          <CardHeader
            title="AI MediaKit · 人声分离"
            icon={<HardDrive size={15} strokeWidth={1.75} />}
            aside={
              <span className={`micro ${mediakit?.has_api_key ? "" : "text-warn"}`}>
                {mediakit?.has_api_key ? "已配置" : "未配置"}
              </span>
            }
          />
          <CardBody className="space-y-4 p-4">
            <p className="text-xs text-muted">
              仅用于人声背景音分离，与火山语音是两套独立凭证。配置项：
              <code className="font-mono text-fg-2">volc.mediakit.api_key</code>。
            </p>
            <Field label="MediaKit API Key" hint="在 AI MediaKit 控制台创建">
              {({ id, ...rest }) => (
                <SecretInput id={id} name="mediakit_api_key" placeholder="留空表示不修改" {...rest} />
              )}
            </Field>
          </CardBody>
        </Card>

        <div className="flex items-center gap-3">
          <Button type="submit" variant="primary" loading={save.isPending}>
            保存凭证
          </Button>
          <span className="text-[11px] text-muted">保存后需重启 Web 服务生效</span>
        </div>
      </form>

      <Card className="mt-4">
        <CardHeader title="连通性测试" icon={<PlugZap size={15} strokeWidth={1.75} />} />
        <CardBody className="space-y-3 p-4">
          <div className="flex flex-wrap items-center gap-3">
            <Button
              variant="secondary"
              onClick={() => test.mutate()}
              loading={test.isPending}
              icon={<PlugZap size={14} strokeWidth={1.75} />}
            >
              测试两套凭证
            </Button>
            <ConnBadge result={test.data ? { ok: test.data.ok, message: `火山语音：${test.data.message}` } : undefined} />
            <ConnBadge
              result={
                test.data?.mediakit
                  ? { ok: test.data.mediakit.ok, message: `MediaKit：${test.data.mediakit.message}` }
                  : undefined
              }
            />
          </div>
          <p className="text-[11px] text-muted">
            语音测试会发起一次极短的合成请求（消耗少量额度）；MediaKit 测试只做鉴权探测。
          </p>
        </CardBody>
      </Card>

      <Card className="mt-4">
        <CardHeader title="存储位置" icon={<FolderOpen size={15} strokeWidth={1.75} />} />
        <CardBody className="space-y-2 p-4">
          <MicroLabel>数据目录</MicroLabel>
          <p className="break-all font-mono text-xs text-fg-2">{data?.data_dir ?? "—"}</p>
          <p className="text-[11px] text-muted">
            产物文件（音频、转写、字幕、对话稿）与任务数据库都保存在此目录；配置文件为
            <code className="font-mono"> ~/.toolbox/config.yaml</code>（权限 0600）。
          </p>
        </CardBody>
      </Card>
    </>
  );
}
