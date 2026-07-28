# AI 资产管理踩坑清单

## 1. 把项目迁移误认为全机迁移

症状：某个项目中 Claude/Codex 都能工作，就宣称迁移完成。

实际还可能缺少：

- 用户级规则；
- 通用 Skills；
- 插件；
- 全局 MCP 模板；
- 设备 profile；
- 跨设备安装包。

排查时必须分别报告项目、用户全局和设备迁移三层状态。

## 2. 在已有 `CLAUDE.md` 的项目生成 `AGENTS.md`

Codex 原生规则文件可能优先于 fallback。新增同级 `AGENTS.md` 会让原本复用
`CLAUDE.md` 的设计失效，形成两份事实源。

处理：

- 扫描现有规则和用户 fallback；
- 已配置 `CLAUDE.md` fallback 时，以 `CLAUDE.md` 为单一事实源，不运行会另建
  `AGENTS.md` 的初始化；
- 两个文件都没有时，先从仓库事实整理并人工审阅 `CLAUDE.md`，再验证 fallback；
- apply 前显示规则加载变化；
- 默认不覆盖项目规则；
- 只有显式选择“拆分规则”时才生成两端文件。

若项目需要支持没有 fallback 配置的其他 Codex 用户，应改为显式的双端生成策略并用
CI 检查漂移；不要手工维护两份近似内容。

aiah 侧的落地设计（三端重定向表、为什么不自动改名、shadowing 的两级严重度）见
[资产模型 §4.1](../asset-model.md)。

## 3. 把同一个 Skill 原样复制到所有工具

Claude/Codex 对 frontmatter、隐式调用控制和 agent 元数据的支持不同。文件复制成功不代表工具能正确发现和执行。

处理：

- 公共 `SKILL.md` + 平台 sidecar；
- 构建时转换；
- 在目标工具中做发现和触发验证。

## 4. 将 Commands、Skills 和 SOP 重复维护

同一工作流同时存在 `/commands/foo.md` 和 `skills/foo/SKILL.md`，内容会逐渐漂移。

处理：

- 可复用 SOP 以 Skill 为事实源；
- 旧 command 归档或由 adapter 生成；
- 不手工双写。

## 5. 把原生 Memory 当成可移植知识

会话记录、自动记忆、SQLite 状态和整理后的长期知识不是同一类资产。直接导入另一工具会造成上下文污染、隐私泄露和重复旧结论。

处理：

- 只迁移整理后的偏好、决策和经验；
- 项目历史放显式调用 Skill；
- 默认 `allow_implicit_invocation: false`；
- 原生历史只做单独加密归档。

## 6. 把真实 MCP 密钥放进工作区或备份

很多 MCP 配置把 env/header 与普通配置存在一起，全量 snapshot 很可能连密钥一起带走。

处理：

- 资产库只存 `${secret:...}`；
- 导出后做 secret scan；
- 日志和 diff 脱敏；
- Secret Provider 只在目标设备 apply 阶段解析。

## 7. 把 `.gz`、`.zip` 当成加密

压缩只改变编码，不提供保密性。

处理：

- 普通资产包必须保证无密钥；
- 含敏感资料的备份使用 age/GPG 等明确加密；
- 恢复演练必须验证解密和密钥轮换。

## 8. 把 SQLite 放进网盘实时同步

多设备会同时同步 DB、WAL 和 SHM，容易出现锁冲突、覆盖和损坏。

处理：

- SQLite 只做可重建索引；
- 网盘只传不可变版本包；
- 多端编辑由 Git 或显式 push/pull 解决。

## 9. 默认使用 symlink

不同工具对目录和文件软链接的发现行为可能不同；网盘、Windows 和容器挂载又会引入额外差异。

处理：

- 首版默认 copy；
- symlink 是经目标工具验证后的 opt-in；
- manifest 记录实际安装模式。

## 10. 覆盖已有用户配置

直接重写 `~/.codex/config.toml`、Claude settings 或项目 MCP 会丢失用户设置和真实凭据。

处理：

- sidecar 与 harness 原生配置分开所有权；
- 原生配置不存在时才默认 create；
- 已存在时默认只报告，不修改；
- identical 语义保持原字节并零写入；
- 未来显式 merge 必须保留 MCP 容器外原始字节/注释，并在修改前备份；
- 备份可能含原配置敏感值，必须私有保存且禁止同步到不可信介质。

