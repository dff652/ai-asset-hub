# ADR-0008：bootstrap 只编排取回与强制交互审阅

- 状态：Accepted / Implemented
- 日期：2026-07-28
- 关联：[ADR-0007](0007-immutable-channel-distribution.md)（不可变通道）、
  [ADR-0006](0006-tui-as-first-interactive-surface.md)（TUI 写操作边界）

## 背景

ADR-0007 把 `pull` 与 `apply` 分成两步，中间必须有人看 diff；roadmap 同时承诺
提供新设备 `aiah bootstrap`。如果 bootstrap 只是把两个命令无条件串起来，就会
删掉这道人工确认，而不是提供便利入口。

现有 TUI Phase C 已经具备所需安全交互：直接调用 `apply.Diff` / `apply.Apply`、
按 action 分组审阅、必须完整输入 `apply`、成功展示 `backupId` 与回滚命令、失败
原样展示 Core findings。bootstrap 不需要再实现第二套部署界面。

## 决策

`aiah bootstrap` 是一个**交互式编排命令**：

```text
TTY 预检 → channel.Pull → TUI Phase C 的 apply.Diff
         → 人审 → 完整输入 apply → apply.Apply
```

1. 命令要求显式 `--channel`、`--name`、`--out`，以及至少一个
   `--home` / `--project`；`--version`、`--profile`、`--targets` 与原命令语义一致。
2. **在 pull 前检查 stdin/stdout 是真实 TTY。** 非交互环境直接失败且零写入，提示
   改用独立的 `pull`、`diff`、`apply`。
3. pull 后直接复用 TUI Phase C；不复制 diff、apply、路径安全、备份或确认逻辑。
4. 不提供 `--yes`、`--force`、环境变量旁路或非交互模式。单独按 `a` 不写，只有
   完整输入 `apply` 并按 Enter 才能执行。
5. 用户取消、diff 失败或 apply 失败时退出非零；已验证的拉取产物保留在显式
   `--out` 中，便于修复后重跑，不伪装成未发生 pull。
6. TUI 退出后在普通终端持久打印 apply 结果。真实写入必须显示 `backupId` 和包含
   home/project 的完整 rollback 命令；no-op 明示无需回滚；失败 findings 保持
   Core 原文。

## 不采用

### `bootstrap --yes`

会把安全边界退化成调用者传一个布尔值，CI、脚本或 agent 都能绕过人审，不采用。

### 自己再做一套文本 diff / prompt

会与已验证的 TUI Phase C 长出两套确认和展示逻辑，后续修复很容易只落一边。

### 非 TTY 时只 pull 再报错

命令会在失败前产生副作用，自动化调用者无法从退出码判断磁盘是否变化。TTY 必须在
pull 前验证。

## 影响

- 新设备恢复有单命令入口，但 ADR-0007 的人审边界没有降低；
- bootstrap 依赖终端交互，自动化仍须显式使用三条独立命令；
- 拉取产物是审计材料，不因取消 apply 自动删除；
- 新增行为由 9 项变异验证锚定；typed confirmation 继续复用 TUI Phase C 已有
  10 项变异验证。
