# PromptHub 调研与采用边界

- 调研日期：2026-07-23
- 上游：<https://github.com/legeling/PromptHub>
- 调研时稳定版：v0.5.9
- 许可证：AGPL-3.0

## 结论

PromptHub 对本项目有较高参考价值，适合参考或复用其产品概念：

- Prompt、Skill、Rules、MCP、Plugin 工作区；
- 本地扫描和项目工作区；
- Skill copy/symlink 分发；
- 版本历史、diff、回滚；
- WebDAV/S3/自托管快照；
- CLI 导入导出。

但它不适合作为 AI Asset Hub 的唯一事实源，也不应直接承担本机原生 memory、密钥和跨工具格式转换。

推荐关系：

```text
AI Asset Hub 纯文本资产
        │
        ├── 自有 CLI / adapters：构建和部署
        └── PromptHub 或未来 UI：浏览、搜索、编辑
```

## 当前源码策略

当前 Phase 0/1 不需要 clone PromptHub，也不把它放入本仓库、Git submodule、
vendor 目录或构建依赖。AI Asset Hub 的 Core、schema 和测试继续独立实现。

如果 Phase 4 的 UI 或兼容性研究确实需要观察上游行为，只允许：

1. clone 到 AI Asset Hub 仓库外的临时目录；
2. 固定并记录上游 commit；
3. 只记录公开行为、输入输出和独立需求；
4. 不复制源码、内部数据库 schema 或受版权保护的实现；
5. 研究结束后移除临时副本。

这项限制不妨碍阅读公开文档、issue、release notes 或运行脱敏测试，但
PromptHub 源码研究不是进入 Phase 1 的前置条件。

## 有价值的设计

### 本地优先

PromptHub 默认把数据保留在本机，并支持桌面端、CLI 和自托管 Web。这证明“个人 AI 资产工作台”存在明确需求。

### 多平台资产发现

它能扫描 `.claude/skills`、`.agents/skills` 等目录，并把 Skill 分发到多个 AI 工具。AI Asset Hub 应保留多平台扫描能力，但扫描结果只能作为导入候选，不能直接成为事实源。

### 版本与冲突保护

保存历史、来源哈希、远端更新检测和覆盖前确认值得借鉴。AI Asset Hub 应把这些信息写入开放的 manifest/lock，而不是只存在数据库中。

## 采用限制

### Codex 路径会随版本变化

PromptHub 的平台映射曾把 Codex Skills 目标设为 `~/.codex/skills`；本机当前可移植个人 Skill 使用 `~/.agents/skills`，项目 Skill 使用 `.agents/skills`。

这不宜简单判定为上游错误，但说明：

- 平台路径必须可覆盖；
- adapter 需要版本探测；
- 测试不能只验证文件复制成功，还要验证目标工具确实发现资产。

### Rules 可能改变加载优先级

现有业务项目当前只跟踪根 `CLAUDE.md`，并让 Codex 通过
`project_doc_fallback_filenames = ["CLAUDE.md"]` 复用它。

如果管理工具又在同级生成 `AGENTS.md`，Codex 会优先加载原生规则文件，原有 fallback 可能被遮蔽。因此 PromptHub 或 AI Asset Hub 都不得默认向已有项目写规则文件。

### MCP 快照可能携带密钥

PromptHub 的 MCP 类型允许保存 env 和 headers，完整 workspace/sync snapshot 又包含 MCP library。其 `.phub.gz` 全量备份是压缩容器，不能仅凭扩展名视为加密保险箱。

结论：

- PromptHub 中只存 MCP 模板和变量引用；
- 启用同步前必须解包检查一次备份；
- 真实 Token 留在密码管理器、系统钥匙串或设备本地环境；
- “私密文件夹加密”不能自动等同于“整个 workspace/MCP 快照都加密”。

相关上游文件：

- <https://github.com/legeling/PromptHub/blob/main/packages/shared/types/mcp.ts>
- <https://github.com/legeling/PromptHub/blob/main/packages/shared/types/sync.ts>
- <https://github.com/legeling/PromptHub/blob/main/apps/desktop/src/renderer/services/database-backup.ts>

### 原生 Memory 不可直接统一

PromptHub 没有一个能无损覆盖 Claude Code、Codex 原生会话和自动记忆的公共模型。可迁移的应是整理后的规则、偏好、决策和经验；会话数据库、缓存和历史索引仍属于工具私有状态。

### Skill 不是完全无差别复制

现有业务项目的共享 Skills 在 Claude 端使用 `disable-model-invocation: true`，在 Codex 端移除此字段，并通过 `agents/openai.yaml` 的
`allow_implicit_invocation: false` 保持“仅显式调用”语义。

普通 copy/symlink 无法完成这种转换，因此需要 adapter。

### AGPL 边界

如果复制 PromptHub 源码形成衍生作品，应继续遵守 AGPL-3.0。修改后通过网络向用户提供服务时，需要向这些用户提供相应源码。

如果 AI Asset Hub 希望未来选择 MIT、Apache-2.0 等其他许可证，应保持需求和实现独立，不复制 PromptHub 源码。正式发布前仍需进行许可证审查。

参考：

- <https://github.com/legeling/PromptHub/blob/main/LICENSE>
- <https://www.gnu.org/licenses/gpl-faq.en.html#UnreleasedModsAGPL>

## 建议的 PromptHub 试点

如需继续验证 PromptHub：

1. 只导入 2–3 个不含敏感信息的 Skill。
2. 首轮使用 copy，不用 symlink。
3. 覆盖 Codex 目标目录并验证实际发现结果。
4. 不允许它覆盖业务项目根规则。
5. 导出 `.phub.gz` 后解包检查是否包含 env/header 值。
6. 在临时 HOME 中测试恢复。
7. 验证完成后再开启 WebDAV、S3 或自托管备份。