## 11. 把路径写死在 Core

Claude/Codex 目录和格式会随版本、操作系统和安装方式变化。

处理：

- 平台路径属于 adapter；
- 支持用户覆盖；
- 记录客户端版本；
- 用临时 HOME 做兼容测试。

## 12. 导入时自动执行脚本

Skill、Hook、MCP server 都可能执行本机命令。来源可信不代表内容始终安全。

处理：

- scan/import 只读；
- 可执行资产单独标记；
- 安装前展示权限和命令；
- 默认不运行第三方脚本。

## 13. 用 mutable `latest` 做设备恢复

同一包名内容变化后无法重现旧设备状态，也无法证明回滚目标。

处理：

- 包名带版本；
- manifest 固定来源 commit/tag；
- 每个文件记录 SHA-256；
- 发布后不覆盖同版本产物。

## 14. 把生成结果提交为另一份事实源

公共资产和 Claude/Codex 生成目录同时进入 Git 后，很容易只改其中一份。

处理：

- 通用资产和 adapter 入库；
- 目标目录默认 gitignored；
- 如需提交生成结果，CI 必须验证可重建且无 diff。

## 15. 自动扫描越过项目和个人边界

把客户项目 memory 或内部 MCP 配置纳入个人公共包，会产生严重信息泄露。

处理：

- 每个资产声明 scope 和 sensitivity；
- 项目资产默认只读盘点；
- 跨 scope 导入要求显式确认；
- build 按 Profile 做 allowlist，而不是“扫描到什么打包什么”。

## 16. 没有真实客户端验收

仅验证文件存在、JSON/TOML 可解析还不够。工具可能忽略目录、拒绝 frontmatter、要求 hook trust，或因版本变化采用其他路径。

最低验收：

1. 客户端能列出 Skill/MCP；
2. 显式 Skill 调用成功；
3. rules 实际进入上下文；
4. hook 在安全夹具上触发；
5. apply 幂等；
6. rollback 恢复原状态。

## 17. 把 MCP “已连接”当成工具可用

`mcp list` 成功只证明 server 能启动或完成握手，不保证工具调用所需的运行时资产齐全。
一次双端验收中，Playwright MCP 在两端都显示 connected，但第一次导航仍因缺少
`chrome-for-testing` 失败。

处理：

- 至少实际调用一个只读、无副作用的代表工具；
- 浏览器 MCP 用 `about:blank` 验证；
- 数据库 MCP 用列 schema 或 `SELECT` 常量验证；
- HTTP MCP 用 health 端点验证；
- 浏览器二进制、Python venv、Node cache 等可重建设备依赖只写入 bootstrap/lock，
  不塞进 AI 资产包；
- 区分持续配置错误与一次性连接中断，失败后用更小的只读调用复测。

## 18. 把账户额度或客户端更新失败误判为迁移失败

Skill 的真实调用依赖模型账户可用。会话额度耗尽时，即使 Skill 已被正确发现，调用也会在
模型执行前失败；CLI doctor 的自动更新权限错误也不等于项目配置加载失败。

处理：

- 分开记录“磁盘安装”“客户端发现”“真实调用”三层状态；
- 保存原始错误类别，不把外部额度阻塞写成 adapter 失败；
- 额度恢复后补跑最小 Skill 和 hook 用例；
- 客户端升级权限作为独立设备维护项处理。

## 19. 把「Grok 能读 Claude/共享 Skill」当成 aiah 已支持 Grok

Grok Build 默认会扫描 `.agents/skills` 与（可配置的）Claude skills，因此共享层看起来
“已经兼容”。但 aiah Phase 0 并不扫描 `~/.grok`，也没有 Grok adapter；用户级
`~/.grok/skills`、`config.toml` MCP、hooks、personas 等仍在盘点视野外。

处理：

- 区分 T0 共享落点可用 与 Target 一等支持（见 ADR-0002）；
- 迁移统计只数权威逻辑 Asset，不把 compat `loadedBy` 当成第二事实源；
- 未完成 Probe/Adapter 前不宣称 Grok 全量迁移；
- 个人通用 Skill 优先放 `~/.agents/skills`，需要 Grok 专属能力时再进 `.grok`。
