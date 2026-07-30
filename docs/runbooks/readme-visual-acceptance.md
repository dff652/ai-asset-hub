# README 与 SVG 视觉验收 SOP

- 适用：修改仓库首页、`assets/readme/*.svg`、图内命令、能力状态或 README 图片嵌入。
- 不适用：产品功能验收。视觉图只能引用已有代码、测试和正式安装包证据，不能替代它们。
- 自动门禁：`python3 scripts/check-readme-assets.py`，并已纳入
  `./scripts/check-local.sh`。

## 1. 固定项目视觉系统

先写清项目故事，避免把通用“AI 科技感”套进仓库：

```text
受众：同时使用 Claude、Codex、Grok 的个人和小团队
一句话价值：把分散的 AI 编程资产整理成可迁移、可审阅、可恢复的资产库
主要证明：真实 TUI 状态、五步写入边界、跨工具与跨设备生命周期
第一次成功：安装后运行 aiah，完成发现 → 整理 → 预览 → 人工应用
视觉主题：终端节奏 + 资产库结构 + 克制的状态色
```

### 颜色

| 角色 | 颜色 | 使用边界 |
|---|---|---|
| 主背景 | `#0D1117` / `#010409` | 提供完整背景，保证 GitHub 明暗主题均可读 |
| 前景 | `#E6EDF3` | 标题和主要内容 |
| 次要信息 | `#8B949E` | 说明、边界和非关键 metadata |
| 只读/流程 | `#58A6FF` | 发现、预览、迁移状态和连接线 |
| 成功/应用 | `#3FB950` | 已纳管、apply 成功和健康状态 |
| 人工门槛 | `#D29922` | typed confirmation；不能装饰性滥用 |
| 风险 | `#F85149` / `#FF7B72` | 删除、冲突或终端状态点 |

紫色只用于 Grok/doctor 等少量类别区分，不增加渐变彩虹或通用发光效果。

### 字体、尺寸与布局

- 正文使用系统 sans-serif：`-apple-system` / `BlinkMacSystemFont` /
  `Segoe UI` / `PingFang SC` / 通用 `sans-serif`；
- 命令、路径和 metadata 使用系统 monospace：`ui-monospace` /
  `SFMono-Regular` / `Menlo` / `Consolas` / 通用 `monospace`；
- 不加载 Inter、远程字体或把中文渲染依赖绑定到单一操作系统；
- full-width SVG 固定 `1200` 宽度，显式声明与 `viewBox` 一致的
  `width` / `height`；
- 重要内容距边缘至少 `48`，hero 可使用更宽的 `84` 安全区；
- hero 标题至少 `48`，图内主要文字至少 `20`，说明至少 `18`；
  `16–17` 只允许用于非必要 badge/metadata，自动门禁拒绝更小字号；
- 外层圆角统一为 `24–28`，普通卡片使用 `16–22`，不为每个元素增加阴影和边框。

## 2. 四张图各自只承担一个职责

| 文件 | 职责 | 必须保持的事实 |
|---|---|---|
| `hero.svg` | 定位、事实源和第一启动入口 | 主命令是 `aiah`；`aiah ui` 只在正文说明兼容性 |
| `usage-flow.svg` | 第一次安全应用 | 前四步目标目录零写入；最后一步 typed `apply` |
| `tui-proof-board.svg` | 已验收的真实 TUI 状态 | 标明证据对应版本；不能把 dev 候选写成正式包或反过来 |
| `asset-lifecycle.svg` | 跨工具和跨设备关系 | 资产库是事实源；分发不是后台双向同步 |

README 只把 `usage-flow.svg` 当主流程图。其它流程使用任务表和
[使用流程总览](../usage-flows.md)，不继续向首屏堆叠流程图。

## 3. 语义核对

修改坐标前先从实现和测试反查图中文字：

1. 启动命令核对 `cmd/aiah/main.go` 和无参数交互测试；
2. 更新命令核对 `internal/update` 的精确字符串测试；
3. TUI 能力核对正式安装包 dogfood，而不只看本地开发构建；
4. Release、平台和版本核对 tag、线上产物与验收记录；
5. README 正文、alt、`<title>` 和 `<desc>` 使用同一事实口径。

普通用户入口统一写 `aiah`。`aiah ui` 只用于兼容说明，或
`aiah ui --package/--workspace/--home` 等高级直达参数。

## 4. 自动检查

```bash
python3 scripts/check-readme-assets.py
./scripts/check-local.sh
git diff --check
```

项目检查器会验证：

- 四张 SVG 存在、尺寸与 `viewBox` 一致且小于 100 KiB；
- `<title>`、`<desc>`、`role="img"`、`aria-labelledby` 齐全；
- 没有脚本、`foreignObject`、远程资源或非系统字体；
- 所有文字都有可追踪字体和字号，最小字号不低于 `16`；
- README/上手指南的本地图片存在且 alt 非空；
- hero 不回退到 `aiah ui`，证明板不回退到 dev-candidate 文案。

若本机安装了 `beautify-github-readme` Skill，再运行其 README audit 作为补充；
项目门禁不能依赖用户目录中的 Skill 才能通过。

## 5. 900px / 360px 人工检查

自动检查不能判断文字重叠、视觉层级和真实可读性。每次改图还要把四张 SVG 分别渲染
到 `900px` 和 `360px` 宽度：

- `900px`：标题、命令、卡片正文不能裁切或重叠；对齐线和安全边距一致；
- `360px`：项目名和主结构仍能辨认；缩小后不可读的非必要细节必须在相邻 Markdown
  或 alt 中完整表达；
- 深色 SVG 自带背景，在 GitHub 明暗主题周围都保持边界；
- 不用“缩小所有文字”解决拥挤，优先删减、缩短、拆分或移动细节到正文。

最终人工核对 hero、五步流程、TUI 证明板和资产生命周期，不以 XML 能解析代替视觉
验收。

## 6. 状态变更协议

- 功能从候选变为发布版：先完成正式安装包 dogfood，再更新证明板版本与 README；
- 启动入口变化：同时更新 hero、README、上手指南、CLI 参考、TUI 内提示和测试；
- 视觉 token 变化：四张 SVG 一起检查，不能只让其中一张使用新字体或新边距；
- 重构前快照保留在 `docs/archive/`，但不把它列为当前操作事实源；
- 未经所有者授权，不因视觉验收自动 commit、push、开 PR 或发布。
