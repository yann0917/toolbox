import { useCallback, useEffect, useState } from "react";

export type ThemePref = "system" | "dark" | "light";
export type ResolvedTheme = "dark" | "light";

const KEY = "theme";
const mq = () => window.matchMedia("(prefers-color-scheme: dark)");

export function readPref(): ThemePref {
  const raw = localStorage.getItem(KEY);
  return raw === "dark" || raw === "light" || raw === "system" ? raw : "system";
}

function resolve(pref: ThemePref): ResolvedTheme {
  if (pref === "system") return mq().matches ? "dark" : "light";
  return pref;
}

function apply(resolved: ResolvedTheme) {
  document.documentElement.classList.toggle("light", resolved === "light");
}

/** 主题偏好：system 跟随系统，dark/light 手动指定，localStorage 持久化。 */
export function useTheme() {
  const [pref, setPrefState] = useState<ThemePref>(readPref);
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolve(readPref()));

  useEffect(() => {
    const next = resolve(pref);
    setResolved(next);
    apply(next);
    localStorage.setItem(KEY, pref);
  }, [pref]);

  // 跟随系统时，系统切换要即时响应
  useEffect(() => {
    if (pref !== "system") return;
    const m = mq();
    const onChange = () => {
      const next = resolve("system");
      setResolved(next);
      apply(next);
    };
    m.addEventListener("change", onChange);
    return () => m.removeEventListener("change", onChange);
  }, [pref]);

  const setPref = useCallback((next: ThemePref) => setPrefState(next), []);

  return { pref, resolved, setPref };
}
