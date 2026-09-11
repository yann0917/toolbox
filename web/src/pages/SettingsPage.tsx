import type { FormEvent } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { fetchJSON } from "../lib/api";

interface Settings { volc: { speech: { app_id: string; has_access_token: boolean; api_key: string }; mediakit: { has_api_key: boolean } }; data_dir: string }

export default function SettingsPage() {
  const { data, refetch } = useQuery({ queryKey: ["settings"], queryFn: () => fetchJSON<Settings>("/api/settings") });
  const save = useMutation({
    mutationFn: (body: Record<string, string>) =>
      fetchJSON("/api/settings", { method: "PUT", body: JSON.stringify(body) }),
    onSuccess: () => refetch(),
  });
  const test = useMutation({
    mutationFn: () => fetchJSON<{ ok: boolean; message: string }>("/api/settings/test-connection", { method: "POST" }),
  });

  const field = (name: string, label: string, placeholder: string) => (
    <label className="block space-y-1">
      <span className="text-sm text-[var(--muted)]">{label}</span>
      <input name={name} placeholder={placeholder} defaultValue=""
        className="w-full rounded-lg bg-[var(--bg)] border border-[var(--border)] px-3 py-2 text-sm" />
    </label>
  );

  const onSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    save.mutate({
      app_id: String(fd.get("app_id") ?? ""),
      access_token: String(fd.get("access_token") ?? ""),
      api_key: String(fd.get("api_key") ?? ""),
      mediakit_api_key: String(fd.get("mediakit_api_key") ?? ""),
    });
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">设置</h1>
      <form onSubmit={onSubmit} className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-4">
        <div className="text-sm">
          APP ID：<span className="text-[var(--muted)]">{data?.volc.speech.app_id || "未配置"}</span>
          ，Access Token：{data?.volc.speech.has_access_token ? "已配置" : <span className="text-[var(--warn)]">未配置</span>}
        </div>
        {field("app_id", "火山语音 APP ID", "留空则不修改")}
        {field("access_token", "Access Token", "留空则不修改")}
        {field("api_key", "新版 API Key（二选一）", "留空则不修改")}
        <button className="px-5 py-2 rounded-lg bg-[var(--accent)] text-[var(--accent-fg)] text-sm font-medium">保存</button>
        {save.isSuccess && <p className="text-xs" style={{ color: "var(--ok)" }}>已保存，重启服务后生效</p>}
      </form>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 space-y-3">
        <button type="button" onClick={() => test.mutate()}
          className="px-5 py-2 rounded-lg border border-[var(--border)] text-sm">测试语音凭证连通性</button>
        {test.data && <p className="text-sm" style={{ color: test.data.ok ? "var(--ok)" : "var(--danger)" }}>{test.data.message}</p>}
        <p className="text-xs text-[var(--muted)]">数据目录：{data?.data_dir}</p>
      </div>
    </div>
  );
}
