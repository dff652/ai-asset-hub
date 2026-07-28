# 第三方依赖许可证清单

- 更新时间：2026-07-28
- 本项目许可证：Apache-2.0（仓库根 `LICENSE`，署名见 `NOTICE`）
- 生成依据：`go list -m all`（模块图）与 Linux amd64 的 `go list -deps ./cmd/aiah`
  （实际链接进二进制的包）；发布所带 `THIRD_PARTY_LICENSES.txt` 由
  `scripts/generate-third-party-licenses.sh` 从 `$GOMODCACHE` 的原始
  `LICENSE` / `NOTICE` 生成

结论：**全部依赖均为 MIT / Apache-2.0 / BSD 系宽松协议，无 copyleft 传染**，
不影响本项目以 Apache-2.0 发布。

## 1. 链接进发布二进制的直接依赖

发布二进制静态链接以下模块，`NOTICE` 中已按各自协议要求保留署名。

| 模块 | 版本 | 协议 | 用途 | 直接/间接 |
|---|---|---|---|---|
| `github.com/charmbracelet/bubbletea` | v1.3.10 | MIT | TUI 事件循环与终端程序 | 直接 |
| `github.com/charmbracelet/bubbles` | v0.21.1 | MIT | TUI 键位与文本输入组件 | 直接 |
| `github.com/charmbracelet/lipgloss` | v1.1.0 | MIT | TUI 布局、宽度与样式 | 直接 |
| `github.com/charmbracelet/x/term` | v0.2.2 | MIT | TTY 探测，非 TTY fail-fast | 直接 |
| `github.com/pelletier/go-toml/v2` | v2.2.4 | MIT | Codex / Grok `config.toml` 读写 | 直接 |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 | Apache-2.0 | manifest / report schema 校验 | 直接 |
| `gopkg.in/yaml.v3` | v3.0.1 | Apache-2.0（libyaml 移植文件为 MIT） | manifest YAML 解析 | 直接 |

Linux amd64 发布构建的传递依赖：

| 模块 | 版本 | 协议 | 用途 |
|---|---|---|---|
| `github.com/atotto/clipboard` | v0.1.4 | BSD-3-Clause | Bubbles 文本输入的剪贴板支持 |
| `github.com/aymanbagabas/go-osc52/v2` | v2.0.1 | MIT | 终端 OSC 52 能力 |
| `github.com/charmbracelet/colorprofile` | v0.4.1 | MIT | 终端色彩能力探测 |
| `github.com/charmbracelet/x/ansi` | v0.11.5 | MIT | ANSI 文本处理 |
| `github.com/charmbracelet/x/cellbuf` | v0.0.15 | MIT | 终端 cell buffer |
| `github.com/clipperhouse/displaywidth` | v0.9.0 | MIT | Unicode 显示宽度 |
| `github.com/clipperhouse/stringish` | v0.1.1 | MIT | 显示宽度辅助 |
| `github.com/clipperhouse/uax29/v2` | v2.5.0 | MIT | Unicode grapheme 分段 |
| `github.com/lucasb-eyer/go-colorful` | v1.3.0 | MIT | 色彩转换 |
| `github.com/mattn/go-isatty` | v0.0.20 | MIT | 终端能力探测 |
| `github.com/mattn/go-runewidth` | v0.0.19 | MIT | Unicode rune 宽度 |
| `github.com/muesli/ansi` | 2023-03-16 pseudo-version | MIT | ANSI reader |
| `github.com/muesli/cancelreader` | v0.2.2 | MIT | 可取消终端输入 |
| `github.com/muesli/termenv` | v0.16.0 | MIT | 终端环境探测 |
| `github.com/rivo/uniseg` | v0.4.7 | MIT | Unicode 文本分段 |
| `github.com/xo/terminfo` | 2022-09-10 pseudo-version | MIT | terminfo 数据 |
| `golang.org/x/sys` | v0.38.0 | BSD-3-Clause | 平台系统调用 |
| `golang.org/x/text` | v0.14.0 | BSD-3-Clause | 本地化与 Unicode 文本 |

