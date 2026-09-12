import { useId } from "react";
import type {
  InputHTMLAttributes,
  ReactNode,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
} from "react";

const control =
  "w-full rounded-[var(--radius-sm)] bg-raise-2 border border-line-strong text-fg " +
  "placeholder:text-muted transition-colors duration-150 " +
  "hover:border-[color-mix(in_oklab,var(--muted)_45%,transparent)] " +
  "focus:outline-none focus-visible:border-accent focus-visible:ring-2 focus-visible:ring-accent/30 " +
  "disabled:opacity-50 disabled:cursor-not-allowed";

export function MicroLabel({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <span className={`micro ${className}`}>{children}</span>;
}

export function Input({ className = "", ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...rest} className={`${control} h-9 px-3 text-sm ${className}`} />;
}

export function Textarea({ className = "", ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...rest} className={`${control} p-3 text-sm leading-relaxed resize-y ${className}`} />;
}

export function Select({ className = "", children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select {...rest} className={`${control} h-9 pl-3 pr-8 text-sm cursor-pointer appearance-none bg-no-repeat ${className}`}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%236c6c76' stroke-width='2' stroke-linecap='round'><path d='m6 9 6 6 6-6'/></svg>\")",
        backgroundPosition: "right 8px center",
      }}
    >
      {children}
    </select>
  );
}

export interface FieldProps {
  label: string;
  hint?: string;
  error?: string;
  required?: boolean;
  /** 右侧小字（如「可选」「默认 mp3」） */
  aside?: ReactNode;
  children: (props: { id: string; "aria-invalid"?: boolean; "aria-describedby"?: string }) => ReactNode;
}

/**
 * 刻印微标签 + 控件 + 说明/错误行的统一字段容器。
 * label 与控件通过 useId 关联，保证 a11y。
 */
export function Field({ label, hint, error, required, aside, children }: FieldProps) {
  const id = useId();
  const descId = hint || error ? `${id}-desc` : undefined;
  return (
    <div className="space-y-1.5">
      <div className="flex items-baseline justify-between gap-3">
        <label htmlFor={id} className="micro">
          {label}
          {required && <span className="text-accent ml-1">*</span>}
        </label>
        {aside && <span className="text-[11px] text-muted">{aside}</span>}
      </div>
      {children({ id, "aria-invalid": error ? true : undefined, "aria-describedby": descId })}
      {error ? (
        <p id={descId} className="text-[11px] text-danger">
          {error}
        </p>
      ) : hint ? (
        <p id={descId} className="text-[11px] text-muted">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
