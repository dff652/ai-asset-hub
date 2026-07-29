# 上手指南

这是完整的用户教程。日常本地使用优先走一条 TUI 主线；CLI 留给自动化、假 HOME
演练和跨设备分发。本指南从只读盘点走到可审计、可回滚的首次部署。命令参数总表见
[命令参考](cli-reference.md)；真实 HOME 的逐项检查见
[真机 dry-run runbook](runbooks/real-home-dry-run.md)。

## 安装

当前安装和 Release 支持范围为 **Linux amd64**。推荐先下载并阅读安装器：

```bash
curl -fsSLo /tmp/aiah-install.sh \
  https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh
less /tmp/aiah-install.sh
sh /tmp/aiah-install.sh
```

安装器默认固定 `0.1.3`，也可显式设置：

```bash
AIAH_VERSION=0.1.3 AIAH_INSTALL_DIR="$HOME/.local/bin" \
  sh /tmp/aiah-install.sh
```

从 [Release](https://github.com/dff652/ai-asset-hub/releases/tag/v0.1.3)
手动下载时，必须同时下载 `SHA256SUMS`。Linux amd64 示例：

```bash
sha256sum -c SHA256SUMS --ignore-missing
chmod +x aiah_0.1.3_linux_amd64
mkdir -p "$HOME/.local/bin"
install -m 0755 aiah_0.1.3_linux_amd64 "$HOME/.local/bin/aiah"
aiah version
```

安装器不会修改 PATH；命令找不到时把 `~/.local/bin` 加入 PATH。

### 升级

重复执行一行安装命令即可：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh | sh
```

脚本不是不受控的“永远取 latest”：仓库中的默认版本只在对应 Release 产物和
`SHA256SUMS` 验证完成后更新。本机版本较旧时，安装器校验下载内容并原子替换
`~/.local/bin/aiah`；已经是同版本时零下载、零写入。固定版本、降级或重装指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh |
  AIAH_VERSION=0.1.3 sh
```

aiah 没有后台自动更新器，升级始终是显式操作。升级后用 `aiah version` 核对版本。

`v0.1.1` 中现存的 macOS、Windows 和 arm64 文件是发布范围收口前生成的历史
交叉编译产物，未做对应平台原生验收。它们不是安装器或当前支持范围的一部分；
后续 Release 只发布 Linux amd64，其他平台通过原生验收后再恢复。

## 1. 三个目录

| 目录 | 角色 | 谁写 |
|---|---|---|
| **工作区** `~/ai-assets/` | 唯一事实源，纯文本 + `manifest.yaml`，建议进入 Git | 人或显式启用写能力的 TUI |
| **包** `*.tar` | 不可变产物，带 manifest、lock 与 SHA256 | `aiah build` |
| **目标** `~/.claude` / `~/.codex` / `~/.grok` | 工具实际加载的位置 | `aiah apply` |

目标目录是产物，不是源头。建立工作区后，应修改工作区再重新 build/apply，而不是
继续直接编辑目标目录。

## 2. 推荐：一个 TUI 完成本地流程

```bash
aiah ui --home "$HOME"
```

启动后按以下顺序操作：

1. 用方向键浏览，`/` 过滤，`f` 只看 findings；此时界面只读。
2. 按 `w`，明确输入要打开或创建的工作区，例如 `~/ai-assets`，回车确认。
3. 用空格勾选候选资产，按 `w` 复制到工作区并登记进 `manifest.yaml`。
4. 按 `b`，确认或输入 profile；TUI 自动校验并构建到 `<工作区>/dist/`。
5. 构建成功后自动进入只读 diff。按 `a` 只打开确认页；完整输入 `apply` 并回车才写
   目标目录。
6. 成功后按 `h` 运行只读 doctor，审阅当前 deployment、backup、drift 和 findings。
7. doctor 通过且存在当前部署时，可按 `x`，完整输入 `rollback` 后回滚当前部署。

没有确认工作区前，TUI 不显示复选框，也不会创建目录；它没有隐藏的默认工作区。
也可以在启动时显式传入 `--workspace ~/ai-assets`。HOME/project 下的 `.agents`、
`.claude`、`.codex`、`.grok` 及其子目录不能作为工作区。构建、diff 和 apply 都
调用与 CLI 相同的 Core，不另做一套规则。

当前 TUI 覆盖日常单机流程：盘点、组装、校验、构建、diff、apply、doctor，以及
当前部署的 rollback。以下场景仍使用 CLI：

- 安装、升级、版本固定；
- publish / pull / versions / bootstrap 和跨设备传输；
- 选择并回滚某个历史 backup；
- JSON 自动化、CI、假 HOME 批量演练和 MCP server；
- secret provider 环境准备，以及直接编辑资产正文或 manifest 的高级字段。

TUI 是工作流操作台，不是设置面板；可审计的 `manifest.yaml` 仍是配置事实源。
非 TTY 环境应使用 JSON 命令。

盘点结果中的 `candidate` 只是迁移候选，不代表应原样打包。凭据、session、cache、
数据库和疑似 secret 会被排除或脱敏报告。

## 3. CLI：只读盘点与建立工作区

需要脚本化时：

```bash
aiah scan --home "$HOME" --output json > /tmp/aiah-scan.json
aiah ui --home "$HOME" --workspace ~/ai-assets
```

`scan` 不写 HOME/project、不执行 hook，也不跟随逃逸软链接。TUI 把选中资产复制进
工作区并登记到 `manifest.yaml`；已有文件 create-only，不覆盖。校验失败会回滚
本次创建的文件和目录。

工作区布局与 manifest 字段见[资产模型](asset-model.md)。可以手工整理，也可以显式
让 TUI 组装。

每条资产的四个属性决定后续行为：

- `targets`：部署到哪些工具；
- `scope`：`global` 写 HOME、`project` 写项目根、`device` 永不 apply；
- `portability`：原样分发或需要 adapter；
- `sensitivity`：敏感 MCP env 只允许 `${ENV:...}` / `${secret:...}` 引用。

项目自己的 `CLAUDE.md` / `AGENTS.md` 由项目 Git 管理。aiah 会盘点 missing 或
shadowing，但不会替项目自动初始化、复制、删除或改名。完整边界见
[资产模型 §4.1](asset-model.md#41-claudemd-与-agentsmd-的处理)。

### “导入”与“导出”分别指什么

- **已有工具目录 → 可编辑工作区**：支持。`scan` / TUI 盘点后，勾选候选并
  compose 到工作区；敏感或设备私有内容会跳过或报告。
- **可编辑工作区 → 不可变包**：支持。`build` 就是导出。
- **不可变包 → Claude/Codex/Grok 目标目录**：支持。通过
  `pull` / `bootstrap` / `diff` / `apply` 导入并部署。
- **不可变包 → 可编辑工作区**：当前不支持。包是部署产物，不是源码恢复格式；
  要继续维护资产，应同步原工作区（例如 Git、NAS 或 U 盘），而不是反向解包重建。

## 4. CLI：校验并构建

```bash
aiah validate --manifest ~/ai-assets/manifest.yaml --output json
aiah build --manifest ~/ai-assets/manifest.yaml --profile personal \
  --out /tmp/aiah-dist --output json
```

`validate` 只读检查 schema、依赖/冲突、路径逃逸、软链接、疑似密钥、二进制和超大
文件。`build` 复用同一套检查，按 profile 选择资产，并原子写出：

```text
<name>-<version>-<profile>.tar
<name>-<version>-<profile>.manifest.json
<name>-<version>-<profile>.lock.json
<name>-<version>-<profile>.sha256
```

相同输入应得到相同 archive 字节与 digest。构建不修改源工作区，也不写工具目录。

## 5. 可选但推荐：先走假 HOME 闭环

不要拿真实 HOME 做第一次全写测试：

```bash
WORK="$(mktemp -d)"
PKG=/tmp/aiah-dist/<name>-<version>-personal.tar

aiah apply --package "$PKG" --home "$WORK/home" \
  --project "$WORK/project" --output json
aiah scan --home "$WORK/home" --project "$WORK/project" --output json
aiah doctor --home "$WORK/home" --project "$WORK/project" --output json
aiah rollback --home "$WORK/home" --project "$WORK/project" --output json
```

也可以对仓库 fixture 运行：

```bash
./scripts/demo-apply-scan-loop.sh
./scripts/demo-apply-scan-loop.sh workspace-2b
```

验收目标是 apply 成功、再次 apply 幂等、doctor 无错误、rollback 后回到原状。

## 6. CLI：真机先 diff，再 apply

```bash
aiah diff --package "$PKG" --home "$HOME" --output json
aiah apply --dry-run --package "$PKG" --home "$HOME" --output json
aiah apply --package "$PKG" --home "$HOME" --output json
aiah doctor --home "$HOME" --output json
```

确认 dry-run 报告 `dryRun=true`、`ok=true` 且没有 error finding。真实 apply 会返回
`backupId`；记录它，出现问题时执行：

```bash
aiah rollback --home "$HOME" --backup <id> --output json
```

也可以在 TUI 审阅部署：

```bash
aiah ui --package "$PKG" --home "$HOME" --targets claude,codex
```

按 `a` 只进入第二次确认；必须完整输入 `apply` 并回车才会写入。成功后界面展示
`backupId` 和完整 rollback 命令。

## 7. 三个常见边界

- Hook 必须带 shebang；aiah 只落盘，不执行也不替目标工具 trust。
- MCP 原生配置是 create-only；已有同名 server 内容冲突会令整单 fail-closed。
- `.aiah/backups` 含设备恢复材料，不应提交 Git 或同步到不可信云盘。

更多排障见[踩坑清单](troubleshooting/ai-asset-pitfalls.md)。

## 8. 跨设备

先 build，再 publish 到普通目录；用 U 盘、Git、rsync 或挂载盘搬运；新设备 pull
后重复假 HOME、diff、apply 流程。aiah 不负责网络传输。

完整命令、不可变规则与故障处理见
[跨设备迁移 runbook](runbooks/cross-device-transfer.md)。
