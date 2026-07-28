# AI Asset Hub

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

面向个人与小团队的 AI 编程资产管理、打包和跨设备部署工具。

> **状态：Technical Preview。** 核心 CLI MVP、跨设备分发与 TUI Phase C 已可用，
> 但安装脚本和 Windows 写入行为验收仍在路线图中。

项目不以复刻 PromptHub 为目标，而是借鉴其资产工作区、扫描、版本比较和分发思路，解决以下问题：

- Skills、Rules、Memory、Agents、Hooks、MCP 模板分散在不同工具目录。
- Claude Code、Codex 等工具的格式和加载路径不同。
- 更换设备时缺少可审计、可回滚的一键恢复过程。
- 网盘同步实时数据库容易冲突，密钥随配置备份存在泄露风险。
- 工具原生会话、缓存和长期可复用知识没有清晰边界。

## 产品定位

AI Asset Hub 是一个“文件优先、工具无关、可打包迁移”的 AI 资产编译器和部署器：

```text
资产目录（唯一事实源）
        │
        ▼
校验、规范化、按 Profile 选择
        │
        ▼
构建不可变资产包
        │
        ├── 网盘 / NAS / Git / 移动介质
        ▼
新设备 bootstrap
        │
        ├── Claude Code adapter
        ├── Codex adapter
        └── Grok / 其它 Target adapter（按 ADR-0002 演进）
```

PromptHub 可以作为产品和交互设计参考，也可以作为可选的浏览、编辑界面，但不作为运行时依赖或唯一事实源。多端（Claude / Codex / Grok 等）采用能力矩阵与可插拔 Target，详见 [ADR-0002](docs/decisions/0002-multi-target-capability-adapters.md)。

## 核心原则

1. 纯文本文件是事实源，SQLite 只能作为可删除、可重建的索引。
2. 网盘保存版本化、不可变的资产包，不同步正在使用的数据库。
3. 通用内容与工具适配分离，同一资产通过 adapter 生成 Claude/Codex 目标格式。
4. 资产、设备配置和密钥分层，资产包中不得包含真实 Token。
5. 安装前必须支持校验和 diff，安装后必须支持回滚。
6. 原生会话历史和缓存默认不迁移；只有整理后的长期知识才进入 Memory。
7. PromptHub 源码为 AGPL-3.0；本项目若采用独立许可证，应保持独立实现，不复制其实现代码。

## 项目说明文件

本仓库以根 [`CLAUDE.md`](CLAUDE.md) 作为 Claude Code / Codex 共用的项目说明；
Codex 通过 `project_doc_fallback_filenames = ["CLAUDE.md"]` 读取，不另建
`AGENTS.md`。两个文件都不存在的项目应先根据仓库事实整理并审阅一份项目说明，
不应先让两个工具各自初始化出两份会漂移的摘要。

