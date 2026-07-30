# TUI 产品体验方案 V2 评审

- 日期：2026-07-29
- 评审对象：[TUI 产品体验与导航方案 V2](../designs/tui-product-experience-v2.md)
- 结论：**通过，可按 E1 → E4 分期实施**

## 1. 结论

方案解决的是产品入口和用户心智问题，不改变已验证的 Core、安全边界或包格式。
“资产管理”定位与现有数据模型、命令和测试一致；“知识库”定位会引入当前不存在的
创作、检索、问答和知识运营预期，因此不采用。

E1 可以立即实施。E2–E4 必须按方案验收，不能在首页放置尚未完成的入口。

## 2. 术语审查

通过的主术语：

- AI 编程资产管理器；
- 资产库；
- 本机 AI 资产；
- 加入资产库；
- 资产组合；
- 目标工具；
- 变更预览；
- 确认应用；
- 安装检查；
- 撤销上次安装；
- 风险与问题。

不作为第一层用户文案：

- inventory、workspace、compose、build、profile、targets、diff、apply、doctor、
  rollback；
- “写出”“组包”“部署”。

例外：`apply` / `rollback` 作为 typed confirmation 和 CLI 命令必须保留，但界面要
先解释它们分别代表“确认应用”和“撤销安装”。

## 3. 业务流程审查

底层流程继续是：

```text
scan → compose → validate → build → diff → apply → doctor / rollback
```

用户主流程改为：

```text
整理本机资产 → 加入资产库 → 预览变化 → 确认应用
```

该映射没有省略安全步骤，只隐藏不必要的工程认知，成立。

## 4. 外部对照

- chezmoi 使用 source state、add、diff、apply，证明“源状态 → 预览 → 应用”是成熟
  且可解释的配置管理心智：
  <https://www.chezmoi.io/quick-start/>
- lazygit 直接运行主命令进入 TUI，同时保留完整 CLI：
  <https://github.com/jesseduffield/lazygit#usage>
- K9s 直接运行主命令进入界面，并保留参数和命令模式：
  <https://k9scli.io/topics/commands/>
- ansible-navigator 同时提供 welcome、inventory、settings 等任务入口：
  <https://docs.ansible.com/projects/navigator/subcommands/>
- IBM 与 Atlassian 对知识管理/知识库的说明都以集中存储、检索、分享和自助获取
  组织知识为核心，和 aiah 当前的可执行资产生命周期不同：
  <https://www.ibm.com/think/topics/knowledge-management>、
  <https://www.atlassian.com/itsm/knowledge-management/what-is-a-knowledge-base>

这些参考支持“默认进入任务首页、按用户任务组织”的方向；不支持复制它们的配置
模型或弱化 aiah 的 typed confirmation。

## 5. 风险与门槛

### 必须守住

- 不能因“开箱即用”恢复隐式写入；
- 建议路径可以预填，但必须明确确认；
- `aiah` 默认启动只允许在真实交互 TTY；
- dashboard 不能显示不可用的假入口；
- 语言开关必须建立在完整字符串目录上，不能做半中文半英文切换；
- UI 偏好与 manifest 业务事实必须分开。

### 暂不阻塞 E1

- 跨设备 publish/pull 的交互细节；
- 设置文件 schema；
- 英文翻译；
- 历史 backup 的可视化选择。

## 6. 实施建议

先交付 E1 的可验证垂直切片：

1. 任务首页；
2. 友好主标题和动作词；
3. `aiah` 默认启动与非 TTY 回退；
4. 回归测试和文档。

E1 完成后用真实安装包做首次启动 dogfood，再进入连续向导 E2。

## 7. E1 实施复核

2026-07-29 已完成 E1 代码与文档：

- 交互 TTY 中直接运行 `aiah` 进入任务首页，`aiah ui` 保持兼容；
- 非 TTY 裸命令只输出帮助并非零退出，不启动界面、不写目录；
- 首页、本机资产、变更预览、安装检查、关于与更新均采用本方案术语；
- 首页只展示已实现任务，设置、双语和跨设备入口没有提前暴露；
- `scripts/check-local.sh` 通过，包括全部 Go 测试、静态门禁和隔离假 HOME 闭环；
- 本地编译二进制在隔离 HOME 的真实 PTY 中用裸命令启动，首页四个任务均可见并可
  正常退出。

这证明 E1 开发态垂直切片成立；从正式安装包执行的真机 dogfood 仍保留为进入 E2
前的验收项，不能由本地编译冒烟测试替代。
