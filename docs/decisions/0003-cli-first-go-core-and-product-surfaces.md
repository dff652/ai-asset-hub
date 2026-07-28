# ADR-0003：CLI-first、Go Core 与产品界面演进

- 状态：Accepted
- 日期：2026-07-24
- 关联：[ADR-0001](0001-file-first-adapter-distribution.md)、
  [ADR-0002](0002-multi-target-capability-adapters.md)

## 背景

Phase 0–2 已形成完整 CLI 链路：

```text
scan → validate → build → diff/apply → rollback → scan
```

需要决定三个相互关联、但不应混为一谈的问题：

1. 核心实现是否从 Go 转为 TypeScript；
2. 是否为了 `npm i -g` / `npx` 增加 JavaScript/TypeScript launcher；
3. 产品长期保持纯 CLI，还是加入 Web UI、编辑器扩展或云服务。

TypeScript 的优势是真实的：目标用户通常熟悉 Node/npm，MCP 与前端生态成熟，
未来 VS Code 扩展和 Web UI 可以共享语言与类型。但 AI Asset Hub 当前的主要复杂度
不是 MCP SDK 或界面，而是路径规范化、软链接、文件权限、确定性打包、原子写入、
备份和 rollback。现有 Go 实现与测试已经覆盖这些边界。

安装体验也不等于实现语言。Go CLI 可以发布预编译单文件程序，用户无需安装 Go；
若未来确有 npm 分发需求，也可以用平台包封装原生二进制，而不必重写 Core。

## 决策

### 1. 产品采用 CLI-first，而不是 CLI-only

CLI 是长期稳定的产品内核、自动化接口和 Agent 接口。所有命令继续提供确定性的
JSON 输出，供人、脚本、CI、编辑器扩展和未来 UI 调用。

界面按实际痛点增加：

- 当前：CLI；
- Phase 3.5：可选、本地、只读 Web UI；
- Phase 4：在稳定 Core 上增加受控写操作；
- 云服务：只有跨设备或团队需求得到验证后才评估。

### 2. Core 与 CLI 继续使用 Go

不进行 TypeScript 全量重写。以下能力只能在 Go Core 中实现一次：

- inventory / validate / build；
- adapter 编译；
- diff / apply / rollback；
- 路径、权限、软链接和事务安全；
- 包格式、哈希与部署记录。

UI 或扩展不得复制上述业务规则。

### 3. 当前不加入 npm launcher

首选分发顺序：

1. GitHub Releases 多平台二进制；
2. Homebrew（macOS/Linux）；
3. Scoop 或 winget（Windows）；
4. 可审查的安装脚本；
5. `go install` 仅作为开发者安装方式。

只有用户验证表明 `npx` 是采用阻塞点时，才增加很薄的 JavaScript launcher。
即使增加，也只是选择和启动平台二进制，不把 TypeScript 引入 Core。

### 4. 跨平台分为“可构建”和“语义已验证”

Go 支持按 `GOOS` / `GOARCH` 生成不同平台二进制，但不能因此直接宣称所有功能
已跨平台验证。

支持顺序：

1. Linux amd64/arm64、macOS amd64/arm64：完整 CLI；
2. Windows amd64：先验证 scan/validate/build/diff；
3. Windows apply/rollback/hooks：完成权限、rename、文件占用、软链接和脚本格式
   的专门测试后再标记完整支持。

Windows 的 `chmod`、shebang 和用户配置根语义与 Unix 不同，必须显式建模，不能用
“编译通过”代替行为验收。

### 5. Web UI 先本地只读，再开放受控写入

> **已被 [ADR-0006](0006-tui-as-first-interactive-surface.md) 取代（2026-07-28）**：
> 第一个交互界面最终是本地 **TUI** 而不是本地 Web UI。本节其余内容保留为历史记录，
> 其中「先只读、写操作须先看 diff、界面不得复制部署逻辑」等原则在 ADR-0006 中延续。

第一版 UI 形态是：

```text
aiah ui
  └── 127.0.0.1:<port>
        └── 调用同一 Go Core / 稳定 JSON 契约
```

只读 UI 可以提供：

- Inventory 搜索、过滤和预览；
- manifest / package 查看；
- diff 可视化；
- findings、安全告警和部署历史。

后续写操作必须满足：

- 先显示 diff，再人工确认；
- 调用同一 Go apply/rollback；
- 返回并展示 `backupId`；
- UI 不直接读写 `.claude`、`.codex`、`.grok` 等目标目录。

TypeScript 只在真正建设 Web UI 或 VS Code 扩展时引入。共享的是 JSON Schema 和
命令契约，不以共享语言为由迁移 Core。

## 讨论过但不采用的方案

### 立即将 Core 重写为 TypeScript

会重做已经验证的文件系统和事务语义，增加回归面；MCP 与 UI 的未来收益不足以抵消
当前迁移成本。

### 现在增加 npm launcher

能改善 Node 用户的安装心智，但会提前维护 npm 与原生 Release 两套发布链路、
平台包选择和供应链边界。当前尚无用户证据证明这是采用阻塞点。

### 永久只做 CLI

不采用。CLI 适合自动化，但当资产、设备和版本增长后，搜索、预览、diff 与部署历史
更适合可视化界面。

### 先建设云端管理台

不采用。它会过早引入账户、鉴权、加密、同步冲突、托管成本和隐私责任，偏离当前
单机可验证、文件优先的主线。

## 影响

正面影响：

- Core 只有一份安全实现；
- 当前 Go 投资和回归测试得到保留；
- 用户分发不依赖 Go 工具链；
- UI、扩展和云服务可以独立演进；
- 不因前端选型改变资产格式。

代价：

- UI 与 Core 之间需要维护稳定 JSON/Schema 契约；
- 若增加 TypeScript UI，仓库会成为多语言项目；
- Windows 完整支持需要单独的行为测试，而不只是交叉编译；
- npm 用户暂时使用 Release 或包管理器安装。

## 启动 UI 的门槛

同时满足以下条件后进入 Phase 3.5：

1. apply/rollback 安全问题清零；
2. CLI schema 和兼容策略稳定；
3. 跨设备不可变包发布/拉取链路跑通；
4. 用户主要问题变为“难搜索、难预览、难比较”，而非基础能力缺失；
5. UI 可以只调用 Core 契约，不复制部署逻辑。