这是项目作者策略，不是 aiah 当前的自动写能力。aiah 能扫描 `CLAUDE.md` /
`AGENTS.md` 并报告 shadowing，但不会自动删除、复制或对齐它们。完整边界见
[资产模型 §4.1](docs/asset-model.md#41-claudemd-与-agentsmd-的处理)。

## 已实现 / 规划中的命令

```bash
aiah scan --home <path> [--project <path>] --output json
aiah validate --manifest <path> [--root <path>] --output json
aiah build --manifest <path> --profile <name> --out <dir> [--root <path>] --output json
aiah diff --package <tar|dir> [--home <path>] [--project <path>] [--targets claude,codex,grok] --output json
aiah apply --package <tar|dir> [--home <path>] [--project <path>] [--targets claude,codex,grok] [--dry-run] --output json
aiah rollback [--home <path>] [--project <path>] [--backup <id>] --output json
aiah publish --package <tar> --channel <dir> --output json
aiah pull --channel <dir> --name <name> [--version <v>] [--profile <p>] --out <dir> --output json
aiah versions --channel <dir> [--name <name>] --output json
aiah bootstrap --channel <dir> --name <name> [--version <v>] [--profile <p>] --out <dir> [--home <path>] [--project <path>] [--targets claude,codex,grok]
aiah doctor [--home <path>] [--project <path>] --output json
aiah ui [--home <path>] [--project <path>] [--workspace <path>] [--package <tar|dir>] [--targets claude,codex,grok]
aiah mcp                          # 只读 MCP server（stdio），供 AI 工具调用
aiah version [--output text|json]
```

### 跨设备：发布到通道，另一台机器拉回来

```bash
# 机器 A
aiah build --manifest ~/ai-assets/manifest.yaml --profile personal --out /tmp/dist
aiah publish --package /tmp/dist/<name>-<version>-personal.tar --channel /mnt/usb/aiah

# 把 /mnt/usb/aiah 搬过去：拔 U 盘、git push、rsync、gh release upload —— 随你

# 机器 B
aiah versions --channel /mnt/usb/aiah
aiah pull --channel /mnt/usb/aiah --name <name> --out /tmp/incoming
aiah diff  --package /tmp/incoming/<...>.tar --home "$HOME"   # 先看，再装
aiah apply --package /tmp/incoming/<...>.tar --home "$HOME"

# 或使用强制交互入口（仍会先展示 diff，并要求完整输入 apply）
aiah bootstrap --channel /mnt/usb/aiah --name <name> \
  --out /tmp/incoming --home "$HOME"
```

**通道就是一个普通目录**：U 盘、挂载的 NAS/网盘、或一个 git checkout 都行。
**aiah 不做网络传输**——把字节搬过网络是 `git` / `rsync` / `gh` / U 盘的事，
它们做得更好而且你已经配好了凭据。aiah 负责它们都不负责的部分：不可变性、布局、
两端的完整性校验。

发布是**不可变**的：同一个 (name, version, profile) 内容相同则幂等，内容不同则
拒绝，**没有 `--force`**——要改内容就换版本号。发布前会核对源包摘要并确认它能被
读回为一个包（决不发布损坏的包），拉取后会核对落地文件，不一致就删掉并报错。

省略 `--version` 取的是**最近发布**的那个，不是版本号最大的那个：`2026.07.1` 与
`2026.07.10` 的字典序是错的，而 aiah 不解析版本号。要确定性就显式传 `--version`。
语义见 [ADR-0007](docs/decisions/0007-immutable-channel-distribution.md)，
逐步流程与故障处理见[跨设备迁移 runbook](docs/runbooks/cross-device-transfer.md)。

`bootstrap` 不提供 `--yes` 或非交互模式：它在 pull 前要求真实 TTY，取回后直接进入
TUI Phase C。取消或 diff 失败不会写 HOME，但已验证的包会保留在显式 `--out`；
成功后普通终端会保留 `backupId` 和完整 rollback 命令。边界见
[ADR-0008](docs/decisions/0008-interactive-bootstrap.md)。

### 从盘点直接组装工作区（TUI Phase B）

```bash
aiah ui --home "$HOME" --workspace ~/ai-assets
```

给了 `--workspace` 才开写能力：空格勾选候选资产，`w` 把它们**复制进工作区**并登记
进 `manifest.yaml`，随即自动 `validate`。**不给 `--workspace` 时界面保持只读**，
不显示复选框——没有默认工作区路径，猜写入目标不是这个工具该做的事。

写入面只有工作区：**永不写 `.claude` / `.codex` / `.grok`**，工作区里已存在的文件
也不覆盖（create-only）。校验不过就删掉临时 manifest 并回滚本次创建的文件与目录，
不留半成品。已有 manifest 走就地编辑，注释、键序和本版本还不认识的字段都保留。
边界见 [ADR-0006](docs/decisions/0006-tui-as-first-interactive-surface.md)；
不给 `--package` 时不会出现部署执行入口。

### 在 TUI 审阅并执行部署（Phase C）

```bash
aiah ui --package /tmp/incoming/<...>.tar --home "$HOME" --targets claude,codex
```

给了 `--package` 后先调用与 CLI 相同的 `apply.Diff`，按 create / update /
unchanged / skipped 分组展示计划。按 `a` 只会进入第二次确认；必须完整输入
`apply` 并按 Enter 才调用 `apply.Apply`。成功后界面显著显示 `backupId` 和可直接
执行的完整 rollback 命令；失败时原样展示 Core findings。`Esc` 返回 inventory，
`d` 重新计算只读 diff。非 TTY 模式提示改用 `aiah diff --output json`。

### 给 AI 工具接入（MCP）

`aiah mcp` 在 stdio 上提供只读 MCP server，暴露 `aiah_scan` / `aiah_validate` /
`aiah_diff` / `aiah_doctor` / `aiah_version` 五个工具。**`apply` 与 `rollback`
不在其中**：它们写 HOME，而 agent 的运行时配置就在同一个 HOME 里；需要写操作时
请走 CLI，由人执行。`build` 同样不暴露，因为它的写入目标由调用方指定——排除后
「经此 server 零写入」才是一条绝对的、可测试的不变式。边界与理由见
[ADR-0005](docs/decisions/0005-read-only-mcp-server-surface.md)。

Claude Code 接入：

```bash
claude mcp add aiah -- aiah mcp
```

Codex / 其它客户端在各自配置里声明 `command: "aiah"`、`args: ["mcp"]` 即可；该
子命令不接受任何 flag 或 operand。

`aiah doctor` 已在 `dev` 实现，将进入首个 public 版本 `v0.1.1`；此前的 private
`v0.1.0` 验证版本不包含该命令。

## 安装已发布版本

从 [GitHub Releases](https://github.com/dff652/ai-asset-hub/releases) 下载与平台
匹配的裸二进制和同一 Release 中的 `SHA256SUMS`：

| 系统 | amd64 | arm64 |
|---|---|---|
| Linux | `aiah_<version>_linux_amd64` | `aiah_<version>_linux_arm64` |
| macOS | `aiah_<version>_darwin_amd64` | `aiah_<version>_darwin_arm64` |
| Windows | `aiah_<version>_windows_amd64.exe` | `aiah_<version>_windows_arm64.exe` |

以下示例用于首个 public 版本 `v0.1.1`；其他 Unix 平台替换 `AIAH_ASSET`：

```bash
(
set -eu
AIAH_VERSION=0.1.1
AIAH_ASSET="aiah_${AIAH_VERSION}_linux_amd64"
AIAH_BASE="https://github.com/dff652/ai-asset-hub/releases/download/v${AIAH_VERSION}"
AIAH_TMP="$(mktemp -d)"
trap 'rm -rf "$AIAH_TMP"' EXIT

curl -fL "$AIAH_BASE/$AIAH_ASSET" -o "$AIAH_TMP/$AIAH_ASSET"
curl -fL "$AIAH_BASE/SHA256SUMS" -o "$AIAH_TMP/SHA256SUMS"

AIAH_EXPECTED="$(awk -v name="$AIAH_ASSET" '$2 == name { print $1 }' "$AIAH_TMP/SHA256SUMS")"
if command -v sha256sum >/dev/null 2>&1; then
  AIAH_ACTUAL="$(sha256sum "$AIAH_TMP/$AIAH_ASSET" | awk '{ print $1 }')"
else
  AIAH_ACTUAL="$(shasum -a 256 "$AIAH_TMP/$AIAH_ASSET" | awk '{ print $1 }')"
fi
if [ -z "$AIAH_EXPECTED" ] || [ "$AIAH_ACTUAL" != "$AIAH_EXPECTED" ]; then
  echo "checksum verification failed for $AIAH_ASSET" >&2
  exit 1
fi

mkdir -p "$HOME/.local/bin"
install -m 0755 "$AIAH_TMP/$AIAH_ASSET" "$HOME/.local/bin/aiah"
"$HOME/.local/bin/aiah" version
)
```

Linux amd64 已完成本地端到端行为验证。其他 Release 目标均经过交叉编译；在各平台
完成原生行为验收前，不把“有二进制”表述为“完整支持”。尤其 Windows 的 apply /
rollback / hooks、文件 mode、软链和配置根语义仍待单独验证。

## 快速上手

### 心智模型：三个目录，别混

| 目录 | 角色 | 谁写 |
|---|---|---|
| **工作区** `~/ai-assets/` | 唯一事实源，纯文本 + `manifest.yaml`，建议 git init | 你手写 |
| **包** `*.tar` | 不可变产物，带 lock 与 sha256 | `aiah build` 产出 |
| **目标** `~/.claude` `~/.codex` `~/.grok` | 工具实际加载的位置 | 只由 `aiah apply` 写 |

**关键：目标目录是产物不是源头。** 上手之后不再直接编辑
`~/.claude/skills/<x>`，而是改工作区再 apply。

### 五步

```bash
# ① 只读盘点现状（不写盘、不执行 hook、不跟随逃逸软链）
aiah scan --home "$HOME" --output json > /tmp/scan.json
aiah ui                                    # 或用 TUI 浏览：/ 过滤、f 只看 findings、r 重扫

# ② 建工作区 ~/ai-assets/{manifest.yaml,assets/{skills,rules,agents,hooks,mcp}}

# ③ 校验 + 打包
aiah validate --manifest ~/ai-assets/manifest.yaml --output json
aiah build --manifest ~/ai-assets/manifest.yaml --profile personal \
           --out /tmp/aiah-dist --output json
# 产出 <name>-<version>-<profile>.{tar,lock.json,manifest.json,sha256}

# ④ 假 HOME 全写闭环（强制，别跳）
W=$(mktemp -d); PKG=/tmp/aiah-dist/<name>-<version>-personal.tar
aiah apply    --package "$PKG" --home "$W/h" --project "$W/p" --output json
aiah scan     --home "$W/h" --project "$W/p" --output json | head
aiah rollback --home "$W/h" --project "$W/p" --output json && rm -rf "$W"

# ⑤ 真机：先 dry-run 再 apply，记住 backupId
aiah apply --dry-run --package "$PKG" --home "$HOME" --output json   # 验收 dryRun=true / ok=true / 无 error
aiah apply           --package "$PKG" --home "$HOME" --output json
aiah doctor          --home "$HOME" --output json
aiah rollback --home "$HOME" --backup <id> --output json
```

`manifest.yaml` 里每条资产的四个属性决定全部行为：`targets`（装到哪几端）、
`scope`（`global` 写 HOME / `project` 写项目根 / `device` **永不 apply**）、
`portability`（`portable` 原样 / `adapter-required` 按端编译）、
`sensitivity`（敏感 MCP env 只允许 `${ENV:...}` / `${secret:...}` 引用）。完整字段见
`spec/manifest.schema.json`，真机流程见
[real-home-dry-run](docs/runbooks/real-home-dry-run.md)。

三个必知边界：**hook 必须有 shebang**（否则 `hook_invalid` 整单失败，且 aiah
只落盘不 trust 不执行）；**MCP 是 create-only**（原生配置已存在且同名 server
内容冲突 → 整个 apply fail-closed）；**`.aiah/backups` 不得进 Git 或不可信云盘**。

## 构建与版本

```bash
go build -o build/aiah ./cmd/aiah   # 版本显示为 dev
./scripts/build.sh                  # 注入 git 版本/commit/时间
VERSION=0.1.0 ./scripts/build.sh    # 发版用
```

每个 JSON 报告和每条部署记录都带 `producedBy`（如
`aiah 0.1.0+d9dd3b263a6f`）。这个工具会写别人的 home 目录，安装记录必须能回答
「是哪个二进制、按哪套 adapter 与分类规则装的」。`go build` 出来的二进制版本是
`dev`，这本身就是准确信息。

包内 `manifest.json` **暂不带**该字段：包读取用 `DisallowUnknownFields`，加字段会
让旧二进制读不了新包，属于格式变更，需要一并抬 `schemaVersion`。

需要 Go 1.26.5。构建和测试：

```bash
go test ./...
go build -o build/aiah ./cmd/aiah
```

假 HOME 闭环（不碰真实家目录）：

```bash
./scripts/demo-apply-scan-loop.sh
./scripts/demo-apply-scan-loop.sh workspace-2b
```

说明见 [假 HOME 闭环](docs/runbooks/fake-home-loop.md)。真机只读预演见 [real-home-dry-run](docs/runbooks/real-home-dry-run.md)（默认不对真实 `$HOME` 执行 apply）。
新设备先按[开发环境搭建 SOP](docs/runbooks/development-environment.md)运行
`./scripts/dev-doctor.sh`；缺少固定版本工具链时再显式执行
`./scripts/bootstrap-dev.sh`。提交前用 `./scripts/check-local.sh` 跑完整本地门禁。

## Phase 0：只读资产盘点

```bash
aiah scan --home <home-path> [--project <project-path>] --output json
```

只读取 Claude Code、Codex 和共享资产目录，输出
`spec/inventory.schema.json` 对应的确定性 JSON。扫描不会写入 HOME 或项目目录，
不会执行 hook，也不会跟随逃逸软链接。

```bash
./build/aiah scan \
  --home testdata/home-basic \
  --output json
```

```bash
./build/aiah scan \
  --home testdata/home-conflicts \
  --project testdata/home-conflicts/workspace \
  --output json
```

输出路径使用 `home/...` 或 `project/...`，不含真实绝对路径。凭据、session、
cache、数据库只报告排除原因；疑似 secret 会隐藏内容哈希。`candidate` 仅表示
盘点候选。

Inventory 区分：

- `assets`：可迁移逻辑单元；
- `entries`：文件与排除/软链接事实；
- `summary.candidateByType`：只统计候选 Asset。

Skill 仅当 `skills/<name>/SKILL.md` 为常规文件时建资产。

## TUI Phase A：只读浏览

```bash
aiah ui --home <home-path> [--project <project-path>]
```

TUI 直接在同一进程调用 inventory 扫描，提供 source → type → asset 树、详情、
`/` 增量过滤、`f` findings 分诊和 `r` 重扫。Phase A 不写 HOME/project，不执行
build/apply/rollback，也不保存私有设置。stdin/stdout 不是 TTY，或 `TERM` 为空 /
`dumb` 时直接失败并提示使用 `scan --output json`。

## 部署状态自查

```bash
aiah doctor [--home <home-path>] [--project <project-path>] --output json
```

`doctor` 只读检查 `.aiah` 中未结 apply journal、残留 stage、backup
metadata/payload 完整性和当前 deployment，并对新格式部署按记录的 SHA256 与 mode
报告 `unchanged` / `locally-modified` / `missing`。它还会提前提示 MCP 原生配置的
软链、空文件/非法格式和空 `args` 数组——这些状态会让 create-only apply
fail-closed。

旧 deployment 没有文件 hash/mode 时明确返回 `drift_unavailable` 与 `unchecked`
计数，不会猜测为健康。warning 不令 `ok` 变 false；未结 journal、损坏 backup 或
非法 deployment 属 error，进程退出 1。`scripts/dev-doctor.sh` 检查开发工具链，
与这个面向用户资产状态的 `aiah doctor` 不是同一个命令。

## Phase 1A：只读 manifest 校验

```bash
aiah validate --manifest <manifest.yaml|json> [--root <workspace>] --output json
```

在工作区根（默认 manifest 所在目录）上只读校验：

- manifest v1 schema；
- 重复 asset id、依赖/冲突/profile 引用；
- 绝对路径与 `..` 越界；
- 软链接逃逸/损坏；
- 疑似密钥、二进制、超大文件。

不写工作区，不生成资产包，不执行 hook/MCP。`ok: false` 或存在 error 级
finding 时进程退出码为 1。报告符合 `spec/validation.schema.json`，路径为相对
工作区路径，不回显 secret 原文。

```bash
./build/aiah validate \
  --manifest testdata/workspace-valid/manifest.yaml \
  --output json
```

## Phase 1B：确定性 build

```bash
aiah build \
  --manifest <manifest.yaml|json> \
  --profile <name> \
  --out <dir> \
  [--root <workspace>] \
  --output json
```

先对工作区执行与 `validate` 相同的 fail-fast 检查，再按 profile 选择资产
（include/exclude，并展开未排除的依赖），写出确定性产物：

| 文件 | 内容 |
|---|---|
| `{name}-{version}.tar` | 包根目录内含 `manifest.json`、`lock.json` 与资产文件 |
| `{name}-{version}.manifest.json` | 解析后的包 manifest（含每文件 sha256） |
| `{name}-{version}.lock.json` | 文件哈希清单 |
| `{name}-{version}.sha256` | tar 的 sha256 校验行 |

不修改源工作区，不写入 `~/.claude` / `~/.codex`。tar 内时间戳/uid 固定，
相同输入重复构建应得到相同 archive 字节与 digest。疑似密钥、软链接与校验失败
导致 `ok: false` 且不写最终产物（临时目录原子发布）。manifest schema 已 embed
进二进制，不依赖当前工作目录下的 `spec/`。stdout 为 `spec/build.schema.json`
风格报告，并含 targets 能力摘要。

```bash
./build/aiah build \
  --manifest testdata/workspace-valid/manifest.yaml \
  --profile personal \
  --out /tmp/aiah-dist \
  --output json
```

## 文档

- [文档索引](docs/README.md)
- [总体架构](docs/architecture.md)
- [资产模型](docs/asset-model.md)
- [安全与隐私](docs/security.md)
- [漏洞报告政策](SECURITY.md)
- [MVP 路线图](docs/roadmap.md)
- [架构决策：文件优先与 adapter 分发](docs/decisions/0001-file-first-adapter-distribution.md)
- [架构决策：多 Target 能力模型与可插拔适配](docs/decisions/0002-multi-target-capability-adapters.md)
- [架构决策：MCP 原生配置所有权与安全写入](docs/decisions/0004-native-mcp-config-ownership.md)
- [PromptHub 调研与采用边界](docs/research/prompthub-assessment.md)
- [AI 资产管理踩坑清单](docs/troubleshooting/ai-asset-pitfalls.md)
- [第三方依赖许可证清单](docs/licenses/third-party.md)
- [外部参考](docs/references.md)

## Phase 2A：部署（diff / apply / rollback）

从 Phase 1 资产包安装到临时或真实 HOME（只写 adapter 声明路径）：

```bash
./build/aiah build \
  --manifest testdata/workspace-valid/manifest.yaml \
  --profile personal \
  --out /tmp/aiah-dist

./build/aiah diff \
  --package /tmp/aiah-dist/fixture-personal-2026.07.1.tar \
  --home /tmp/fake-home \
  --targets claude,codex

./build/aiah apply \
  --package /tmp/aiah-dist/fixture-personal-2026.07.1.tar \
  --home /tmp/fake-home \
  --targets claude,codex

./build/aiah rollback --home /tmp/fake-home
```

Phase 2 映射：

| 包内路径 | Claude | Codex | Grok | Shared |
|---|---|---|---|---|
| `assets/skills/<name>/…` | `~/.claude/skills/…` | `~/.codex/skills/…` | `~/.grok/skills/…` | `~/.agents/skills/…` |
| `assets/rules/…` | `~/.claude/rules/…` | `~/.codex/rules/…` | `~/.grok/rules/…` | — |
| `assets/rules/CLAUDE.md`（scope=project） | 项目根 `CLAUDE.md` | — | — | — |
| `assets/agents/…` | `~/.claude/agents/…` | `~/.codex/agents/…` | `~/.grok/agents/…` | — |
| `assets/hooks/…` | `~/.claude/hooks/…` | `~/.codex/hooks/…` | `~/.grok/hooks/…` | — |
| `assets/mcp/…` | sidecar `~/.claude/mcp/…`；native user `~/.claude.json` / project `.mcp.json` | sidecar `~/.codex/mcp/…`；native `~/.codex/config.toml` / project `.codex/config.toml` | sidecar `~/.grok/mcp/…`；native `~/.grok/config.toml` / project `.grok/config.toml` | — |

`apply --home` 安装 global 资产；`apply --project` 安装 project 资产；可同时给。
`device` scope 永不安装。同路径不同内容会 `path_collision` 失败。
MCP 模板允许 `${ENV:}` / `${env:}` / `${secret:}` 引用；原始密钥 fail-closed。
apply 计划阶段把前两者解析为非空环境变量，把 `${secret:path}` 解析为
`pass show -- path` 的第一行；解析失败整单不写。sidecar 和包始终保留引用，只有
设备本地一次性创建的 native config 含真值，报告、journal 与 backup 元数据不含
真值。现行行为按
ADR-0004 收口为一次性 create-only bootstrap：native config 不存在时创建；从创建
完成起即视为用户/harness 文件，后续包版本只检查 identical/conflict，永不自动
更新。同名冲突会令整个 apply fail-closed。2C.3.1/2C.3.2 已于 2026-07-25
[通过严格复审](docs/reviews/2026-07-25-mcp-create-only-strict-review.md)，含 MCP
asset 的包解除「仅 dry-run」限制；长期流程仍是先假 HOME 闭环、再 `--dry-run` 看
diff、人工确认后再 apply。已知边界：已有原生配置是软链、0 字节/非法 JSON，或把
等价内容写成 `"args": []` 时，当前会让整单 apply 失败（复审 P6，待决策）。
备份写入 meta 根（优先 home）下 `.aiah/backups/<id>/`；安装中途失败会按备份
自动回滚已提交文件；恢复不完整时保留 journal。部署和回滚拒绝目标祖先软链接、
不属于包的 target 和不安全的备份元数据。外部 tar/目录包会验证成员类型、大小、
路径、重复项及 manifest/lock/hash 关系。重复 apply 幂等。

## 当前状态

已实现完整 Phase 1、经安全收口的 Phase 2A/2B 部署，以及 Phase 2C 全部子阶段
（2C.1、2C.2、2C.3.1/2C.3.2、2C.4）。MCP create-only 所有权、finding 契约与
policy 边界已于 2026-07-25 通过严格复审（六条门槛逐条实证）。
资产、包和 sidecar 只保留密钥引用；真实值仅在目标设备 apply 时进入 native
config。云服务尚未实现。

TUI Phase A/B/C 均已实现并完成真机 PTY dogfood。
private `v0.1.0` 已完成六平台二进制、许可材料、SHA256 与版本信息的发布链路验收；
首个 public 版本计划为包含 `doctor` 的 `v0.1.1`。
下一步与待决策项见 [MVP 路线图](docs/roadmap.md)。

产品采用 **CLI-first、Go Core**：当前不引入 npm/TypeScript launcher；优先发布
多平台原生二进制。第一个界面是本地 TUI，不是 Web UI；所有写操作复用同一 Go
Core。
决策与启动门槛见
[ADR-0003](docs/decisions/0003-cli-first-go-core-and-product-surfaces.md)。

## 许可证

本项目以 [Apache-2.0](LICENSE) 发布，版权署名见 [NOTICE](NOTICE)。

第三方依赖全部为 MIT / Apache-2.0 / BSD 系宽松协议，无 copyleft 传染，清单见
[第三方依赖许可证清单](docs/licenses/third-party.md)。本项目不复制
PromptHub（AGPL-3.0）源码，也不移植其受版权保护的实现细节，边界见
[安全与隐私](docs/security.md) §6。
