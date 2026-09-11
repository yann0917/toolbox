import { useEffect, useState } from "react";

export interface TaskEvent {
  type: "progress" | "done" | "error" | "canceled" | "task.snapshot";
  task_id?: string;
  progress?: number;
  note?: string;
  error?: string;
  tasks?: unknown[];
}

export function useTaskEvents(): TaskEvent | null {
  const [last, setLast] = useState<TaskEvent | null>(null);
  useEffect(() => {
    const url = (import.meta.env.DEV ? "ws://localhost:8080" : `ws://${location.host}`) + "/api/ws";
    let retry: ReturnType<typeof setTimeout>;
    let ws: WebSocket;
    const connect = () => {
      ws = new WebSocket(url);
      ws.onmessage = (e) => setLast(JSON.parse(e.data));
      ws.onclose = () => { retry = setTimeout(connect, 2000); };
    };
    connect();
    return () => { clearTimeout(retry); ws.close(); };
  }, []);
  return last;
}
