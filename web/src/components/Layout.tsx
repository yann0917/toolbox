import { useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import {
  Activity,
  AudioLines,
  Calculator,
  History,
  Info,
  Languages,
  LayoutDashboard,
  Menu,
  Mic,
  Monitor,
  Moon,
  NotebookPen,
  Podcast,
  Settings,
  Sun,
  Waves,
  X,
} from "lucide-react";
import { useTheme, type ThemePref } from "../lib/theme";
import { usePlayer } from "../lib/player";
import { useWSStatus } from "../lib/ws";
import PlayerBar from "./PlayerBar";
import TaskToasts from "./TaskToasts";
import { IconButton } from "../ui";

const nav = [
  { to: "/", label: "工作台", desc: "工具总览与最近任务", icon: LayoutDashboard },
  { to: "/tts", label: "语音合成", desc: "同步/流式/长文本三通道", icon: AudioLines },
  { to: "/asr", label: "语音识别", desc: "音频转文字与字幕", icon: Mic },
  { to: "/podcast", label: "播客工坊", desc: "生成双人播客", icon: Podcast },
  { to: "/separate", label: "人声分离", desc: "人声与背景音分轨", icon: Waves },
  { to: "/translate", label: "机器翻译", desc: "32 语种互译与术语定制", icon: Languages },
  { to: "/minutes", label: "语音妙记", desc: "音视频转结构化纪要", icon: NotebookPen },
  { to: "/history", label: "历史", desc: "全部任务与产物", icon: History },
  { to: "/pricing", label: "计费测算", desc: "刊例价用量估算", icon: Calculator },
  { to: "/settings", label: "设置", desc: "凭证与连接", icon: Settings },
  { to: "/about", label: "关于", desc: "产品与使用指南", icon: Info },
];

const themeOrder: ThemePref[] = ["system", "dark", "light"];
const themeMeta = {
  system: { icon: Monitor, label: "主题：跟随系统" },
  dark: { icon: Moon, label: "主题：暗色" },
  light: { icon: Sun, label: "主题：亮色" },
};

function HealthIndicator() {
  // 徽标跟随 WS 事件通道状态：WS 经 HTTP 升级建立，"open"即服务可达且实时通道可用，
  // 比轮询 /api/health 更强也更相关（任务进度依赖这条通道）。服务端 ping/pong 保证
  // 状态真实；断了 2 秒自动重连，无需人工处理。
  const status = useWSStatus();
  const meta = {
    open: { label: "实时连接", cls: "text-meter", title: "事件通道已连接，任务进度实时推送" },
    connecting: { label: "连接中", cls: "text-warn", title: "正在建立事件通道" },
    closed: { label: "已断开", cls: "text-danger", title: "事件通道中断，将自动重连" },
  }[status];
  return (
    <span
      className={`hidden items-center gap-1.5 text-[11px] sm:inline-flex ${meta.cls}`}
      title={meta.title}
      role="status"
    >
      <Activity size={13} strokeWidth={1.75} />
      {meta.label}
    </span>
  );
}

function NavItems({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <>
      {nav.map(({ to, label, icon: Icon }) => (
        <NavLink
          key={to}
          to={to}
          onClick={onNavigate}
          className={({ isActive }) =>
            `group relative flex items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-sm transition-colors duration-150 ${
              isActive ? "bg-raise text-fg" : "text-fg-2 hover:bg-raise-2 hover:text-fg"
            }`
          }
        >
          {({ isActive }) => (
            <>
              <span
                className={`absolute left-0 top-1.5 bottom-1.5 w-[2px] rounded-full ${isActive ? "bg-accent" : "bg-transparent"}`}
              />
              <Icon size={18} strokeWidth={1.75} className={isActive ? "text-accent shrink-0" : "shrink-0"} />
              <span className="hidden truncate lg:inline">{label}</span>
            </>
          )}
        </NavLink>
      ))}
    </>
  );
}

export default function Layout() {
  const { pref, setPref } = useTheme();
  const [navOpen, setNavOpen] = useState(false);
  const hasTrack = usePlayer((s) => Boolean(s.track));
  const { pathname } = useLocation();
  const current = nav.find((n) => n.to === pathname) ?? nav[0];
  const ThemeIcon = themeMeta[pref].icon;

  const cycleTheme = () => setPref(themeOrder[(themeOrder.indexOf(pref) + 1) % themeOrder.length]);

  return (
    <div className="flex h-screen bg-bg">
      {/* 左侧导轨：lg 全宽，md 图标态 */}
      <aside className="hidden shrink-0 flex-col border-r border-line bg-panel md:flex md:w-16 lg:w-[232px]">
        <div className="flex h-14 items-center gap-2.5 border-b border-line px-3 lg:px-4">
          <div className="flex size-7 shrink-0 items-center justify-center rounded-[var(--radius-sm)] bg-accent text-accent-ink">
            <AudioLines size={16} strokeWidth={2} />
          </div>
          <span className="hidden text-sm font-semibold tracking-tight lg:inline">toolbox</span>
        </div>
        <nav className="flex flex-col gap-0.5 p-2">
          <NavItems />
        </nav>
        <div className="mt-auto hidden p-4 lg:block">
          <p className="micro leading-relaxed opacity-70">
            火山引擎
            <br />
            语音能力控制台
          </p>
        </div>
      </aside>

      {/* 窄屏抽屉 */}
      {navOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-[2px]" onClick={() => setNavOpen(false)} />
          <aside className="rise absolute left-0 top-0 h-full w-[248px] border-r border-line bg-panel">
            <div className="flex h-14 items-center justify-between border-b border-line px-3">
              <span className="text-sm font-semibold">toolbox</span>
              <IconButton label="关闭导航" onClick={() => setNavOpen(false)}>
                <X size={16} strokeWidth={1.75} />
              </IconButton>
            </div>
            <nav className="flex flex-col gap-0.5 p-2">
              <NavItems onNavigate={() => setNavOpen(false)} />
            </nav>
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        {/* 顶栏：只放全局控件与当前位置，页面标题由各页 PageHeader 承担（避免重复） */}
        <header className="flex h-14 shrink-0 items-center gap-3 border-b border-line bg-panel/70 px-3 backdrop-blur md:px-6">
          <IconButton label="打开导航" className="md:hidden" onClick={() => setNavOpen(true)}>
            <Menu size={18} strokeWidth={1.75} />
          </IconButton>
          <nav aria-label="当前位置" className="micro flex min-w-0 items-center gap-1.5 truncate">
            <span>toolbox</span>
            <span className="text-line-strong">/</span>
            <span className="text-fg-2">{current.label}</span>
          </nav>
          <div className="ml-auto flex items-center gap-3">
            <HealthIndicator />
            <IconButton label={themeMeta[pref].label} onClick={cycleTheme}>
              <ThemeIcon size={16} strokeWidth={1.75} />
            </IconButton>
          </div>
        </header>

        <main className={`min-h-0 flex-1 overflow-y-auto ${hasTrack ? "pb-24" : ""}`}>
          <div className="mx-auto max-w-[1100px] px-4 py-6 md:px-8 md:py-8">
            <Outlet />
          </div>
        </main>
      </div>

      <PlayerBar />
      <TaskToasts />
    </div>
  );
}
