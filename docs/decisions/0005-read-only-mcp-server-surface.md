# ADR-0005：MCP server 只读边界

- 状态：Accepted
- 实施：`internal/mcp` + `aiah mcp`；2026-07-28 首版 5 工具落地，2026-07-30
  N6 扩展为 7 个只读工具，2026-08-02 N10.3 扩展为 8 个只读工具
- 日期：2026-07-28（N10.3 修订 2026-08-02）
- 关联：[ADR-0003](0003-cli-first-go-core-and-product-surfaces.md)（CLI 是 Agent
  接口）、[ADR-0004](0004-native-mcp-config-ownership.md)（**不同主题**：那份讲
  MCP 模板作为**资产**如何写入原生配置，本份讲 aiah 自己**作为 MCP server**）、
  [产品形态与分发边界评估 §5](../research/product-form-and-distribution-assessment.md)

## 背景

ADR-0003 §1 把 CLI 定为「自动化接口和 **Agent 接口**」，并要求所有命令输出确定性
JSON。该承诺一直没有对应的实现：AI 工具要用 aiah，只能自己拼命令行、自己解析
stdout。

同时存在一个具体且现实的风险。`aiah apply` 写用户 HOME，而 Claude Code 自身从同
一个 HOME 读取配置。若把 apply 暴露给 agent，等于允许它在会话进行中改写自己的运行
时配置：harness 可能中途重载，结果不可预测；一个坏 prompt 就足以损坏 dotfiles。

因此问题不是「要不要做 MCP server」，而是「**边界划在哪，以及如何让边界不可被悄悄
放宽**」。

## 决策

### 1. 提供 `aiah mcp`，只暴露只读子集

当前注册 8 个工具：

| 工具 | 对应命令 | 写盘 |
|---|---|---|
| `aiah_scan` | `scan` | 无 |
| `aiah_asset_status` | `inventory.Scan` + `workspace.Catalog` | 无 |
| `aiah_validate` | `validate` | 无 |
| `aiah_diff` | `diff` | 无 |
| `aiah_doctor` | `doctor` | 无 |
| `aiah_migration_status` | `migration.Inspect` | 无 |
| `aiah_migration_readiness` | `readiness.Inspect` / `aiah readiness` | 无 |
| `aiah_version` | `version` | 无 |

不变式：**经此 server 可达的任何工具都不写任何文件。**

状态与准备检查工具要求调用方明确给出资产库路径：

- `aiah_asset_status` 返回“未纳管 / 已纳管 / 源端有更新 / 仅库内 / 阻止”统一状态，
  MCP 层不重新实现分类；
- `aiah_migration_status` 比较资产库、当前受管安装和可选普通目录通道，不构建、
  发布、取回或应用，也不把“版本不同”解释为某一方较新；
- `aiah_migration_readiness` 汇总打包条件、迁移前置与可选证据文件（仅
  `<workspace>/.aiah/evidence/`），不创建证据、不构建、不发布、不取回、不应用、
  不回滚，也不访问网络。

### 2. `apply` 与 `rollback` 永不进入该 surface

理由见背景段。这不是"暂时不做"，是边界：需要写操作的场景走 CLI，由人执行。

`aiah_diff` 调用 `apply.Diff` 而**不是** `apply.Apply{DryRun: true}`——只读入口不应
该和写入口只差一个布尔字段。

### 3. `build` 也不暴露（对早期评估的收紧）

[形态评估 §5.4](../research/product-form-and-distribution-assessment.md) 原本把
`build` 列为可暴露，理由是「只写 `--out`，不碰 HOME」。落地时收紧：它是清单里唯一
会写盘的命令，且写入目标**由调用方指定**，agent 完全可以把 `out` 指向 `~/.claude`。

排除它换来的是一个绝对而非条件的不变式——「零写入」比「只写你指定的地方」既好测试，
也好向用户承诺。若日后确有需要，再加并配 out 路径护栏，属于边界变更，须修订本 ADR。

### 4. 边界锚在行为上，不锚在名单上

一张"允许的工具名"清单挡不住「有人加了写工具，顺手也改了名单」。因此主防线是
`TestToolCallsWriteNothing`：对**注册表里的每个工具**执行后比较其可达的 HOME、
project、资产库、通道和包目录，所有树必须逐字节（含 mode）一致。

变异验证已证明：即使同时把写工具加进注册表**并**更新期望名单，该测试仍然变红。
新增工具若没有写入安全覆盖，测试同样直接失败。

### 5. 传输与协议

- stdio 上的换行分隔 JSON-RPC 2.0（MCP stdio framing）；
- 实现 `initialize` / `ping` / `tools/list` / `tools/call`，无 id 的请求按
  JSON-RPC 视为通知、不回包；
- `initialize` 的 `instructions` 字段**在带内声明只读边界**，让会把它转给模型的
  客户端也能说明这台 server 做不到什么；
- `tools/list` 为每个工具统一声明 `readOnlyHint=true`、
  `destructiveHint=false`、`idempotentHint=true`、`openWorldHint=false`；
- 工具参数用 `DisallowUnknownFields` 解码：拼错的参数必须显式失败，静默忽略会在
  用户不知情的情况下改变工具读取的路径；
- `inputSchema.additionalProperties=false` 与实际解码行为保持一致；
- 未知工具返回 `isError` 的工具级失败而非 JSON-RPC 错误，让模型能看见并改用别的
  工具；协议层错误保留给协议层问题；
- 单行请求上限 1 MiB。超限后流位置不再可信，因此**停止**而不是尝试重新同步。

### 6. 不引入 MCP SDK

协议面就是 stdin/stdout 上的 JSON-RPC，现有命令已产确定性 JSON，手写实现约 300 行。
Bubble Tea 一族刚为 TUI 付出 `+0.97 MiB` 体积与一轮 `NOTICE` / 第三方清单同步成本；
为一层薄协议再引一族依赖不划算。协议稳定后可重新评估换库。

### 7. `aiah mcp` 不接受任何 flag 或 operand

调用方可访问的路径全部是**工具参数**，不存在 server 级开关。加一个
`--allow-write` 之类的旗标就等于把本 ADR 变成默认值而非边界，因此该子命令连
operand 都拒绝。

## 讨论过但不采用的方案

### 暴露 apply，靠"确认"参数保护

MCP 的确认发生在 agent 与用户之间，不在协议里；server 无法验证确认真的发生过。
把安全性寄托在调用方自律，与本项目其它 fail-closed 契约不一致。

### 暴露 `apply --dry-run` 而不是 `diff`

功能等价，但入口离写路径只有一个布尔字段的距离。用独立函数更难被误改。

### 用 SDK 换取协议完备性

首版只需要 tools 能力，resources / prompts / sampling 都不需要。SDK 带来的完备性
当前用不上，成本却是长期的。

## 影响

正面：

- ADR-0003 §1 的 Agent 接口承诺兑现；
- AI 工具可以回答「这台机器有哪些资产」「这个包会改什么」「我的部署健康吗」；
- AI 工具可以进一步回答「哪些源端资产尚未纳管或已经变化」「资产库、当前安装与
  分发通道是否同版本」；
- 零依赖增量，二进制体积基本不变；
- 只读不变式有行为级测试兜底，不依赖 review 自律。

代价：

- MCP 协议演进需要手工跟进；
- 需要写包时用户仍须切回 CLI（这是**目的**，不是缺陷）；
- 工具集扩张必须修订本 ADR，节奏上比"加个函数"慢。
