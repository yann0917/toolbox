import { useQuery } from "@tanstack/react-query";
import { fetchJSON } from "./api";

export interface SettingsShape {
  volc: {
    speech: { app_id: string; has_access_token: boolean; api_key: string };
    mediakit: { has_api_key: boolean };
  };
  storage?: {
    provider: string;
    endpoint: string;
    region: string;
    bucket: string;
    access_key: string;
    has_secret_key: boolean;
    prefix: string;
    lifecycle_days: number;
    enabled: boolean;
  };
  data_dir: string;
}

/**
 * 对象存储配置与启用态（与设置页共用 ["settings"] 缓存：设置页保存后此 hook 自动刷新）。
 * enabled = provider 已配置且连接参数齐全，URL-only 工具页据此展示「本地上传」通道。
 */
export function useStorageEnabled() {
  const q = useQuery({
    queryKey: ["settings"],
    queryFn: () => fetchJSON<SettingsShape>("/api/settings"),
  });
  return {
    storage: q.data?.storage,
    enabled: !!q.data?.storage?.enabled,
    isLoading: q.isLoading,
  };
}
