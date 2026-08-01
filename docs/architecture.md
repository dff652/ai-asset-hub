# 总体架构

## 1. 目标

AI Asset Hub 负责把工具无关的 AI 资产编译成各 AI 工具能够直接加载的文件，并在不同设备上可重复部署。

它不是：

- Prompt 聊天客户端；
- 云端密钥保险箱；
- 任意 AI 工具的会话数据库迁移器；
- 必须常驻运行的同步服务；
- PromptHub 的私有 Fork。

## 2. 分层

```text
┌──────────────────────────────────────────────────┐
│ Source（唯一事实源）                               │
│ Skills / Rules / Memory / Agents / MCP 模板等    │
└────────────────────────┬─────────────────────────┘
                         │ validate + profile
┌────────────────────────▼─────────────────────────┐
│ Core IR                                          │
│ Manifest / Profile / Capability / Secret Ref     │
│ TargetSet（不写 HOME）                             │
└────────────────────────┬─────────────────────────┘
                         │ per-target compile
┌──────────────┬─────────┼──────────┬──────────────┐
│ AdapterClaude│ AdapterCodex │ AdapterGrok │ …   │
│ → staging/<target>/ only                         │
└──────────────┴─────────┴──────────┴──────────────┘
                         │ shared diff / apply
┌────────────────────────▼─────────────────────────┐
│ Device（派生态）                                    │
│ ~/.claude ~/.codex ~/.grok ~/.agents 项目目录…   │
└──────────────────────────────────────────────────┘
```

多端细节与演进顺序见
[ADR-0002：多 Target 能力模型与可插拔适配](decisions/0002-multi-target-capability-adapters.md)。
产品界面、实现语言和跨平台边界见
[ADR-0003：CLI-first、Go Core 与产品界面演进](decisions/0003-cli-first-go-core-and-product-surfaces.md)。

### Source

保存人可读、可审查、适合进入 Git 的原始资产。优先沿用已经具备跨工具基础的标准，例如 `SKILL.md`。平台专属字段放 sidecar，不污染公共正文。

### Core

负责：

- 解析 `manifest.yaml`；
- 合并通用、Profile 和设备配置；
- 按 **Capability** 与 Target 注册表解析依赖、冲突与部署计划；
- 校验引用和敏感信息；
- 生成锁文件和内容哈希；
- 组织构建产物。

Core 只认识 target id 与能力矩阵，不应硬编码各工具 HOME 路径，也不直接修改用户目录。

### Adapters / Probes

每个 harness 是一个 Target：可选 **Probe**（只读盘点）+ **Adapter**（编译到 staging）。

- Claude Code：`CLAUDE.md`、Skills、Agents、Hooks、MCP JSON；
- Codex：`AGENTS.md`、Skills、Agent 配置、MCP TOML；
- Grok Build：已覆盖 `~/.grok` / 项目 `.grok` 的只读 Probe 与部署子集，尚不宣称
  全量语义兼容；
- 过滤目标不支持的 frontmatter，输出 `dropped` / `degraded` 报告；
- Adapter 之间不得互引；共享逻辑放 skill 解析、secret、staging/apply。

### Apply

所有安装都经过临时 staging 目录，与具体 harness 解耦：

1. 编译到 staging；
2. 在设备侧解析 MCP `env` 的 Secret Ref（环境变量 / `pass`），失败则整单不写；
3. 验证目录和文件格式；
4. 显示目标 diff；
5. 备份将被覆盖的文件；
6. 原子替换；
7. 运行目标工具 doctor（若有）；
8. 写入部署记录。

解析值只进入设备本地 native MCP config，不进入 sidecar、报告、日志、journal 或
backup 元数据。

## 2.1 多 Target 原则（摘要）

1. **新工具 = 注册表 + Probe + Adapter + fixture**，禁止在 Core 堆工具名分支。
2. **能力表驱动**可移植性；工具名只出现在边界。
3. **T0 共享落点**（如 `~/.agents/skills`）优先于 N 份拷贝。
4. **T1** 中立正文 + sidecar；**T2** 表驱动映射；**T3** 单 target 且显式不可移植。
5. 盘点区分 **权威资产** 与 **compat 加载关系**（`loadedBy`），迁移只数逻辑 Asset。
6. 设备私有（auth、sessions、cache、bundled、marketplace-cache 等）默认排除。

## 2.2 人工入口与 AI 入口

TUI/CLI 与 MCP 调用同一 Core，但权限不同：

- TUI 是人工操作台，可以在明确路径、diff 和 typed confirmation 后写资产库或
  目标工具目录；
- CLI 是完整自动化接口，写操作由调用方显式承担路径、确认、错误恢复和审计；
- `aiah mcp` 是 AI 工具的**只读查询面**。public `v0.1.10` 注册 8 个工具（含
  `aiah_migration_readiness`），覆盖盘点、统一资产状态、校验、包 diff、安装检查、
  迁移状态、迁移准备和版本；
- MCP 不注册 build、资产库纳入/更新/移出、publish/pull、apply/rollback 或证据写入。
  MCP handler 只做参数解码和 Core 报告转发，不复制分类、版本对齐或路径规则。

完整边界见 [ADR-0005](decisions/0005-read-only-mcp-server-surface.md)。

