# N7.5 设置与 i18n 源码候选验收

- 日期：2026-07-31
- 分支：`feat/n7-i18n-catalog`
- 范围：100/60 列双语页面、写入确认作用域、fake HOME/config、Linux amd64
  release-style 候选
- 结论：源码候选可进入正式 Release 安装包复验；尚未发布，不能标记为已发布能力

## 1. 本次补齐的产品信息

设置的显示密度不能隐藏决策和恢复信息。N7.5 因此不只检查界面是否“能画出来”，
而是逐屏检查用户做决定所需的值是否仍完整：

| 页面 | 必须完整显示 |
|---|---|
| 首页、资产页 | 当前资产库、资产状态、所选资产路径和文件 |
| 变更预览、阻止页 | 安装包、目标工具、变更路径、SHA256、Core finding |
| 安装检查 | 包、版本、资产组合、备份 ID、漂移或 finding |
| 跨设备迁移 | 资产库、通道、包/版本/组合、targets、完整 SHA256、设备私有路径 |
| 关于与更新 | 当前/最新版本、Release 页、可复制升级命令 |
| 偏好设置 | 当前首选资产库完整路径和实际偏好文件路径 |

双栏的详情区域放不下必要值时，界面改用“列表 → 分隔线 → 完整详情”的单栏布局。
该判断既覆盖 60 列，也覆盖 100 列下过长的 SHA 或路径，而不是用固定宽度猜测内容。

## 2. 写入确认矩阵

zh-CN/en × 100/60 列均覆盖：

| 操作 | 确认前必须显示 |
|---|---|
| `apply` | 完整包路径、targets、create/update/unchanged/skipped 数量、`apply` |
| `rollback` | 当前包、版本、资产组合、备份 ID、`rollback` |
| `publish` | 完整包路径、通道路径、发布边界、`publish` |
| `update` | 资产库路径、所选资产数量、源端更新边界、`update` |
| `remove` | 资产库路径、所选资产数量、源端不删除/非备份边界、`remove` |

确认词继续保持稳定英文，不随界面语言变化。测试入口：

```text
go test ./internal/tui -run TestN75WriteConfirmationsPreserveScopeAtSupportedWidths -count=1
```

## 3. fake HOME/config

`TestN75FakeHomePreferenceLifecycleAndCorruptionRecovery` 使用注入路径完成一个连续生命周期：

1. 无偏好文件启动，不创建配置目录；
2. 明确保存 English + detailed；
3. 以中文 locale 重启，仍恢复已保存值；
4. 写入损坏 JSON 后重启，回退安全默认值并给出稳定 warning；
5. 损坏文件保持逐字节不变，范围外哨兵目录保持逐字节不变。

该测试与 preferences Core 原有的软链、权限、原子替换和中途失败测试共同证明：
可选界面偏好不会成为隐式写入入口，也不会落入被测 fake config 之外。

## 4. 安全变异

本次新增测试做了两项手工变异：

1. 令“详情是否适合双栏”的判断永远返回 true：100 列的 64 位 SHA 被截断，
   `TestN75CoreViewsPreserveRequiredInformationAtSupportedWidths` 按预期失败；
2. 删除 apply 确认页的完整包路径：
   `TestN75WriteConfirmationsPreserveScopeAtSupportedWidths` 在四组语言/宽度组合中
   全部按预期失败。

两项生产改动均已恢复。它们证明测试约束的是安全信息，而不只是标题或快照。

## 5. 完整门禁

修复测试中一处静态检查发现的无效赋值，并把它改为显式首启 locale/defaults
断言后，重新从头执行：

```bash
./scripts/check-local.sh
git diff --check
```

最终通过 dev doctor、脚本语法与安装器/Release 校验、README 资产检查、`go test
./...`、`go test -race ./...`、`go vet ./...`、gofmt、golangci-lint 和真实
apply/doctor/rollback 闭环。失败的中间运行不计为通过。

## 6. 本地 release-style 候选

使用隔离临时目录构建并安装候选，不写真实 HOME/XDG config：

```bash
VERSION=0.1.7-dev.n7.5 OUT="$candidate_root/release" ./scripts/release-build.sh
install -m 0755 \
  "$candidate_root/release/aiah_0.1.7-dev.n7.5_linux_amd64" \
  "$candidate_root/install/bin/aiah"
```

验收结果：

- `SHA256SUMS`、Linux amd64 静态 ELF、strip 状态和 `aiah version` 自检通过；
- 60 列真实 PTY 中裸 `aiah` 进入任务首页；
- 保存中文 + detailed 后，配置目录/文件分别为 `0700` / `0600`；
- English + standard CLI override 只改变当前进程，保存文件 SHA256 不变；
- 重启恢复保存值；损坏 JSON 后安全回退、首页显示警告且文件不自动覆盖。

locale 遵循 `LC_ALL` → `LC_MESSAGES` → `LANG` 优先级；测试环境已有上层 locale 时，
只改 `LANG` 不应覆盖它。显式语言 override 与保存值的优先级均按方案工作。

## 7. 尚未完成

本记录中的二进制来自当前工作树和本地 release 构建脚本，不是 GitHub Release
下载产物。以下动作未获授权，也未执行：

- commit 之外的 push、PR、tag、GitHub Release；
- 用安装器替换真实用户的 `aiah`；
- 把 README 或上手指南中的双语设置改写为“已发布”。

下一步是在用户明确授权发版后，从正式 Release 下载 Linux amd64 产物，按
[安装/升级 dogfood SOP](../runbooks/install-upgrade-dogfood.md)完成首次启动、
保存、重启、语言切换、损坏恢复和幂等复装。只有该步骤通过，N7 与用户文档发布声明
才能收口。