注意事项：

- `gopkg.in/yaml.v3` 是双协议：`apic.go` / `emitterc.go` / `parserc.go` /
  `readerc.go` / `scannerc.go` / `writerc.go` / `yamlh.go` / `yamlprivateh.go`
  从 libyaml 的 C 代码移植而来，仍受原 MIT 协议约束，其余部分为 Apache-2.0。
  该模块自带 `NOTICE`（Canonical Ltd.），按 Apache-2.0 §4(d) 已转录进本项目
  `NOTICE`。
- `github.com/santhosh-tekuri/jsonschema/v6` 自身不带 `NOTICE` 文件，只需保留
  其 Apache-2.0 协议文本与版权声明。
- TUI 依赖均为 MIT / BSD-3-Clause；没有引入 GPL / AGPL / LGPL。
- 当前清单只覆盖实际分发的 Linux amd64 二进制。其他平台恢复分发时，必须同步
  扩展生成范围并重新审计 build-tag 依赖。

## 2. 仅出现在模块图、未链接进二进制的模块

这些模块是依赖自身的测试或工具链依赖，不进入发布产物，仅为审计完整性登记。

| 模块 | 版本 | 协议 | 说明 |
|---|---|---|---|
| `github.com/MakeNowJust/heredoc` | v1.0.0 | MIT | Bubbles 的非运行时模块依赖 |
| `github.com/aymanbagabas/go-udiff` | v0.3.1 | BSD-3-Clause | UI 依赖的测试/工具路径 |
| `github.com/bits-and-blooms/bitset` | v1.24.4 | BSD-3-Clause | 依赖模块图 |
| `github.com/charmbracelet/harmonica` | v0.2.0 | MIT | Bubbles 可选组件依赖 |
| `github.com/charmbracelet/x/exp/golden` | 2024-10-11 pseudo-version | MIT | Charmbracelet 测试工具 |
| `github.com/dlclark/regexp2` | v1.11.0 | MIT | jsonschema 可选正则引擎，本项目未启用 |
| `github.com/dustin/go-humanize` | v1.0.1 | MIT | Bubbles 可选组件依赖 |
| `github.com/kylelemons/godebug` | v1.1.0 | Apache-2.0 | 依赖的测试/调试工具 |
| `github.com/sahilm/fuzzy` | v0.1.1 | MIT | Bubbles list 可选模糊搜索 |
| `golang.org/x/exp` | 2023-10-06 pseudo-version | BSD-3-Clause | 依赖的工具/实验包 |
| `golang.org/x/mod` | v0.8.0 | BSD-3-Clause | 依赖的工具链依赖 |
| `golang.org/x/tools` | v0.6.0 | BSD-3-Clause | 依赖的工具链依赖 |
| `gopkg.in/check.v1` | v0.0.0-20161208181325 | BSD-2-Clause | yaml.v3 的测试框架 |

## 3. 与 PromptHub 的边界

`docs/security.md` §6 的约束继续有效，且是本项目可以选择非 AGPL 协议的前提：

- 不复制 PromptHub（AGPL-3.0）源码；
- 不移植其受版权保护的实现细节；
- 只借鉴公开行为、数据分类与交互思路，并保留独立设计记录
  （`docs/research/prompthub-assessment.md`）。

## 4. 维护规则

- 新增或升级依赖时同步更新本清单与 `NOTICE`；引入任何 GPL / AGPL / LGPL 依赖
  前必须先评估协议兼容性，不得直接合入。
- 执行 `./scripts/generate-third-party-licenses.sh` 更新许可证正文，并用
  `./scripts/generate-third-party-licenses.sh --check` 验证未漂移。
- 复核命令：

```bash
go list -m all                      # 模块图
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go list -deps ./cmd/aiah          # 实际发布目标链接包
ls "$(go env GOMODCACHE)"/<module>@<version>/LICENSE
```
