# scripts/

| 脚本 | 用途 | 谁调用 |
|---|---|---|
| `bootstrap-dev.sh` | 新设备无 root 安装仓库锁定的 Go 与 golangci-lint | 人（显式执行） |
| `dev-doctor.sh` | 只读检查工具链版本与本地依赖 | 人、`check-local.sh` |
| `test-dev-doctor.sh` | fake PATH 验证 doctor 禁用 Go 自动下载 | `check-local.sh` |
| `check-local.sh` | 一次运行完整本地门禁 | 人 |
| `generate-third-party-licenses.sh` | 生成/校验发布所带第三方许可证正文 | 人、`check-local.sh` |
| `test-release-checksums.sh` | 恶意同名前缀不能冒充许可文件校验项 | `check-local.sh` |
| `install.sh` | 校验 Release SHA256 后原子安装 Linux/macOS 二进制 | 人、`curl \| sh` |
| `install.ps1` | 校验 Release SHA256 后原子安装 Windows 二进制 | 人、PowerShell |
| `test-install.sh` | fake 下载验证 Unix 安装器校验、幂等与旧版本保护 | `check-local.sh`、`ci.yml` |
| `test-install.ps1` | 验证 PowerShell 安装器校验、幂等与原子替换 | `check-local.sh`（有 pwsh 时）、`ci.yml` |
| `_sha256.sh` | Linux/macOS 共用 SHA256 函数 | 被 bootstrap、release、install 脚本 source |
| `check-release-checksums.sh` | 用主机可用的 SHA256 工具复验发布产物 | 人 |
| `_stamp.sh` | **版本戳单一事实源**：算 VERSION / COMMIT / DATE 与 ldflags | 被 `build.sh`、`release-build.sh` source |
| `build.sh` | 本机构建带版本戳的 `build/aiah` | 人、`demo-apply-scan-loop.sh` |
| `release-build.sh` | 六平台发布产物 + 许可材料 + `SHA256SUMS` + 版本自检 | 人（本地预演）、`release.yml` |
| `check-gofmt.sh` | gofmt 门禁 | 人、`ci.yml`、`release.yml` |
| `demo-apply-scan-loop.sh` | 假 HOME 闭环：build → apply → scan → rollback | 人、`ci.yml` |

两条约定：

1. **CI 只调用这些脚本，不写 inline 步骤**。workflow YAML 没法本地跑，脚本可以；
   门禁必须能在本机复现，见 [development.md §2.3](../docs/development.md)。
2. **版本戳只算一次**。要改 VERSION/COMMIT/DATE 的推导规则就改 `_stamp.sh`，
   别在调用方各算一遍——那正是本地构建与发布构建开始分叉的方式。

新设备搭建、PATH 约束与版本升级流程见
[开发环境搭建 SOP](../docs/runbooks/development-environment.md)。