## 3. 事实源与派生状态

| 数据 | 是否事实源 | 是否进入资产包 |
|---|---:|---:|
| `assets/` 纯文本资产 | 是 | 是 |
| `profiles/` 非敏感配置 | 是 | 是 |
| `manifest.yaml`、锁文件 | 是 | 是 |
| SQLite 搜索索引 | 否 | 否 |
| 编译后的 `.claude/`、`.codex/`、`.grok/` 等 | 否 | 可选预览 |
| 安装恢复点（`.aiah/backups`） | 否 | 否 |
| API Key、Token | 否 | 否 |
| 会话数据库、缓存 | 否 | 默认否 |

## 4. 包与跨设备分发

构建输出采用普通、可解压的开放格式，产物名含 profile（不同 profile 曾互相覆盖，
见评审 P8）：

```text
dist/
├── <name>-<version>-<profile>.tar
├── <name>-<version>-<profile>.lock.json
├── <name>-<version>-<profile>.manifest.json
└── <name>-<version>-<profile>.sha256
```

网盘、NAS、Git Release、WebDAV 或移动介质只负责传输这些不可变产物。首版不实现多端实时合并。

术语边界：

- `apply` 创建的 backup 是设备本地**安装恢复点**，只服务于明确的 rollback；
- **资产库备份**应保留历史、支持独立恢复并校验结果，当前由私有 Git、NAS 快照或
  用户自己的备份工具承担；
- `publish` / `pull` 是不可变版本的**跨设备分发**，不是双向同步，不传播删除、
  不做冲突合并，也不取代资产库备份。

**这条分工已在 [ADR-0007](decisions/0007-immutable-channel-distribution.md) 中固化**：
aiah 提供 `publish` / `pull` / `versions`，通道就是一个普通目录（U 盘、挂载的
NAS/网盘、或一个 git checkout）；**aiah 自己不实现任何网络传输**。搬字节归
git / rsync / gh / U 盘，aiah 只负责它们都不负责的部分——不可变性、布局、两端
的完整性校验。

这条边界针对**用户资产包的搬运**。`aiah update --check` 是独立的、用户显式触发的
工具版本元数据 GET：它只读取 GitHub latest release，不下载资产包、不上传本地
状态，也不在 TUI 启动时自动发生。

```bash
# 机器 A：发布到通道（不可变；同坐标内容不同即拒绝，无 --force）
aiah publish --package dist/<name>-<version>-<profile>.tar --channel /mnt/usb/aiah

# 用任意方式把 /mnt/usb/aiah 搬到机器 B

# 机器 B：查看、取回、人审、安装
aiah versions --channel /mnt/usb/aiah
aiah pull --channel /mnt/usb/aiah --name <name> --out /tmp/incoming
aiah diff  --package /tmp/incoming/<...>.tar --targets claude,codex,grok
aiah apply --package /tmp/incoming/<...>.tar --targets claude,codex,grok
```

`pull` 与 `apply` 是**两步**，中间那步是人看 diff；合并成一步会把「取回」变成
「部署」。省略 `--version` 取的是**最近发布**的那个，不是版本号最大的——aiah
不解析版本号（ADR-0007 §5）。

`--targets` 为已注册 target id 列表；未实现的 target 不得在 CLI 中假装可用。

`aiah bootstrap` 已作为强制交互编排入口实现：pull 前先检查真实 TTY，取回后复用
TUI Phase C 展示 diff，只有完整输入 `apply` 才安装。它没有 `--yes` 或非交互旁路，
因此没有降低上面的两步边界；详见 ADR-0008。

## 5. 技术方向

首选：

- 核心与 CLI：Go，便于输出跨平台单文件程序；
- 配置：YAML 输入，JSON manifest/lock 输出；
- 包：未压缩 `tar`（成员固定、可复现；压缩留待有需求时再评估）；
- 哈希：SHA-256；
- 可选索引：SQLite；
- 当前不增加 npm/TypeScript launcher；优先发布原生 Release 与系统包管理器；
- 界面：裸 `aiah` 是普通用户的**本地 TUI**入口，`aiah ui` 保留为兼容和高级参数
  入口；它不是 Web UI（[ADR-0006](decisions/0006-tui-as-first-interactive-surface.md)
  已取代 ADR-0003 §5 的 Web UI 规划）——不开监听端口、不引入 TypeScript，
  SSH 与新装机器上单二进制即可用；当前可在一次会话内从显式资产库组装、构建并
  进入 diff/apply；
- TypeScript 只在将来真做 Web UI 或 VS Code 扩展时引入，不迁移 Go Core。

Go 的交叉编译能力不等于所有平台语义已经验证。Windows apply/rollback/hooks
必须单独覆盖权限、rename、文件占用、软链接和脚本格式。

## 6. 与现有项目的关系

项目仓库内的 AI 资产继续随项目 Git 管理。例如现有业务项目的 `CLAUDE.md` 和
项目安装脚本仍是项目事实源。

AI Asset Hub 主要管理：

- 个人全局资产；
- 多项目复用资产；
- 新设备 bootstrap；
- 项目资产的只读盘点和显式导入。

除非用户明确选择，部署器不得覆盖项目仓库中的规则文件。
