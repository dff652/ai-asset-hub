# scripts/

| 脚本 | 用途 | 谁调用 |
|---|---|---|
| `bootstrap-dev.sh` | 新设备无 root 安装仓库锁定的 Go 与 golangci-lint | 人（显式执行） |
| `dev-doctor.sh` | 只读检查工具链版本与本地依赖 | 人、`check-local.sh` |
| `test-dev-doctor.sh` | fake PATH 验证 doctor 禁用 Go 自动下载 | `check-local.sh` |
| `check-local.sh` | 一次运行完整本地门禁 | 人 |
| `generate-third-party-licenses.sh` | 生成/校验发布所带第三方许可证正文 | 人、`check-local.sh` |
| `test-release-checksums.sh` | 验证许可精确项与 Linux-only Release 产物边界 | `check-local.sh` |
| `install.sh` | 校验 Release SHA256 后原子安装 Linux amd64 二进制 | 人、`curl \| sh` |
| `test-install.sh` | fake 下载验证 Linux 安装器校验、幂等、旧版本保护与平台拒绝 | `check-local.sh`、`ci.yml` |
| `_sha256.sh` | Unix SHA256 兼容函数 | 被 bootstrap、release、校验脚本 source；**`install.sh` 不 source 它**——它以 `curl \| sh` 发布，运行时加载校验器会让校验器自身可被替换，故内联 |
| `check-release-checksums.sh` | 用主机可用的 SHA256 工具复验发布产物 | 人 |
| `_stamp.sh` | **版本戳单一事实源**：算 VERSION / COMMIT / DATE 与 ldflags | 被 `build.sh`、`release-build.sh` source |
| `build.sh` | 本机构建带版本戳的 `build/aiah` | 人、`demo-apply-scan-loop.sh` |
| `release-build.sh` | Linux amd64 发布产物 + 许可材料 + `SHA256SUMS` + 版本自检 | 人（本地预演）、`release.yml` |
| `check-gofmt.sh` | gofmt 门禁 | 人、`ci.yml`、`release.yml` |
| `demo-apply-scan-loop.sh` | 假 HOME 闭环：build → apply → scan → rollback | 人、`ci.yml` |

两条约定：

1. **CI 只调用这些脚本，不写 inline 步骤**。workflow YAML 没法本地跑，脚本可以；
   门禁必须能在本机复现，见 [development.md §2.3](../docs/development.md)。
2. **版本戳只算一次**。要改 VERSION/COMMIT/DATE 的推导规则就改 `_stamp.sh`，
   别在调用方各算一遍——那正是本地构建与发布构建开始分叉的方式。

新设备搭建、PATH 约束与版本升级流程见
[开发环境搭建 SOP](../docs/runbooks/development-environment.md)。
