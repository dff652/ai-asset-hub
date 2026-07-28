# 开发环境搭建 SOP

- 更新时间：2026-07-26
- 适用：新的 Linux / macOS 开发设备，以及设备切换后的环境复验
- 原则：仓库声明版本，设备只提供可重建工具链；不把用户名、HOME 绝对路径或某台
  机器的安装状态当成项目事实

## 1. 单一事实源

Go 版本只由仓库根 `go.mod` 声明：

```text
go 1.26.0
toolchain go1.26.5
```

`go` 是最低语言版本，`toolchain` 是本仓库开发使用的工具链。CI 通过
`go-version-file: go.mod` 读取同一文件。不要在会话交接里再写一份
`/home/<user>/.../go1.26.5`。

注意：`toolchain` 不能凭空启动。新设备上如果连 `go` 命令都没有，必须先做一次
bootstrap；已有 Go 1.21+ 时，Go 可以按 `GOTOOLCHAIN=auto` 自动选择和下载新工具链，
但本项目仍用下面的脚本安装精确版本，确保 `gofmt` 也来自同一套 GOROOT。

## 2. 新设备 bootstrap

从仓库根执行：

```bash
./scripts/bootstrap-dev.sh
```

脚本的边界：

- 只支持 Linux / macOS 的 amd64 / arm64；Windows 尚无原生行为验收；
- 从 `go.mod` 读取版本，从 `https://go.dev/dl/` 下载官方 archive，并用仓库内评审过
  的 SHA256 校验；
- 无 root 安装到 `${XDG_DATA_HOME:-$HOME/.local/share}/aiah/toolchains/`；
- 在 `${XDG_BIN_HOME:-$HOME/.local/bin}` 创建 `go` / `gofmt` 软链；
- 用仓库选中的 Go 从源码安装固定版本 `golangci-lint v1.62.2`；CI 使用相同的
  `goinstall` 口径，不下载由旧 Go 构建的预编译 linter；
- 不改 shell profile、不运行 `go env -w`、不覆盖系统 Go或已有的未知文件。

如果脚本提示 PATH 缺少用户 bin 目录，由设备所有者按所用 shell 显式加入；不要让
项目脚本代改 `.profile` / `.zshrc`。

## 3. 只读诊断

每次换设备、接手会话或怀疑 PATH 漂移时先运行：

```bash
./scripts/dev-doctor.sh
```

它只读检查：

- `go version` 是否为 `go.mod` 的 `toolchain`；
- `gofmt` 是否来自所选 GOROOT，避免 Go 与 gofmt 串版本；
- 所有 Go 探测均强制 `GOTOOLCHAIN=local`，确保 doctor 不触发自动下载；
- `golangci-lint` 是否为 CI 使用的 v1.62.2；
- `git`、`python3`、`curl`、`tar`、`file` 与 SHA256 工具是否存在。Linux 通常用
  `sha256sum`，macOS 可直接用 `shasum`。

诊断失败就是本地阻塞，不得把未运行的门禁推给 CI。

## 4. 本地验收

开发提交或发布预演前统一运行：

```bash
./scripts/check-local.sh
```

它按顺序执行环境诊断、所有 shell 脚本语法检查、`go test`、race、vet、gofmt、
golangci-lint 与假 HOME 闭环。任一步失败立即停止。需要定位单项问题时，仍可运行
[development.md §2.2](../development.md) 中的原始命令。

TUI 的真实 TTY 行为与发版前的真机 home 只读扫描不能由容器或交叉编译替代。
换设备后候选资产数量不同是正常的：先在该设备建立只读基线，再判断后续变化，
不要拿另一台设备的绝对数量直接判回归。

## 5. 版本升级

升级 Go 时必须同一个提交完成：

1. 用 Go 官方发布页确认目标版本与四个平台 archive 的 SHA256；
2. 更新 `go.mod`；
3. 更新 `scripts/bootstrap-dev.sh` 的版本 allowlist 与四个 SHA256；
4. 在至少一台新目录环境中实际跑 bootstrap；
5. 运行 `./scripts/check-local.sh`；
6. 本地验证过后才能调整 CI。

禁止只改交接文档里的版本号，或让 bootstrap 静默安装未评审的 latest。

## 6. 首次固化记录

2026-07-26 在一台新的 Linux amd64 设备上发现：PATH 已包含
`~/.local/bin`，但 Go 目录与软链均不存在。随后用本 SOP 安装并复验：

- Go `1.26.5`；doctor 强制 `GOTOOLCHAIN=local` 探测，Go 与 gofmt 来自同一
  GOROOT；
- golangci-lint `1.62.2`；
- bootstrap 连续执行两次均无重复安装或覆盖；
- 完整 `check-local.sh` 基线通过；
- 真机只读扫描基线为 15 个候选资产、0 finding（修复 Codex
  `vendor_imports` 与 Grok 根 `README.md` 两个设备状态漏项后）。

后续验收结果记录脚本输出与版本，不记录用户名、hostname 或真实 home 资产内容。
