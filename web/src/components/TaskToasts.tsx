import { useEffect } from "react";
import { onTaskEvent } from "../lib/ws";
import { toolLabel } from "../lib/toolNames";
import { useToast } from "../ui";

/** 全局任务通知：订阅 WS 事件流，终态（完成/失败/取消）弹 toast。
 *  progress 不弹——推送高频且页面内已有进度条；长任务提交后可离开页面，靠此获知结果。
 *  必须挂在 ToastProvider 内（Layout 满足）。 */
export default function TaskToasts() {
  const { toast } = useToast();
  useEffect(
    () =>
      onTaskEvent((ev) => {
        if (ev.type !== "done" && ev.type !== "error" && ev.type !== "canceled") return;
        const tool = ev.tool ? toolLabel(ev.tool) : "";
        const name = tool ? `${tool}任务` : "任务";
        const ref = ev.task_id ? `任务 ${ev.task_id.slice(0, 8)}` : "";
        switch (ev.type) {
          case "done":
            toast({ tone: "ok", title: `${name}完成`, description: ref });
            break;
          case "error":
            // 描述给具体错误（可能较长，Toast 已 break-words）
            toast({ tone: "error", title: `${name}失败`, description: [ref, ev.error].filter(Boolean).join("：") });
            break;
          case "canceled":
            toast({ tone: "warn", title: `${name}已取消`, description: ref });
            break;
        }
      }),
    [toast]
  );
  return null;
}
