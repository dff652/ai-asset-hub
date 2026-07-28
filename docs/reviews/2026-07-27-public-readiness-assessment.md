# Public readiness、仓库身份与 TS 边界评估

- 日期：2026-07-27
- 状态：**所有者已确认转 public、使用 `dff652` 和干净公开历史；仍不授权 push、
  tag、合并默认分支或修改仓库可见性**
- 范围：当前 checkout、GitHub 仓库与 `v0.1.0` Release
- 关联：[ADR-0003](../decisions/0003-cli-first-go-core-and-product-surfaces.md)、
  [MVP 路线图 D8](../roadmap.md)、[发布 runbook](../runbooks/release.md)

## 1. 结论

AI Asset Hub 的核心 CLI MVP 已达到**自用和技术预览可用**：本地完整门禁通过，
scan / validate / build / diff / apply / rollback / doctor、Phase 2C 安全契约和
TUI Phase A 均已实现并有闭环验证。

当前**不应直接把仓库切为 public**。剩余问题主要是仓库身份、公开内容、默认分支、
Release 一致性和公开仓库治理，不是核心代码不可运行。

“可用”不等于“全功能完成”。Phase 3 跨设备分发、TUI Phase B/C、`aiah mcp`、
安装渠道、Windows 写入行为验收、云服务和编辑器扩展仍未完成。

## 2. 仓库命名空间：已选择 `dff652`

Go module path 是公开源码的稳定身份，不只是一个注释。评估时曾存在不一致：

| 项目 | 评估时 |
|---|---|
| 实际 GitHub 仓库 | `github.com/dff652/ai-asset-hub` |
| `go.mod` 与内部 import | 使用了不存在的 `ilabel` namespace |

公开后，这会影响 `go install` 路径、pkg.go.dev、源码链接和其他 Go 项目的 import。
所有者于 2026-07-27 确认使用 `github.com/dff652/ai-asset-hub`；当前工作树已统一
`go.mod`、内部 import 和构建版本注入路径。发布前验收必须确认旧 namespace
字符串归零。

## 3. “TS” 的两个含义

### 3.1 TypeScript

本项目没有进行 TypeScript 重构，也没有“TypeScript 全功能实现”的目标。
ADR-0003 已决定：

- Core、CLI 和当前 TUI 保持 Go；
- 当前不增加 npm / TypeScript launcher；
- 只有真正建设 Web UI 或 VS Code 扩展时才引入 TypeScript；
- 即使以后引入 TypeScript UI，也不迁移或复制 Go Core 的部署逻辑。

因此准确表述是：**项目主动选择不做 TypeScript 全量重写**，而不是 TypeScript
重构尚未完成。

### 3.2 内部业务项目

如果 “TS” 指内部业务项目，则当前状态是**项目级双端迁移基本完成，但不是全部
完成**：

- Claude / Codex 双端配置、安装器和共享 Skills 已实现；
- Codex 运行态已验收；
- 一项 Claude 侧 Skill 运行态检查尚待补测；
- 跨设备资产包和由 AI Asset Hub 接管的完整链路尚未完成。

该项目的事实源仍由其自身 Git 和安装器管理；AI Asset Hub 负责后续通用
资产盘点、打包、部署和分发，不取代其项目事实源。设备与项目迁移台账保留在私有
仓库，不进入干净 public export。

## 4. AI Asset Hub 功能完成边界

### 已完成

- Phase 0–1：资产盘点、validate、确定性 build；
- Phase 2A/2B/2C：多 target diff / apply / rollback、安全写入与闭环扫描；
- MCP 原生配置 create-only 所有权契约；
- `aiah doctor` 只读部署状态自查；
- TUI Phase A：只读资产树、详情、过滤、findings 分诊和重扫；
- `v0.1.0` 六平台二进制、许可证材料和 SHA256 发布闭环。

### 尚未完成

- Phase 3 跨设备发布 / 拉取；
- TUI Phase B manifest 组装和 Phase C diff / apply 确认；
- `aiah mcp` 只读 server；
- `install.sh`、`install.ps1`、Homebrew、Scoop / winget；
- Windows apply / rollback / hooks 原生行为验收；
- 受控写入 UI、云服务和编辑器扩展。

产品定位应写成：**核心 CLI MVP 完成、技术预览可用；完整产品路线尚未全部实现**。

## 5. 转 public 前的发布收口

以下项目完成前，不执行仓库可见性切换。

### P0：所有者决策

- [x] 拍板 D8：仓库目标转 public；
- [x] canonical module path 使用 `github.com/dff652/ai-asset-hub`；
- [x] 采用干净 public history，不公开原 Git 作者邮箱和历史对象；
- [x] 设备、内部项目和会话迁移台账不进入 public export。

### P0：仓库内容和版本一致性

- [x] 按最终命名空间统一 GitHub 地址、`go.mod`、内部 import 和构建版本注入路径；
- [x] 用 `export-ignore` 将设备迁移台账和会话交接排除出干净 public export；
- [x] 所有者 push 当前 private `dev` 并等待同一提交的远端 CI 通过；
- [x] 按 [Public 发布 runbook](../runbooks/public-launch.md)从已验证提交导出干净
      工作树，建立新的单提交 `main` / `dev`；不把落后 41 个提交的旧 `main` 或旧
      refs 推入 public；
- [x] 在 README 明确 `aiah doctor` 尚未进入
      `v0.1.0`，并确认 public `v0.1.1` 已包含该命令。

### P0：公开项目入口和安全治理

- [x] 增加 GitHub 可识别的根目录 `SECURITY.md`，写明漏洞报告渠道；
- [x] README 增加匿名下载、SHA256 校验、手动安装、支持平台和 Windows 限制；
- [x] 给 `main` / `dev` 配置分支保护和 8 项必需 CI；
- [x] 补 GitHub description、topics，并检查 README、LICENSE、NOTICE 和 Release
      链接可从默认分支访问。
- [x] 启用 GitHub private vulnerability reporting。

### P1：可在 public 后紧接完成

- [x] 匿名环境验证 clone、Release 下载、SHA256 和 Linux amd64 启动；macOS
      产物已构建并校验，原生行为仍按跨平台边界单独验收；
- [ ] 开启 GitHub secret scanning、Dependabot 等公开仓库安全能力；
- [x] 实现并验证 Linux amd64 `install.sh`；其他平台在原生验收后再开放安装入口；
- [ ] 增加 CONTRIBUTING、Issue / PR 模板；Code of Conduct 按社区开放程度决定。

安装脚本是 public 分发的后续能力，不是切换可见性的先决条件；但 README 中必须先
有可执行、可校验的手动安装路径。

## 6. 建议执行顺序与验收

1. 所有者拍板命名空间、历史隐私和 D8；✅
2. 做公开内容脱敏、module path 与公开文档修改；✅
3. 本地运行 `./scripts/check-local.sh`；✅
4. 所有者 push private `dev`，等待 CI 全绿；
5. 从通过验证的提交建立干净 public `main` / `dev`；
6. 发布包含最新用户可见功能的 `v0.1.1` 并验收线上产物；
7. 配置分支保护和仓库元数据；
8. 切换 public；
9. 在未登录环境完成匿名下载和启动验收；
10. 开始安装脚本和包管理器渠道。

push、tag、合并 `main` 和修改 visibility 始终是所有者保留动作，需要针对具体操作
明确授权。
