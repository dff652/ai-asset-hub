# 安全与隐私

发现漏洞时不要公开披露敏感细节；报告方式见仓库根
[SECURITY.md](../SECURITY.md)。

## 1. 威胁边界

AI 资产可能包含：

- API Key、Token、Cookie 和私有 MCP headers；
- 客户、项目和个人信息；
- 可执行脚本及 hooks；
- 会影响 AI 权限和行为的规则；
- 来自第三方仓库的 Skill。

因此资产安装应按“代码部署”而不是“复制文档”处理。

## 2. 密钥策略

资产包中只允许密钥引用：

```text
${ENV:OPENAI_API_KEY}
${secret:personal/openai}
```

支持的 Secret Provider 可逐步加入：

- 系统环境变量；✅
- macOS Keychain；
- Windows Credential Manager；
- `pass`；✅
- 1Password CLI；
- age 加密的本机文件。

当前只在 MCP 模板 `env` 的完整字段值中解析引用：

- `${ENV:NAME}` / `${env:NAME}`：读取非空环境变量 `NAME`；
- `${secret:path}`：执行 `pass show -- path`，只取第一行作为值。

解析发生在 apply 的计划阶段（`diff` / `--dry-run` 也走同一路径）。provider
缺失、引用不存在或结果为空时整单 fail-closed，任何 sidecar、native config 或
其它资产都不写。sidecar 始终保留引用；只有设备本地的一次性 native config
bootstrap 含解析值。后续 inventory 会把这个 native config 识别为
`suspected_secret` 并排除，防止真实值回流资产包。

默认命令输出、diff、日志、manifest、journal、backup 元数据和 crash report 都
不得包含解析值。

## 3. 构建安全

`validate` 至少检查：

- 常见私钥和 Token 特征；
- `.env`、credential、cookie 文件；
- 绝对路径和用户名泄露；
- 软链接逃逸；
- 超出资产根目录的引用；
- 重复目标和规则遮蔽；
- 未声明的可执行文件；
- 超大文件和二进制文件。

检测到疑似密钥时默认构建失败，不能只打印 warning。

## 4. 安装安全

- 默认 copy，不默认 symlink。
- 安装前输出目标清单和 diff（`aiah diff` / `aiah apply --dry-run`）。
- 不自动执行导入包中的脚本；hook 只落盘。
- Hook、MCP 和可执行 Skill 需要单独确认（真机流程见 runbooks/real-home-dry-run.md）。
- Script hook：必须 shebang；安装 `0755`；禁止原始密钥与二进制。
- 覆盖文件前生成带时间和备份 ID 的备份。
- 备份 payload 使用私有权限；记录被覆盖文件的原 mode，rollback 必须恢复。
- MCP sidecar 由 aiah 管理；原生配置属于用户/harness，不因包内存在 MCP asset
  自动取得整文件所有权。
- 新建 MCP 原生配置默认 `0600`，且只做一次性 bootstrap；创建后也按已有用户文件
  处理，后续包版本只报告、不修改。
- MCP 已有同名 server 只有内容相同才幂等，内容不同令整个 apply fail-closed。
- 内容相同时必须保持原字节，不 stage、不写盘、不生成 backup。
- 精确 rollback 的 backup 不做不可逆脱敏，可能包含原配置已有敏感值；必须保持
  私有权限，且不得提交 Git、写入日志或同步到不可信介质。
- 只允许写入 adapter 声明的精确路径。
- 不删除未知文件，除非用户显式启用 exact 模式。

2C.3.1/2C.3.2 已实现上述 create-only 契约收口，并于 2026-07-25 通过严格复审
（[报告](reviews/2026-07-25-mcp-create-only-strict-review.md)，六条门槛逐条实证）。
含 MCP asset 的包解除「仅 dry-run」限制，但长期流程不变：先假 HOME 闭环、再
`--dry-run` 检查 diff、人工确认后 apply，见 ADR-0004。

复审发现的 fail-closed 过宽（P6）尚未处理：已有原生配置是软链、0 字节/非法 JSON，
或把等价内容写成 `"args": []` 时，当前会让**整单 apply 失败**而不是跳过 MCP。
方向是拒绝写入，不构成安全风险，但会阻断无关资产的安装。

## 5. AI 只读接入

`aiah mcp` 的权限边界锚在行为测试而不是工具名清单：

- 注册表中的每个工具必须通过 HOME、project、资产库、通道和包目录前后逐字节
  不变测试；
- 每个工具通过 MCP annotations 声明只读、非破坏、幂等和封闭世界；
- 参数 schema 禁止未知字段，server 端也用 `DisallowUnknownFields` 二次执行；
- 不注册 build、资产库写操作、publish/pull、apply/rollback，也没有
  `--allow-write` server 开关；
- 状态报告可以包含用户提供的本地路径和非敏感恢复点 id，但不得返回 secret 值、
  auth/session/cache 内容或设备凭据。

客户端显示 `Connected` 只证明协议握手；模型实际调用、账户授权和零写入对账是独立
证据，不能互相替代。可重复步骤见
[MCP 客户端接入 runbook](runbooks/mcp-client-acceptance.md)。

## 6. 供应链

第三方 Skill 必须记录：

- 来源 URL；
- commit/tag；
- 导入时间；
- 内容 SHA-256；
- 本地修改状态；
- 审核结果。

更新第三方资产时先展示 diff，不允许静默跟随远端最新版本。

工具自身的 Release 检查同样不静默跟随：`aiah update --check` 和 TUI `v` → `c`
只读取 GitHub latest release 元数据，不下载、不替换二进制。返回的升级命令绑定
精确 tag；真正安装仍由用户显式执行，并继续经过安装器的 SHA256 与原子替换门禁。
但 tag URL 不能单独证明目标版本已绑定：`v0.1.4` / `v0.1.5` 的生成命令缺少显式
`AIAH_VERSION`，配合 staged installer pin 会停留在旧版。`v0.1.6` Release 已公开
bridge workaround，并完成 legacy no-op 与显式版本升级验收；具体命令见
[上手指南](getting-started.md)。`v0.1.6` 二进制已让后续升级命令显式绑定
`AIAH_VERSION` 并加入精确字符串回归；该修复不追溯改变旧二进制。下一版本仍须执行
程序实际生成的命令，并核对安装后的版本、commit 和 SHA256。

## 7. 许可证

本项目许可证已定为 **Apache-2.0**（2026-07-25 决策）：仓库根 `LICENSE` 为官方
协议正文，版权署名在 `NOTICE`，第三方依赖清单在
[docs/licenses/third-party.md](licenses/third-party.md)。选择 Apache-2.0 而非
MIT 是为了带上显式专利授权；依赖侧全部为 MIT / Apache-2.0 / BSD 系宽松协议，
无 copyleft 传染。

PromptHub 使用 AGPL-3.0。本项目可以研究其公开行为、数据分类和交互思路，但以下
约束是选择非 AGPL 协议的**前提**，长期有效：

- 不复制 PromptHub 源码；
- 不移植受版权保护的实现细节；
- 记录独立需求和设计过程；
- 对第三方依赖做许可证清单（已完成，见上）；
- 在正式开源前完成一次许可证审查（已完成一轮：依赖协议逐个核对模块自带
  `LICENSE`，结论见清单）。

新增依赖时同步更新 `NOTICE` 与第三方清单；引入 GPL / AGPL / LGPL 依赖前必须先
评估协议兼容性，不得直接合入。
