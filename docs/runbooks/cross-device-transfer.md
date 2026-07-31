# 跨设备迁移 runbook（发布 → 搬运 → 取回 → 安装）

把一台机器上的资产带到另一台，全程有摘要校验、有 diff 可审、有 backup 可回滚。

相关：[假 HOME 闭环](fake-home-loop.md)、[真机 dry-run](real-home-dry-run.md)、
语义见 [ADR-0007](../decisions/0007-immutable-channel-distribution.md)。

## 0. 先分清三件事

| | 是什么 | 谁负责 |
|---|---|---|
| **包** | `build` 产出的四件套，不可变 | aiah |
| **通道** | 一个**普通目录**：U 盘 / 挂载的 NAS、网盘 / git checkout | aiah 管布局与校验 |
| **搬运** | 把通道目录弄到另一台机器上 | **不是 aiah**：git / rsync / gh / U 盘 |

`aiah` 不实现 HTTP、git、rsync、WebDAV。搬字节的工具你已经有了，而且配好了凭据
与代理；aiah 只负责它们不负责的部分——不可变性、布局、两端校验。

### 0.1 TUI 连续向导与换机检查（当前源码候选 E3.2–E3.4）

`v0.1.6` 公开版的“迁移到其他设备”只提供只读状态。当前源码候选已把
下面第 1、2、4、5、7、8 步编排进同一个 TUI，同时保留所有安全边界：

```text
机器 A：aiah → 迁移到其他设备 → e 换机检查 → 选择资产组合
       → c 选择已有目录作为通道 → p
       → 选择资产组合 → 核对包/通道 → 输入 publish

机器 B：aiah → 迁移到其他设备 → c 选择已有目录作为通道 → v
       → 明确选择版本/profile → 输入已有输出目录
       → 取回版本检查 → Enter → 变更预览 → 输入 apply → 安装检查
```

TUI 不创建通道目录、不自动选择或取回最后发布项、不接管第 3 步传输；空目录只在
typed publish 成功后初始化索引和包布局。取回和 apply 仍是两个 Core 阶段。需要
自动化、精确 JSON 或故障定位时继续使用下列 CLI。

`e` 只读检查**当前资产库/profile**，适合机器 A 发布前使用：本机私有项按设计
不迁移；缺失 secret、不支持目标和 adapter 丢弃是阻止项；adapter 降级需确认。
它不生成安装包、不创建 `dist/`。

机器 B 取回后自动进入另一张“取回版本检查”页。该页绑定用户选择的
name/version/profile/SHA256，重新校验实际 `.tar`，并检查目标设备上的 secret、
本机排除项和 adapter 结果。坐标或摘要不匹配、有阻止项时不能进入 diff；检查通过
也要用户按 Enter，随后仍须审阅 diff 并完整输入 `apply`。所选 SHA256 会继续约束
diff/apply；即使包在检查后被替换也会拒绝。这不是自动安装。

## 1. 机器 A：建包

```bash
aiah validate --manifest ~/ai-assets/manifest.yaml --output json
aiah build --manifest ~/ai-assets/manifest.yaml --profile personal \
           --out /tmp/dist --output json
```

`ok=false` 就先修 findings，不要带着 error 往下走。

## 2. 机器 A：发布到通道

```bash
CHANNEL=/mnt/usb/aiah          # 或 ~/nas/aiah、~/git/ai-asset-channel
mkdir -p "$CHANNEL"
aiah publish --package /tmp/dist/<name>-<version>-personal.tar \
             --channel "$CHANNEL" --output json
```

验收：

| 检查 | 期望 |
|---|---|
| `ok` | `true` |
| `unchanged` | 首次发布为 `false`；重复发布同一份为 `true`（幂等） |
| `path` | `packages/<name>/<version>/<profile>` |
| `sha256` | 与 `/tmp/dist/*.sha256` 里的一致 |

**发布是不可变的**：同一个 (name, version, profile) 内容不同时会被**拒绝**，
且没有 `--force`。要改内容就抬版本号重新 build——「这台机器上的 2026.07.1」和
「那台机器上的 2026.07.1」必须是同一份字节，回滚与审计才有依据。

发布前 aiah 会核对源包摘要，并确认它能被读回为一个包；**损坏的包不会进入通道**。

## 3. 搬运（aiah 不参与）

按介质选一种，通道目录整体搬过去：

```bash
# U 盘 / 移动硬盘：直接拔走

# NAS / 网盘：已挂载则第 2 步就写进去了，无需额外动作

# git（通道就是一个仓库）
cd "$CHANNEL" && git add -A && git commit -m "publish <name> <version>" && git push

# rsync / scp
rsync -av --delete "$CHANNEL/" user@machine-b:/path/to/aiah-channel/
```

大包走 rsync 优于一次性 scp：断点续传，且能看出到底传完没有。

## 4. 机器 B：看通道里有什么

```bash
CHANNEL=/media/usb/aiah
aiah versions --channel "$CHANNEL" --output json
```

输出按**发布顺序**排列。这个顺序有意义：省略 `--version` 时取的就是最后一条。

## 5. 机器 B：取回

