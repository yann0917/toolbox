# Design System Master File — toolbox

> **LOGIC:** 构建具体页面时，先查 `design-system/toolbox/pages/[page].md`。存在则该页规则**覆盖**本文件；不存在则严格遵循本文件。
>
> 本文件基于 ui-ux-pro-max 生成结果**人工校正**：保留其有效判据（Dark audio + warm accent、Fira Code/Fira Sans 技术精确调性、间距与反模式清单），替换掉不适用于后台工具的落地页 Pattern/Style，并补齐中文与离线约束。

**Project:** toolbox（AI 语音工具箱 · 单用户本地工具）
**Category:** Admin Console / Developer Tool
**Updated:** 2026-09-12

---

## 0. 设计方向（一句话）

**把界面做成一台专业音频设备**：近黑阳极面板、机架刻度线、琥珀色信号色（VU 表头）、等宽数字读数、刻印感微标签。产品调性是「精密仪器」，不是「炫酷 AI 产品」。

设计决策的判据：任何视觉选择都要能回答「这像不像一件录音棚里的设备」。不像的，删掉。

---

## 1. 色彩

### 1.1 暗色（默认）

| 角色 | 值 | 语义 / 用途 |
|---|---|---|
| `--color-bg` | `#0A0A0C` | 应用画布（近黑中性，**禁止纯黑 #000**） |
| `--color-panel` | `#0F0F12` | 侧栏、面板底、表头 |
| `--color-raise` | `#141418` | 卡片、内容容器 |
| `--color-raise-2` | `#1B1B20` | 卡片内嵌槽、输入框、hover 底 |
| `--color-line` | `rgba(255,255,255,.07)` | 发丝分隔线（骨架级别，几乎每处都用） |
| `--color-line-strong` | `rgba(255,255,255,.13)` | 分组边界、输入框边框 |
| `--color-fg` | `#EDEDEF` | 主文本 |
| `--color-fg-2` | `#A6A6AE` | 次级文本、值 |
| `--color-muted` | `#6C6C76` | 微标签、说明、占位 |
| `--color-accent` | `#FF8A3D` | **信号琥珀**：主操作、激活态、焦点环 |
| `--color-accent-hi` | `#FFA566` | accent hover |
| `--color-accent-ink` | `#1A0E04` | 落在 accent 底上的文字 |
| `--color-meter` | `#45D483` | **信号绿**：成功、波形、电平（不做主操作色） |
| `--color-warn` | `#F5B942` | 警示 |
| `--color-danger` | `#FF6B6B` | 错误、破坏性操作 |

### 1.2 亮色（完整适配，非降级）

| 角色 | 值 |
|---|---|
| `--color-bg` | `#F6F6F7` |
| `--color-panel` | `#FFFFFF` |
| `--color-raise` | `#FFFFFF` |
| `--color-raise-2` | `#F1F1F3` |
| `--color-line` | `rgba(0,0,0,.10)` |
| `--color-line-strong` | `rgba(0,0,0,.16)` |
| `--color-fg` | `#14141A` |
| `--color-fg-2` | `#4A4A55` |
| `--color-muted` | `#6E6E7A` |
| `--color-accent` | `#D9600F` |
| `--color-accent-hi` | `#B94F0A` |
| `--color-accent-ink` | `#FFFFFF` |
| `--color-meter` | `#15803D` |
| `--color-warn` | `#B45309` |
| `--color-danger` | `#DC2626` |

**色彩纪律**：琥珀是唯一主操作色；绿只表示「信号/成功/电平」；红只表示「错误/破坏」。禁止第二个装饰性强调色。

---

## 2. 字体

**Fira 不覆盖中文**，必须逐字形回退——中英混排是本产品常态，字体栈顺序不可改：

```css
--font-sans: "Fira Sans", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", system-ui, sans-serif;
--font-mono: "Fira Code", ui-monospace, SFMono-Regular, Menlo, monospace;
```

- **Fira Sans**：界面文本（拉丁 + 数字）
- **Fira Code**：**全部数字读数**——时间码、时长、耗时、计数、ID、参数值
- **中文**：PingFang SC / 微软雅黑回退（不做中文字体定制）

