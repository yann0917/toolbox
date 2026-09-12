import { useEffect, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";

const nav = [
  { to: "/", label: "工作台", icon: "◎" },
  { to: "/tts", label: "语音合成", icon: "🗣" },
  { to: "/asr", label: "语音识别", icon: "🎙" },
  { to: "/podcast", label: "播客工坊", icon: "🎧" },
  { to: "/separate", label: "人声分离", icon: "🎚" },
  { to: "/history", label: "历史", icon: "🕘" },
  { to: "/settings", label: "设置", icon: "⚙" },
];

export default function Layout() {
  const [theme, setTheme] = useState<"dark" | "light">(
    () => (localStorage.getItem("theme") as "dark" | "light") ?? "dark"
  );
  useEffect(() => {
    document.documentElement.classList.toggle("light", theme === "light");
    localStorage.setItem("theme", theme);
  }, [theme]);

  return (
    <div className="flex h-screen">
      <aside className="w-56 border-r border-[var(--border)] bg-[var(--surface)] p-4 flex flex-col gap-1">
        <div className="text-lg font-semibold px-2 pb-4">toolbox</div>
        {nav.map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            className={({ isActive }) =>
              `px-3 py-2 rounded-lg text-sm transition-colors ${
                isActive ? "bg-[var(--surface-hover)] text-[var(--accent)]" : "text-[var(--muted)] hover:text-[var(--fg)]"
              }`
            }
          >
            <span className="mr-2">{n.icon}</span>{n.label}
          </NavLink>
        ))}
        <div className="mt-auto">
          <button
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            className="w-full px-3 py-2 rounded-lg text-sm text-[var(--muted)] hover:text-[var(--fg)] border border-[var(--border)]"
          >
            {theme === "dark" ? "☀ 亮色" : "☾ 暗色"}
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-y-auto p-8">
        <Outlet />
      </main>
    </div>
  );
}
