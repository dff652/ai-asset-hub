# 上手指南

本指南从现有工具目录的只读盘点开始，走到可审计、可回滚的首次部署。命令参数总表
见[命令参考](cli-reference.md)；真实 HOME 的逐项检查见
[真机 dry-run runbook](runbooks/real-home-dry-run.md)。

## 安装

当前安装和 Release 支持范围为 **Linux amd64**。推荐先下载并阅读安装器：

```bash
curl -fsSLo /tmp/aiah-install.sh \
  https://raw.githubusercontent.com/dff652/ai-asset-hub/main/scripts/install.sh
less /tmp/aiah-install.sh
sh /tmp/aiah-install.sh
```

安装器默认固定 `0.1.2`，也可显式设置：

```bash
AIAH_VERSION=0.1.2 AIAH_INSTALL_DIR="$HOME/.local/bin" \
  sh /tmp/aiah-install.sh
```

从 [Release](https://github.com/dff652/ai-asset-hub/releases/tag/v0.1.2)
手动下载时，必须同时下载 `SHA256SUMS`。Linux amd64 示例：

```bash
sha256sum -c SHA256SUMS --ignore-missing
chmod +x aiah_0.1.2_linux_amd64
mkdir -p "$HOME/.local/bin"
install -m 0755 aiah_0.1.2_linux_amd64 "$HOME/.local/bin/aiah"
aiah version
```

安装器不会修改 PATH；命令找不到时把 `~/.local/bin` 加入 PATH。

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

## 2. 只读盘点

```bash
aiah scan --home "$HOME" --output json > /tmp/aiah-scan.json
aiah ui
```

`scan` 和不带写参数的 `ui` 不写 HOME/project、不执行 hook，也不跟随逃逸软链接。
TUI 提供 source → type → asset 树、详情、`/` 过滤、`f` findings-only 和 `r`
重扫。非 TTY 环境应使用 JSON 命令。

盘点结果中的 `candidate` 只是迁移候选，不代表应原样打包。凭据、session、cache、
数据库和疑似 secret 会被排除或脱敏报告。

## 3. 建立工作区

工作区布局与 manifest 字段见[资产模型](asset-model.md)。可以手工整理，也可以显式
让 TUI 组装：

```bash
aiah ui --home "$HOME" --workspace ~/ai-assets
```

只有给出 `--workspace` 才会出现勾选和 `w` 写出能力。TUI 把选中资产复制进工作区并
登记到 `manifest.yaml`；已有文件 create-only，不覆盖。校验失败会回滚本次创建的
文件和目录。

每条资产的四个属性决定后续行为：

- `targets`：部署到哪些工具；
- `scope`：`global` 写 HOME、`project` 写项目根、`device` 永不 apply；
- `portability`：原样分发或需要 adapter；
- `sensitivity`：敏感 MCP env 只允许 `${ENV:...}` / `${secret:...}` 引用。

项目自己的 `CLAUDE.md` / `AGENTS.md` 由项目 Git 管理。aiah 会盘点 missing 或
shadowing，但不会替项目自动初始化、复制、删除或改名。完整边界见
[资产模型 §4.1](asset-model.md#41-claudemd-与-agentsmd-的处理)。

## 4. 校验并构建

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

## 5. 先走假 HOME 闭环

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

## 6. 真机先 diff，再 apply

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