**字体交付**：`@fontsource/fira-sans` 与 `@fontsource/fira-code`（npm 自托管、Vite 打包）。**禁止 Google Fonts CDN**——本地工具要离线可用，且 CDN 在境内不稳定。

### 字号与字重标尺

| Token | 值 | 用途 |
|---|---|---|
| `micro` | 11px / `letter-spacing .08em` / uppercase | **刻印微标签**：字段名、区块标题、表头 |
| `xs` | 12px | 辅助、时间戳 |
| `sm` | 13px | 次级正文、按钮 |
| `base` | 14px | 正文 |
| `lg` | 16px | 区块标题 |
| `xl` | 20px | 页面标题 |
| `2xl` | 26px | 面板级大标题 |
| `3xl` | 34px / mono | **统计读数**（工作台） |

字重仅用 400 / 500 / 600。禁止 700+（会破坏精密感）。数字一律 `font-variant-numeric: tabular-nums`。

---

## 3. 空间、圆角、阴影、动效

**间距**（只用这几个，禁止随手写 px）：`4 / 8 / 12 / 16 / 24 / 32 / 48`

**圆角**：`--radius-sm 6px`（输入、按钮）· `md 10px`（卡片）· `lg 14px`（浮层）· `full`（徽标、圆点）

**阴影**（暗色底靠「内发光 + 深投影」造层次，不是靠大黑影）：
```css
--shadow-1: 0 1px 2px rgba(0,0,0,.4), inset 0 1px 0 rgba(255,255,255,.03);
--shadow-2: 0 8px 24px -8px rgba(0,0,0,.6), inset 0 1px 0 rgba(255,255,255,.03);
--shadow-3: 0 24px 48px -12px rgba(0,0,0,.7);
```

**动效**：`--dur-1 120ms`（状态切换）· `--dur-2 200ms`（进浮层）· `--ease cubic-bezier(.2,.8,.2,1)`
- 只做**有意义的动效**：任务进度、播放电平、浮层进离场、页面首屏分段浮现（staggered）。
- **禁止**装饰性无限动画（呼吸/闪烁只用于 running 状态，且尊重 `prefers-reduced-motion`）。
- **禁止** layout-shifting hover（`scale` / `translateY` 位移）。

---

## 4. 图标

**Lucide**（`lucide-react`），统一 `stroke-width 1.75`、尺寸 16 / 20 两档。

- **严禁 emoji 作图标**（当前实现的 🗣🎙🎧🎚🕘⚙ 全部替换）
- 导航图标、操作图标、空状态图标一律来自同一套 stroke 图标
- 图标不单独承担语义时配文字标签（a11y）

---

## 5. 组件规格

组件位于 `web/src/ui/`，**所有页面必须复用，禁止再手写卡片/按钮 class**。

| 组件 | 要点 |
|---|---|
| `Button` | 变体 `primary`(琥珀实底) / `secondary`(描边) / `ghost` / `danger`；尺寸 sm/md；`loading` 态内置 spinner 且禁点；hover 只变底色/描边色，**不位移** |
| `Field` | **刻印微标签**(micro) + 控件 + hint/error 行；label 必须与控件关联（htmlFor） |
| `Input` / `Textarea` / `Select` | 底 `raise-2`、边框 `line-strong`、focus 时边框转 accent + 2px accent 外环（`focus-visible`，不可移除） |
| `Card` | 可选 header（标题 + 右侧操作）；内部 16/24 间距；hover 只提亮边框/底色 |
| `Badge` | 任务状态：pending 灰 / running 琥珀(带呼吸点) / succeeded 绿 / failed 红 / canceled 灰；**不得只靠颜色**（带文字） |
| `ProgressBar` | 细条(2px)，底 `raise-2`、条 accent；running 时条上叠加流光 |
| `Skeleton` | 与最终布局同形状的骨架（**内容区不用 spinner**），`animate-pulse` |
| `EmptyState` | 图标 + 标题 + 一句说明 + 一个主操作（禁止空白页） |
| `Toast` | 右上角，250ms 滑入淡出，4s 自动消失，`role="status"`；**全站替换 `alert()`** |
| `ConfirmDialog` | 破坏性操作（删除任务）必须二次确认，危险按钮在右侧 |
| `Tabs` | 分段控件（segmented）；键盘方向键可切，`role="tablist"` |
| `IconButton` | 方形图标按钮，必有 `title`/`aria-label` |
| `MicroLabel` | 刻印微标签的统一实现 |
| **`WavePlayer`** | **签名组件**：WebAudio 解码取峰值 → canvas 波形（amber 已播 / muted 未播）+ 播放头 + mono 时间码 `mm:ss.d`；用于所有音频产物 |
| `MiniPlayerBar` | 全局底部播放条：当前产物、上/下一个、波形、关闭；跨页面常驻（zustand） |
| `LevelMeter` | 播放中电平条（绿），仅在播放态出现 |

