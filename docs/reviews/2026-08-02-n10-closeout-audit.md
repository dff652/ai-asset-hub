# N10 迁移准备收尾审计

- 日期：2026-08-02
- 发布：public `v0.1.10` @ `72b05c2e9ee0`
- Release：https://github.com/dff652/ai-asset-hub/releases/tag/v0.1.10
- 最终树：`origin/main` 与 `origin/dev` 文件树相同（squash 提交不同）

| 标准 | 证据 | 结论 |
|---|---|---|
| 真实资产库三状态（missing/mismatch/recorded） | 真库 temp-copy；live 哈希不变；redacted captures | 通过 |
| N10.2 TUI 合入 dev + CI | PR #44 `cd2663a`；CI 30707660989 / 30707659298 | 通过 |
| N10.3 只读 MCP 合入 dev + CI | PR #46 `5c54b7c`；CI 30707838858 / 30707836897 | 通过 |
| 正式 Release main/tag | PR #47 → main `72b05c2`；tag v0.1.10；Release 30708101913 | 通过 |
| 严格升级 v0.1.9→v0.1.10 | 程序 upgradeCommand 逐字匹配；SHA 一致；幂等复装 | 通过 |
| 安装包 CLI/TUI/MCP | pkg-accept 捕获 | 通过 |
| installer pin + 用户文档 | PR #48；`DEFAULT_AIAH_VERSION=0.1.10` | 通过 |
| main/dev 文件树一致 | `git diff origin/main origin/dev` 空；PR #49/#50 收口 | 通过 |
| N10.4 延期 | 无 auto recorder / 无 MCP 写 / 无后台同步 | 明确延期 |

## 不扩大范围

未实现：N10.4、后台同步、自动上传、云存储/NAS/Git 客户端、MCP 写操作、N8.3/N8.4、新 AI target。