```bash
mkdir -p /tmp/incoming
aiah pull --channel "$CHANNEL" --name <name> --out /tmp/incoming --output json
```

验收：

| 检查 | 期望 |
|---|---|
| `ok` | `true` |
| `version` | 你要的那个 |
| `resolvedLatest` | 省略 `--version` 时为 `true`——**确认它解析到的版本是你想要的** |
| `files` | 四件套齐全 |

**`--version` 省略时取的是「最近发布」，不是「版本号最大」。** `2026.07.1` 与
`2026.07.10` 的字典序是错的，manifest 也不保证 semver，所以 aiah 干脆不解析
版本号。要确定性就显式传：

```bash
aiah pull --channel "$CHANNEL" --name <name> --version 2026.07.1 \
          --out /tmp/incoming --output json
```

取回后 aiah 会核对**真正落地的字节**；不一致会删掉本次新建文件并报错。输出目录
已有完整、逐字节相同的四件套时为幂等 no-op；只存在一部分或任一同名文件内容
不同则在写入前拒绝，绝不覆盖或删除原文件。

## 6. 机器 B：先假 HOME 闭环（强制）

不要拿真 `$HOME` 当第一个试验场。

```bash
W=$(mktemp -d)
PKG=/tmp/incoming/<name>-<version>-personal.tar
aiah apply    --package "$PKG" --home "$W/h" --project "$W/p" --output json
aiah scan     --home "$W/h" --project "$W/p" --output json | head
aiah rollback --home "$W/h" --project "$W/p" --output json
rm -rf "$W"
```

完成过假 HOME 闭环后，也可以用交互式入口合并下面第 7、8 步：

```bash
aiah bootstrap --channel "$CHANNEL" --name <name> --version 2026.07.1 \
  --out /tmp/incoming --home "$HOME"
```

它在 pull 前要求真实 TTY，随后进入与 `aiah ui --package` 相同的 diff 审阅；只有
完整输入 `apply` 才写 HOME。没有 `--yes`。取消时包留在 `/tmp/incoming`，HOME
不变；成功后退出 TUI 仍会在终端显示 `backupId` 与 rollback 命令。

## 7. 机器 B：真机 dry-run，人审 diff

```bash
aiah diff --package "$PKG" --home "$HOME" --output json > /tmp/diff.json
```

逐条读 `changes`，重点看 `update`（会覆盖既有内容）与这些高风险路径：
`~/.claude.json`、项目 `.mcp.json`、`~/.codex/config.toml`、`~/.grok/config.toml`、
项目根 `CLAUDE.md` / `AGENTS.md`。`findings` 里有 `error` 就先修，不要强行 apply。

## 8. 机器 B：安装，记住 backupId

```bash
aiah apply --package "$PKG" --home "$HOME" --output json | tee /tmp/apply.json
aiah doctor --home "$HOME" --output json      # 装完自查
# 出问题：
aiah rollback --home "$HOME" --backup <backupId> --output json
```

装完还要人工做的事：在 Claude / Codex 里 trust 或确认 hooks。apply 前必须确认
MCP 引用的环境变量已 export，或 `${secret:path}` 能被 `pass` 读取；包和 sidecar
仍只有引用，没有真 token，解析值只进入目标设备 native config。

## 9. 故障处理

| 现象 | 原因与处理 |
|---|---|
| publish 报 `already published with different content` | 同版本内容变了。**这是设计行为**，抬版本号重新 build，不要想办法覆盖 |
| publish 报 `does not load as a package` | 源包损坏或不是 build 产出的 tar，重新 build |
| publish 报 `is missing next to the package` | 四件套不全，`--package` 要指向 build 输出目录里的 `.tar` |
| pull 报 `checksum mismatch` | 通道里的包与其校验和不符：传输损坏或被改过。重新从机器 A 发布并重传 |
| pull 报 `index lists ... but ... is missing from the tree` | 索引与目录树不一致（多半是搬运时漏了文件）。整体重传通道目录 |
| pull 报 `release not found` | 先跑 `aiah versions` 看清 name/version/profile 拼写 |
| 取回的版本不是预期 | 省略了 `--version` 时取的是**最近发布**；显式传 `--version` |
| bootstrap 报 `interactive TTY` | 当前是管道/CI；改用独立 `pull`、`diff`、`apply`，不要绕过确认 |

## 10. 不要做的事

- 不要手工往通道目录里塞文件——布局与 `channel.json` 会失去一致，`pull` 会
  fail-closed。
- 不要为了「修一下」而在通道里原地改包：版本号不再唯一确定内容，回滚与审计就废了。
- 不要跳过第 6、7 步直接对真 `$HOME` apply。
- 不要把 `.aiah/backups` 提交到 Git 或同步到不可信介质。
- 不要把含真实 token 的包放进共享通道；`sensitivity: sensitive` 的资产只允许
  `${ENV:...}` / `${secret:...}` 引用。

## 11. 与自动化测试的关系

`internal/channel` 的测试覆盖不可变性、两端校验、失败回滚与「不发明版本序」，
全部只用临时目录。本 runbook 的真机步骤是人工流程，不进 CI。
