import { useEffect, useState } from "react";

export interface TaskEvent {
  type: "progress" | "done" | "error" | "canceled" | "task.snapshot";
  task_id?: string;
  progress?: number;
  note?: string;
  error?: string;
  tasks?: unknown[];
  // 仅 progress 事件携带的工具自定义展示数据（后端 task.Event.Detail，如播客对话流轮次）。
  detail?: { round_id?: number; speaker?: string; text?: string; rounds_done?: number; task_id?: string; poll?: number; status?: string; chunks?: number };
}

export function useTaskEvents(): TaskEvent | null {
  const [last, setLast] = useState<TaskEvent | null>(null);
  useEffect(() => {
    let disposed = false;
    let ws: WebSocket | null = null;
    let retry: ReturnType<typeof setTimeout> | undefined;
    const connect = () => {
      if (disposed) return;
      // 同源相对连接：dev 经 vite 代理（ws: true），https 部署自动用 wss
      const wsProto = location.protocol === "https:" ? "wss:" : "ws:";
      const url = `${wsProto}//${location.host}/api/ws`;
      ws = new WebSocket(url);
      ws.onmessage = (e) => {
        try {
          setLast(JSON.parse(e.data) as TaskEvent);
        } catch {
          // 忽略坏帧
        }
      };
      ws.onclose = () => {
        if (!disposed) retry = setTimeout(connect, 2000);
      };
    };
    connect();
    return () => {
      disposed = true;
      if (retry) clearTimeout(retry);
      ws?.close();
    };
  }, []);
  return last;
}
