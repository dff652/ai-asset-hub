# beautify-github-readme 适用性评估

- 日期：2026-07-30
- 对象：[oil-oil/beautify-github-readme](https://github.com/oil-oil/beautify-github-readme)
- 结论：**可用；asset-only 与 README mode 均已进入 dev 候选，默认分支首页待合入**
- 本机状态：Skill 已安装；已生成项目首屏、五步使用流程、资产生命周期图和真实
  TUI 状态证明板。
- 已执行：在项目所有者明确授权后进入 README mode，重构阅读顺序与首屏并嵌入
  静态 SVG；开发态证明继续与公开 Release 验收分开标注。

## 当前完成度

| 范围 | 状态 | 说明 |
|---|---|---|
| Skill 安装与适用性评估 | ✅ 完成 | 已按仓库真实内容确定终端/资产生命周期视觉方向 |
| asset-only 静态素材 | ✅ 完成 | 五步使用流程、生命周期图和 TUI 状态证明板已生成 |
| README 项目原生首屏 | ✅ 完成 | 新增 terminal-first 静态 hero，不依赖远程字体或脚本 |
| README 文案、阅读顺序与首屏重构 | ✅ 本地完成 | 先回答定位与状态，再给安装、流程、TUI 证据、入口和安全边界 |
| 把 SVG 嵌入 README | ✅ 本地完成 | 三张 SVG 均有文字替代说明，正文保留命令与限制 |
| 默认分支 GitHub 页面验收 | ⬜ 未执行 | dev 候选不改变 main 首页；需随 Release 候选合入 main 后检查 |

README mode 已在本地完成，但不把 dev 候选写成已发布能力：安装与启动命令按
`v0.1.4` 表达，任务首页、统一状态和迁移状态证明板明确标注为 dev candidate。
正式 Release dogfood 后需要更新这条边界，而不是仅删除提示文字。

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

## 采用方式

1. 第一轮采用“asset-only”：新增 `assets/readme/` 下的静态工作流图和真实 TUI
   状态证明板。✅
2. 第二轮在 owner 授权后采用 README mode：新增项目原生 hero、重排正文并嵌入
   三张 SVG；重构前全文保存在
   [`docs/archive/README-before-readme-mode-20260730.md`](../archive/README-before-readme-mode-20260730.md)
   供对比，该快照不是当前说明事实源。✅
3. 证明板明确标注“dev candidate / 非正式 Release 安装验收”，正文把公开版和
   开发候选分开说明。✅
4. 首屏在一屏内回答“是什么、解决什么、如何开始”，保留安装命令、Technical
   Preview、Linux amd64 支持范围和安全边界为可搜索 Markdown。
5. 视觉主题来自终端/资产生命周期，不使用与产品无关的通用 AI 插画；默认不做
   GIF，避免体积、可访问性和维护成本。
6. 本地检查宽屏/窄屏、链接、SVG 安全和 diff；仍需项目所有者明确授权后才
   commit/push。

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

- 不把 dev 候选的“更新/移出”画成已完成正式 Release 验收的能力；
- 不让图片替代安装命令、使用限制或安全说明；
- 不删除重构前快照或把它列为当前操作文档；
- 不引入远程字体、脚本或难以维护的动画；
- 不用 stars、下载量等易过期数字充当核心证明。

项目采用 MIT 许可证；若复制其代码或模板而非仅执行 Skill，需要按仓库许可证和
本项目第三方依赖流程确认归属。仅参考工作方法、由本项目生成自己的 SVG/Markdown，
不构成运行时依赖。
