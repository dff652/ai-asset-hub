# beautify-github-readme 适用性评估

- 日期：2026-07-30
- 对象：[oil-oil/beautify-github-readme](https://github.com/oil-oil/beautify-github-readme)
- 结论：**可用；asset-only 与 README mode 已进入默认分支首页，并随 `v0.1.7`
  完成版本证据复验**
- 本机状态：Skill 已安装；已生成项目首屏、五步使用流程、资产生命周期图和真实
  TUI 状态证明板。
- 已执行：在项目所有者明确授权后进入 README mode，重构阅读顺序与首屏并嵌入
  静态 SVG；`v0.1.7` 正式安装包已完成 TUI 证明板对应能力的隔离验收。

## 当前完成度

| 范围 | 状态 | 说明 |
|---|---|---|
| Skill 安装与适用性评估 | ✅ 完成 | 已按仓库真实内容确定终端/资产生命周期视觉方向 |
| asset-only 静态素材 | ✅ 完成 | 五步使用流程、生命周期图和 TUI 状态证明板已生成 |
| README 项目原生首屏 | ✅ 完成 | 新增 terminal-first 静态 hero，不依赖远程字体或脚本 |
| README 文案、阅读顺序与首屏重构 | ✅ 已发布 | 先回答定位与状态，再给安装、流程、TUI 证据、入口和安全边界 |
| 把 SVG 嵌入 README | ✅ 已发布 | 三张 SVG 均有文字替代说明，正文保留命令与限制 |
| 默认分支 GitHub 页面验收 | ✅ 已发布 | 主入口、流程、证明文字和版本边界已进入 `main`；本次发布后分支把版本证据同步到 `v0.1.7` |
| 项目原生视觉门禁 | ✅ 已发布 | 四张 SVG、README/上手指南嵌入和 900/360 人工检查已纳入 SOP 与 `check-local.sh` |

README mode 已随 `v0.1.5` 发布，规范化主入口和静态门禁已随 `v0.1.6` 进入
`main`。本次发布后更新把徽章、证明板、安装文案和已发布能力同步到验收完成的
`v0.1.7`，不改变信息架构。安装命令仍如实保留旧二进制的升级限制。

2026-07-30 规范化通过 PR #20 与 PR #21 进入 `main`：hero 的主命令从兼容入口 `aiah ui` 改为
正式主入口 `aiah`；证明板内部从 dev-candidate 文案更新为 `v0.1.5` 正式安装包
证据；四张 SVG 补齐显式尺寸和统一安全边距，并新增项目原生静态检查脚本及
[README/SVG 验收 SOP](../runbooks/readme-visual-acceptance.md)。`v0.1.7` 正式包
复验后，证明板版本文字和自动门禁同步更新为 `v0.1.7`。

## 为什么适合

该项目的 Skill 强调先读取真实仓库、README 和已有截图，不虚构功能；默认生成
GitHub 可安全渲染的静态 SVG，把可复制命令和正文保留在 Markdown，并要求在本地
预览宽屏/窄屏效果。这与 aiah 的可验证文档原则、安全边界和“不自动提交/推送”
约束一致。

aiah 也有适合可视化的真实产品流程：

```text
发现资产 → 整理资产库 → 检查并准备 → 预览变化 → 人工确认
```

安装检查与撤销作为应用后的安全闭环单独说明，不挤进需要首次用户记忆的五步主线。

## README 需要几张流程图

结论：**一张主流程 SVG 足够，但只列图、不说明其它流程则不够。**

项目实际有六类入口任务：

1. 第一次整理并应用；
2. 日常更新与移出；
3. 安装检查与撤销；
4. 跨设备迁移；
5. AI/MCP 与脚本自动化；
6. aiah 自身安装和升级。

`usage-flow.svg` 只承担第一类“首次成功”路径。其它五类不是同一条时序：维护是
循环，撤销由安装状态触发，迁移跨两台设备和外部传输层，MCP 是只读接口，工具升级
管理的是 aiah 二进制。把它们强行合成一张大图会产生重复节点，并在 360px GitHub
视图中不可读。

采用的分层是：

| 层级 | 表达方式 | 作用 |
|---|---|---|
| README 主流程 | 一张 `usage-flow.svg` | 一眼理解第一次安全应用 |
| README 其它任务 | 可搜索 Markdown 表格 | 告诉用户还有哪些流程、从哪里进入 |
| 上手指南 | `asset-lifecycle.svg` + 正文 | 解释跨设备资产库/包/目标关系 |
| 使用流程总览 | 完整 Markdown | 固化六类入口流程、CLI/TUI/MCP 和写入边界 |
| TUI 证明 | `tui-proof-board.svg` | 展示真实状态与实现证据，不充当第二张流程图 |

这符合“一张主线、一张证明、详细流程留正文”的 README 信息密度。后续只有当某条
次要流程升级为主要入口，且 Markdown 已不能清楚表达时，才新增第二张 README 流程
SVG。

## 采用方式

1. 第一轮采用“asset-only”：新增 `assets/readme/` 下的静态工作流图和真实 TUI
   状态证明板。✅
2. 第二轮在 owner 授权后采用 README mode：新增项目原生 hero、重排正文并嵌入
   三张 SVG；重构前全文保存在
   [`docs/archive/README-before-readme-mode-20260730.md`](../archive/README-before-readme-mode-20260730.md)
   供对比，该快照不是当前说明事实源。✅
3. 发布前证明板明确标注 dev candidate；`v0.1.5` 正式安装包验收后才移除该边界。
   ✅
4. 首屏在一屏内回答“是什么、解决什么、如何开始”，保留安装命令、Technical
   Preview、Linux amd64 支持范围和安全边界为可搜索 Markdown。
5. 视觉主题来自终端/资产生命周期，不使用与产品无关的通用 AI 插画；默认不做
   GIF，避免体积、可访问性和维护成本。
6. 本地检查宽屏/窄屏、链接、SVG 安全和 diff；提交、push 和 main 提升均由项目
   所有者分别授权。✅

本轮产物：

- [`assets/readme/usage-flow.svg`](../../assets/readme/usage-flow.svg)：
  发现资产 → 整理资产库 → 检查并准备 → 预览变化 → 人工确认；各阶段输出和
  “前四步目标目录零写入”的边界在上手指南正文中可搜索；

- [`assets/readme/asset-lifecycle.svg`](../../assets/readme/asset-lifecycle.svg)：
  工具来源 → 版本化资产库 → 本机应用 / 跨设备迁移；README 主流程图替换后，
  该图继续用于上手指南的跨设备章节；
- [`assets/readme/tui-proof-board.svg`](../../assets/readme/tui-proof-board.svg)：
  统一资产状态、typed apply 结果和 E3.1 只读迁移状态；
- [`assets/readme/hero.svg`](../../assets/readme/hero.svg)：终端式项目首屏，强调
  多工具来源、版本化资产库和 read-only-first 生命周期；
- 四张 SVG 都包含 `<title>` / `<desc>`，不含脚本、远程资源或外部字体；已用
  librsvg 分别渲染 900px 和 360px 并人工检查，首屏重叠问题在验收中修正。

## 仍然不做的事

- 不把尚未实现的 E3.2 跨设备写操作或 MCP 统一状态画成已发布能力；
- 不让图片替代安装命令、使用限制或安全说明；
- 不删除重构前快照或把它列为当前操作文档；
- 不引入远程字体、脚本或难以维护的动画；
- 不用 stars、下载量等易过期数字充当核心证明。

项目采用 MIT 许可证；若复制其代码或模板而非仅执行 Skill，需要按仓库许可证和
本项目第三方依赖流程确认归属。仅参考工作方法、由本项目生成自己的 SVG/Markdown，
不构成运行时依赖。
