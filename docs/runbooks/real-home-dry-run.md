# 真机 dry-run 注意事项（不写真实家目录）

本 runbook 说明如何在**真实机器**上安全地预演 AI Asset Hub 部署计划。
默认目标：**只读扫描 + 只读 diff**；**不**对真实 `$HOME` 执行 `apply`。

相关：假 HOME 闭环见 [fake-home-loop.md](fake-home-loop.md)。

## 原则

1. **先假后真**：任何包先在 `mktemp` 假 HOME 上 `apply → scan → rollback` 通过。
2. **真机只 dry-run**：`aiah diff` 或 `aiah apply --dry-run`，确认 `dryRun: true` 且磁盘无变化。
3. **明确路径**：真机预演时显式传 `--home` / `--project`，避免误用错误目录。
4. **密钥与 MCP**：包内只允许 `${ENV:…}` / `${secret:…}`；2C.3.1/2C.3.2 已通过
   严格复审，含 MCP asset 的包不再限制为只做 dry-run，但仍按本 runbook 走完
   假环境闭环与人工 diff 确认。运行前先 export 引用的环境变量，或确认
   `${secret:path}` 能由 `pass show -- path` 读到非空首行。
5. **Hooks**：脚本 hook 安装后为 `0755` 且需 shebang；工具侧仍可能要求 trust / 注册，**aiah 不自动执行 hook**。
6. **备份意识**：真 apply 才会写 `.aiah/backups/<id>/`；dry-run 不产生备份。

## 推荐流程

### 0. 构建 CLI

```bash
cd /path/to/ai-asset-hub
go build -o build/aiah ./cmd/aiah
```

### 1. 只读扫描真实环境（可选）

```bash
# 默认扫描真实 $HOME；也可指定副本
./build/aiah scan --home "$HOME" --output json > /tmp/aiah-scan-home.json

# 有项目规则时
./build/aiah scan --home "$HOME" --project /path/to/project --output json > /tmp/aiah-scan-full.json
```

检查报告：

- `source` / `type` 是否合理
- 是否出现不想迁移的 sessions / auth / 密钥形内容 finding
- 现有 hooks / MCP / skills 布局

### 2. 校验并构建资产包（工作区，非 $HOME）

```bash
./build/aiah validate --manifest path/to/workspace/manifest.yaml --output json
./build/aiah build \
  --manifest path/to/workspace/manifest.yaml \
  --profile personal \
  --out /tmp/aiah-dist \
  --output json
PKG=/tmp/aiah-dist/<name>-<version>.tar
```

### 3. 假 HOME 全写闭环（强制）

```bash
./scripts/demo-apply-scan-loop.sh workspace-2b
# 或对你的真实包：
WORKDIR=$(mktemp -d)
./build/aiah apply --package "$PKG" --home "$WORKDIR/h" --project "$WORKDIR/p" --output json
./build/aiah scan --home "$WORKDIR/h" --project "$WORKDIR/p" --output json | head
# 确认 hook 可执行位、MCP native 计划、project CLAUDE.md 等
./build/aiah rollback --home "$WORKDIR/h" --project "$WORKDIR/p" --output json
rm -rf "$WORKDIR"
```

### 4. 真机 dry-run（不写盘）

```bash
# 方式 A：diff（始终 dry-run）
./build/aiah diff \
  --package "$PKG" \
  --home "$HOME" \
  --project /path/to/project \
  --targets claude,codex,grok \
  --output json > /tmp/aiah-diff-real.json

# 方式 B：apply --dry-run（等价）
./build/aiah apply --dry-run \
  --package "$PKG" \
  --home "$HOME" \
  --project /path/to/project \
  --targets claude,codex,grok \
  --output json > /tmp/aiah-dry-real.json
```

验收 dry-run：

| 检查 | 期望 |
|---|---|
| 报告 `dryRun` | `true` |
| 报告 `ok` | 若 `false`，先修 finding，禁止 apply |
| `$HOME/.aiah` | **不应**因本步新建（对比 dry-run 前后 `ls`） |
| `changes` | 人审 create/update 路径；关注 `.claude.json` / `.mcp.json` / `config.toml` / hooks |
| findings | 无 `error`；检查 `mcp_native_created` / `unchanged` / `skipped` 与 `hook_policy` |

快速确认未写盘：

```bash
# dry-run 前后对比（示例）
find "$HOME/.claude" "$HOME/.claude.json" "$HOME/.codex" "$HOME/.grok" "$HOME/.aiah" -type f 2>/dev/null | sort > /tmp/before.txt
./build/aiah apply --dry-run --package "$PKG" --home "$HOME" --output json >/tmp/dry.json
find "$HOME/.claude" "$HOME/.claude.json" "$HOME/.codex" "$HOME/.grok" "$HOME/.aiah" -type f 2>/dev/null | sort > /tmp/after.txt
diff -u /tmp/before.txt /tmp/after.txt   # 应无输出
```

