/** 与后端 DTO 对齐的共享类型（internal/server/dto.go）。 */

export type TaskStatus = "pending" | "running" | "succeeded" | "failed" | "canceled" | "interrupted";

export interface Task {
  id: string;
  provider: string;
  tool: string;
  status: TaskStatus;
  progress: number;
  progress_note: string;
  error?: string;
  cost_ms: number;
  created_at: string;
  /** 仅任务详情接口返回：provider.TaskOutput.Summary 的 JSON */
  summary?: {
    segments?: { text: string; start_ms: number; end_ms: number }[];
    rounds?: number;
    duration_s?: number;
    duration_ms?: number;
    tracks?: string[];
    scene?: string;
    speakers?: string[];
    source?: string;
    char_count?: number;
    audio_url_fallback?: boolean;
  };
}

export type ArtifactKind = "audio" | "transcript" | "dialog" | "subtitle";

export interface Artifact {
  id: string;
  kind: ArtifactKind;
  filename: string;
  format: string;
  size: number;
  duration_ms: number;
  meta?: { track?: "voice" | "background" | "music" | "sfx"; [k: string]: unknown };
}

export interface TaskDetail {
  task: Task;
  artifacts: Artifact[];
}

export interface Voice {
  id: string;
  gender: string;
  category: string;
}