---

## 6. 页面模式（后台控制台，非落地页）

**外壳**：左侧固定导轨 232px（图标+文字，激活态 = 琥珀左侧标记 + 提亮底）· 顶部条（页面标题 + 上下文 + 全局操作：主题切换/健康状态）· 主区 `max-w-[1100px]` 居中，32px 内边距。
**响应式**：`<1024` 导轨收为纯图标（64px），`<768` 导轨变底部/抽屉；主区不出现横向滚动。

**页面结构**：
1. **页头**：标题(20/600) + 一句说明(13 muted) + 右侧主操作
2. **内容**：表单类页面在 `≥1024` 走两栏（编辑区 flex-1 + 参数面板 320px），窄屏单栏
3. **结果**：卡片 + 行式布局；音频行 = 轨道标签 + WavePlayer + 下载/联动操作
4. **任务列表**：表格，mono 时间戳、状态 Badge、行内操作
5. **加载**：Skeleton 同构骨架；**空态**：EmptyState 带主操作；**错误**：行内错误 + Toast，不用 alert

**工作台**：只放真数据（`/api/tasks` 汇总：各工具任务数/成功率/最近任务），**禁止编造统计**。

---

## 7. 反模式（禁止）

沿用并强化 ui-ux-pro-max 清单：

- ❌ **emoji 作图标** —— 一律 SVG（Lucide）
- ❌ **纯黑 #000** —— 用 `#0A0A0C`
- ❌ **位移/缩放 hover** —— 只改颜色与边框
- ❌ **超过 300ms 的过渡**，或瞬时无过渡
- ❌ **`alert()` / `confirm()`** —— 用 Toast / ConfirmDialog
- ❌ **只靠颜色区分状态**（状态必须带文字）
- ❌ **移除默认焦点环**（必须换成本设计的 accent 焦点环）
- ❌ **内容区 spinner**（用 Skeleton）
- ❌ **空白页**（必须有 EmptyState）
- ❌ **`var(--x)` 任意值散写**（用 `@theme` 语义工具类，如 `bg-panel` / `text-fg-2`）
- ❌ 第二个装饰性强调色、700+ 字重、多套圆角混用

---

## 8. 交付前检查表

- [ ] 无 emoji 图标；图标全部来自 Lucide 且尺寸统一
- [ ] 所有可点元素有 `cursor-pointer` 与可见 hover 反馈
- [ ] 过渡 150–300ms；hover 无位移
- [ ] 亮色模式文本对比 ≥ 4.5:1，边框可见，两种主题都实测
- [ ] 键盘可达：焦点环可见、Tab 顺序合理、对话框可 Esc 关闭
- [ ] `prefers-reduced-motion` 下禁用呼吸/滑入动画
- [ ] 375 / 768 / 1024 / 1440 四档响应式，无横向滚动
- [ ] 加载有 Skeleton、空态有引导、错误有 Toast 与行内提示
- [ ] 数字读数使用 mono + tabular-nums
- [ ] 无 `alert()` 残留、无 `var()` 任意值散写

---

## 9. 页面级覆盖

页面特定偏离写在 `design-system/toolbox/pages/<page>.md`（存在即覆盖本文件）。当前无覆盖文件。