### 5. 真机 apply（显式人工确认）

**默认本 runbook 不执行此步。** 2C.3.1/2C.3.2 已通过严格复审，含 MCP asset 的包
不再被禁止，但门槛不变：假环境闭环通过、dry-run 无 error、逐项检查 diff 且用户
明确决定执行后，才按以下流程操作。

含 MCP asset 时额外注意复审 P6：若 `~/.claude.json` 是软链（dotfiles 管理）、
是 0 字节/非法 JSON，或已有 server 写成 `"args": []`，当前会让**整单 apply 失败**
（不改任何文件）。先 dry-run 看有没有 `mcp_native_failed` 再决定。

```bash
# 再次 dry-run 确认
./build/aiah apply --dry-run --package "$PKG" --home "$HOME" --project "$PROJ" --output json | tee /tmp/final-dry.json

# 真写（会产生 backupId）
./build/aiah apply --package "$PKG" --home "$HOME" --project "$PROJ" --output json | tee /tmp/apply.json
# 记录 backupId；出问题：
# ./build/aiah rollback --home "$HOME" --project "$PROJ" --backup <id> --output json
```

真 apply 后建议：

1. `aiah doctor --home "$HOME" --project "$PROJ" --output json` 检查 journal /
   stage / backup 与部署漂移
2. `aiah scan` 对照 inventory
3. 在对应工具内 **手动 trust / 确认 hooks**（Claude/Codex 可能要求审核）
4. 确认 MCP 引用的环境变量在 shell 中已 export
5. **不要**在 apply 日志/截图中粘贴真实 token

## 高风险路径清单

以下路径 dry-run 出现 `update` 时务必逐条阅读内容 diff（当前 CLI 以路径级 change 为主，可结合备份与人工对比）：

- Claude user `~/.claude.json`、project `.mcp.json`
- Codex user/project `.codex/config.toml`
- Grok user/project `.grok/config.toml`
- 已有同名 skill / agent / hook 文件（可能覆盖）
- 项目根 `CLAUDE.md` / `AGENTS.md`

`device` scope 资产永远不会 apply。

## Hooks 真机注意

- 包内 `*.sh` 必须有 `#!` shebang，否则 apply fail-closed（`hook_invalid`）。
- 安装模式为 `0755`；JSON 类 hook 为 `0644`。
- aiah **只落盘文件**，不注册、不 trust、不执行。
- 事件名（PreToolUse 等）因 harness 而异；见 ADR-0002 `hook.lifecycle`。

## MCP 真机注意

- 2C.3.1/2C.3.2 已通过严格复审（2026-07-25），含 MCP asset 的包可在真机 apply；
  仍先假环境闭环 + dry-run + 人工 diff。
- 模板 sidecar 仍写在 `*/mcp/*.json`，其中只保留引用。
- `diff` / apply 计划阶段解析 MCP `env` 引用；环境变量、`pass` entry 缺失或为空
  时返回 `mcp_native_failed`，整单不写。
- 一次性创建的 native config 含设备本地解析值；报告、journal 与 backup 元数据
  不含该值。后续 scan 将 native config 报为 `suspected_secret` 并排除，防止它
  被重新打包。
- create-only：native config 不存在时执行一次性 bootstrap；创建后也永不自动
  更新。已存在且 identical 时零写入；缺 server 时返回 `mcp_native_skipped`，
  只安装 sidecar。
- 已有同名 server 与包内容冲突时返回 `mcp_native_failed`，整个 apply
  fail-closed，其它 sidecar/skill/hook 也不会落盘。
- 若未来显式更新已有配置，精确 backup 可能包含原文件已有敏感值；backup 必须保持
  私有且不得提交或同步到不可信介质。

## 不要做的事

- 不要对真实 `$HOME` 先 `apply` 再看报告。
- 不要用含真实密钥的包做任何环境（含假 HOME）演示。
- 不要把 `.aiah/backups` 提交到 Git 或同步到不可信云盘。
- 不要假设 dry-run 失败后“忽略 findings 强行 apply”。

## 故障时

| 现象 | 处理 |
|---|---|
| dry-run `ok: false` | 读 `findings`；修包或改 targets/scope |
| 假 HOME 通过、真机 diff 大量 update | 预期：真机已有配置；逐项决定是否覆盖 |
| 需要撤销真 apply | `aiah rollback --backup <id>`；确认 journal 已清理 |
| hook 工具不触发 | 检查 trust、配置注册、可执行位与 shebang |

## 与自动化测试的关系

- CI / `go test ./internal/e2e` 与 `scripts/demo-apply-scan-loop.sh` **只使用临时目录**。
- 本 runbook 的真机步骤是人工流程，不进 CI。
