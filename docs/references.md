# 外部参考

本项目借鉴公开产品和官方工具规范，但资产格式和实现保持独立。

## PromptHub

- 项目主页：<https://github.com/legeling/PromptHub>
- 许可证：<https://github.com/legeling/PromptHub/blob/main/LICENSE>
- 可借鉴能力：资产分类、本地扫描、Skill 分发、版本比较、项目工作区、备份恢复。
- 不直接采用：PromptHub 源码、内部数据库格式、专有快照格式和实时运行依赖。

## Claude Code

- Memory 与 `CLAUDE.md`：
  <https://docs.anthropic.com/zh-CN/docs/claude-code/memory>
- Claude Code 文档：
  <https://docs.anthropic.com/en/docs/claude-code/>

Claude Code 的加载路径和配置格式可能随版本变化，adapter 必须有版本探测和兼容测试，不能把当前路径永久写死在 Core 中。

## Codex

- Codex 文档：<https://developers.openai.com/codex/>

Codex 的 `AGENTS.md`、Skills、MCP 和 agent 配置由独立 adapter 维护。平台目录属于目标状态，不属于公共资产模型。

## 跨设备配置

- chezmoi：<https://www.chezmoi.io/>
- 模板：<https://www.chezmoi.io/user-guide/templating/>
- 加密：<https://www.chezmoi.io/user-guide/encryption/>
- 密码管理器集成：
  <https://www.chezmoi.io/user-guide/password-managers/>

AI Asset Hub 首版不绑定 chezmoi，但可以把它作为设备配置和 Secret Provider 的集成选项。

## 许可证说明

PromptHub 使用 AGPL-3.0。GNU 对网络交互场景的说明：
<https://www.gnu.org/licenses/gpl-faq.en.html#UnreleasedModsAGPL>

此处仅记录工程边界，不构成法律意见。正式开源前应对源码来源、依赖和发布方式做许可证审查。
