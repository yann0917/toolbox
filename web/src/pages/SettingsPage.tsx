import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bell,
  CheckCircle2,
  CloudUpload,
  Eye,
  EyeOff,
  FolderOpen,
  HardDrive,
  KeyRound,
  PlugZap,
  RotateCw,
  XCircle,
} from "lucide-react";
import { fetchJSON } from "../lib/api";
import type { SettingsShape } from "../lib/useStorageEnabled";
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
  Select,
  Skeleton,
  useToast,
} from "../ui";
interface ConnResult {
  ok: boolean;
  message: string;
  mediakit?: { ok: boolean; message: string };
  storage?: { ok: boolean; message: string };
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
  const [notifyOn, setNotifyOn] = useState(
    typeof localStorage !== "undefined" && localStorage.getItem("sysnotify") === "on"
  );
  const notifyPermission =
    typeof Notification !== "undefined" ? Notification.permission : "不支持";
  const { toast } = useToast();
  const qc = useQueryClient();
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["settings"],
    queryFn: () => fetchJSON<SettingsShape>("/api/settings"),
  });

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      fetchJSON("/api/settings", { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => {
      toast({ tone: "ok", title: "凭证已保存", description: "已即时生效，无需重启服务。" });
      void refetch();
    },
    onError: (e: Error) => toast({ tone: "error", title: "保存失败", description: e.message }),
  });

  const saveStorage = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      fetchJSON("/api/settings", { method: "PUT", body: JSON.stringify({ storage: body }) }),
    onSuccess: () => {
      toast({ tone: "ok", title: "存储配置已保存", description: "下一任务即使用新存储通道。" });
      void refetch();
      void qc.invalidateQueries({ queryKey: ["settings"] });
    },
    onError: (e: Error) => toast({ tone: "error", title: "存储配置保存失败", description: e.message }),
  });

  const test = useMutation({
    mutationFn: () => fetchJSON<ConnResult>("/api/settings/test-connection", { method: "POST" }),
    onError: (e: Error) => toast({ tone: "error", title: "连通性测试失败", description: e.message }),
  });

  const applyLifecycle = useMutation({
    mutationFn: () => fetchJSON<{ message: string }>("/api/storage/lifecycle", { method: "POST", body: JSON.stringify({}) }),
    onSuccess: (d) => {
      toast({ tone: "ok", title: "生命周期规则已应用", description: d.message });
      void refetch();
    },
    onError: (e: Error) => toast({ tone: "error", title: "应用生命周期规则失败", description: e.message }),
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
      <PageHeader title="设置" description="火山语音 / MediaKit 凭证、对象存储中转通道，保存即时生效" />

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
          <CardBody className="space-y-4">
            <p className="text-xs text-muted">
              播客生成<strong className="text-fg">必须 APP ID + Access Token</strong>
              （播客协议只认这对凭证，不支持新版 API Key）；TTS / 语音识别两者皆可——
              <code className="font-mono text-fg-2">volc.speech.app_id</code> +
              <code className="font-mono text-fg-2">volc.speech.access_token</code>，或新版
              <code className="font-mono text-fg-2">volc.speech.api_key</code> 单键。
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
            <Field label="新版 API Key（仅 TTS / 语音识别可用）" hint="播客不支持此键，仍需上方 APP ID + Access Token">
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
          <CardBody className="space-y-4">
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
        </div>
      </form>

      {data?.storage && (
        <Card className="mt-4">
          <CardHeader
            title="对象存储 · 本地文件中转"
            icon={<CloudUpload size={15} strokeWidth={1.75} />}
            aside={
              <span className={`micro ${data.storage.enabled ? "" : "text-warn"}`}>
                {data.storage.enabled ? "已启用" : "未启用"}
              </span>
            }
          />
          <CardBody className="space-y-4">
            <p className="text-xs text-muted">
              语音识别（闲时/极速版）、人声分离、语音妙记的上游只收公网 URL；配置对象存储后，本地上传的文件会在任务执行时
              <strong className="text-fg">自动转存并换取签名 URL</strong>
              ，上游用完即弃。推荐火山引擎 TOS；阿里 OSS / 腾讯 COS 等 S3 兼容通道规划中。
            </p>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const fd = new FormData(e.currentTarget);
                saveStorage.mutate({
                  provider: String(fd.get("provider") ?? ""),
                  endpoint: String(fd.get("endpoint") ?? "").trim(),
                  region: String(fd.get("region") ?? "").trim(),
                  bucket: String(fd.get("bucket") ?? "").trim(),
                  access_key: String(fd.get("access_key") ?? "").trim(),
                  secret_key: String(fd.get("secret_key") ?? ""),
                  prefix: String(fd.get("prefix") ?? "").trim(),
                  lifecycle_days: Number(fd.get("lifecycle_days") ?? 0) || 0,
                });
              }}
              className="space-y-4"
            >
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="存储类型" hint="留空（未启用）表示不使用对象存储">
                  {({ id, ...rest }) => (
                    <Select id={id} name="provider" defaultValue={data.storage!.provider} {...rest}>
                      <option value="">未启用</option>
                      <option value="tos">火山引擎 TOS</option>
                    </Select>
                  )}
                </Field>
                <Field label="Endpoint" hint="如 tos-cn-beijing.volces.com（与桶所在地域一致）">
                  {({ id, ...rest }) => (
                    <Input id={id} name="endpoint" defaultValue={data.storage!.endpoint} placeholder="tos-cn-beijing.volces.com" {...rest} />
                  )}
                </Field>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Region" hint="如 cn-beijing">
                  {({ id, ...rest }) => (
                    <Input id={id} name="region" defaultValue={data.storage!.region} placeholder="cn-beijing" {...rest} />
                  )}
                </Field>
                <Field label="Bucket" hint="私有读即可，上传对象经签名 URL 访问">
                  {({ id, ...rest }) => <Input id={id} name="bucket" defaultValue={data.storage!.bucket} placeholder="my-audio-bucket" {...rest} />}
                </Field>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="Access Key" hint="火山引擎 IAM 的 AK">
                  {({ id, ...rest }) => <Input id={id} name="access_key" defaultValue={data.storage!.access_key} autoComplete="off" {...rest} />}
                </Field>
                <Field label="Secret Key" hint={data.storage!.has_secret_key ? "当前已配置，留空表示不修改" : "火山引擎 IAM 的 SK"}>
                  {({ id, ...rest }) => <SecretInput id={id} name="secret_key" placeholder="留空表示不修改" {...rest} />}
                </Field>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label="对象前缀" hint="可留空；上传对象落在 前缀/日期/ 下">
                  {({ id, ...rest }) => <Input id={id} name="prefix" defaultValue={data.storage!.prefix} placeholder="toolbox" {...rest} />}
                </Field>
                <Field label="生命周期（天）" hint="0 = 不设置规则；推荐 3：音频处理完即无用，到期自动清理">
                  {({ id, ...rest }) => (
                    <Input id={id} name="lifecycle_days" type="number" min={0} max={3650} defaultValue={data.storage!.lifecycle_days || 0} {...rest} />
                  )}
                </Field>
              </div>
              <div className="flex flex-wrap items-center gap-3">
                <Button type="submit" variant="primary" loading={saveStorage.isPending}>
                  保存存储配置
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  loading={applyLifecycle.isPending}
                  icon={<RotateCw size={14} strokeWidth={1.75} />}
                  onClick={() => applyLifecycle.mutate()}
                >
                  应用自动清理规则
                </Button>
                <span className="text-[11px] text-muted">
                  按「前缀 + 天数」写入桶生命周期规则（保留桶上其他规则）；天数填 0 时按上方配置值执行。
                </span>
              </div>
            </form>
          </CardBody>
        </Card>
      )}

      <Card className="mt-4">
        <CardHeader title="通知" icon={<Bell size={15} strokeWidth={1.75} />} />
        <CardBody className="space-y-3">
          <label className="flex cursor-pointer items-start gap-2 text-sm text-fg-2">
            <input
              type="checkbox"
              checked={notifyOn}
              onChange={(e) => {
                if (e.target.checked) {
                  if (!("Notification" in window)) {
                    toast({ tone: "error", title: "当前浏览器不支持系统通知" });
                    return;
                  }
                  void Notification.requestPermission().then((p) => {
                    if (p === "granted") {
                      localStorage.setItem("sysnotify", "on");
                      setNotifyOn(true);
                      toast({ tone: "ok", title: "系统通知已开启", description: "页面在后台时，任务终态会发系统通知" });
                    } else {
                      toast({ tone: "error", title: "浏览器拒绝了通知权限", description: "请在浏览器地址栏的站点设置中允许通知" });
                    }
                  });
                } else {
                  localStorage.setItem("sysnotify", "off");
                  setNotifyOn(false);
                }
              }}
              className="mt-0.5 size-4 cursor-pointer accent-accent"
            />
            <span>
              任务终态系统通知
              <span className="block text-[11px] text-muted">
                页面在后台时，任务完成/失败/取消发系统级通知（需要浏览器授权）。
                当前权限：{notifyPermission}
              </span>
            </span>
          </label>
        </CardBody>
      </Card>

      <Card className="mt-4">
        <CardHeader title="连通性测试" icon={<PlugZap size={15} strokeWidth={1.75} />} />
        <CardBody className="space-y-3">
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
            <ConnBadge
              result={
                test.data?.storage
                  ? { ok: test.data.storage.ok, message: `对象存储：${test.data.storage.message}` }
                  : undefined
              }
            />
          </div>
          <p className="text-[11px] text-muted">
            语音测试会发起一次极短的合成请求（消耗少量额度）；MediaKit 测试只做鉴权探测；对象存储测试为桶探活（HeadBucket，不计费）。
          </p>
        </CardBody>
      </Card>

      <Card className="mt-4">
        <CardHeader title="存储位置" icon={<FolderOpen size={15} strokeWidth={1.75} />} />
        <CardBody className="space-y-2">
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
