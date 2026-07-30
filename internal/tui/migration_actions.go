package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dff652/ai-asset-hub/internal/channel"
	"github.com/dff652/ai-asset-hub/internal/migration"
)

type migrationMode int

const (
	migrationModeStatus migrationMode = iota
	migrationModeVersions
)

type migrationAction int

const (
	migrationActionNone migrationAction = iota
	migrationActionPublish
	migrationActionVersions
)

type migrationFlow struct {
	status          status
	report          migration.Report
	err             error
	channel         string
	channelInput    textinput.Model
	choosingChannel bool

	afterChannel   migrationAction
	afterMigration migrationAction
	mode           migrationMode

	versionsStatus status
	versionsReport channel.ListReport
	versionsErr    error
	versionsCursor int

	publishPackage    string
	publishConfirming bool
	publishInput      textinput.Model
	publishing        bool

	selectedRelease channel.Release
	choosingPullOut bool
	pullOutInput    textinput.Model
	pulling         bool
}

func newMigrationFlow() migrationFlow {
	channelInput := textinput.New()
	channelInput.Prompt = "> "
	channelInput.Placeholder = "/mnt/usb/aiah"
	channelInput.CharLimit = 512
	channelInput.Width = 64

	publishInput := textinput.New()
	publishInput.Prompt = "> "
	publishInput.Placeholder = "publish"
	publishInput.CharLimit = 7
	publishInput.Width = 14

	pullOutInput := textinput.New()
	pullOutInput.Prompt = "> "
	pullOutInput.Placeholder = "/tmp/aiah-incoming"
	pullOutInput.CharLimit = 512
	pullOutInput.Width = 64

	return migrationFlow{
		channelInput: channelInput,
		mode:         migrationModeStatus,
		publishInput: publishInput,
		pullOutInput: pullOutInput,
	}
}

type versionsMsg struct {
	report channel.ListReport
	err    error
}

type publishMsg struct {
	report channel.PublishReport
	err    error
}

type pullMsg struct {
	report channel.PullReport
	err    error
}

func versionsCommand(options channel.ListOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := channel.List(options)
		return versionsMsg{report: report, err: err}
	}
}

func publishCommand(options channel.PublishOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := channel.Publish(options)
		return publishMsg{report: report, err: err}
	}
}

func pullCommand(options channel.PullOptions) tea.Cmd {
	return func() tea.Msg {
		report, err := channel.Pull(options)
		return pullMsg{report: report, err: err}
	}
}

func (m Model) handleVersionsMessage(message versionsMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.versionsStatus = statusReady
	m.migrationFlow.versionsReport = message.report
	m.migrationFlow.versionsErr = message.err
	if message.err != nil {
		m.migrationFlow.versionsStatus = statusFailed
		m.migrationFlow.versionsCursor = 0
		return m, nil
	}
	if len(message.report.Releases) > 0 {
		m.migrationFlow.versionsCursor = len(message.report.Releases) - 1
	}
	return m, nil
}

func (m Model) handlePublishMessage(message publishMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.publishing = false
	if message.err != nil || !message.report.Ok {
		m.notice = "发布失败：" + operationError(message.err)
		m.noticeIsWarn = true
		return m, nil
	}
	action := "已发布"
	if message.report.Unchanged {
		action = "通道中已有相同版本"
	}
	m.notice = action + "：" + message.report.Name + " " +
		message.report.Version + " (" + message.report.Profile + ")"
	m.noticeIsWarn = false
	m.migrationFlow.status = statusLoading
	return m, migrationCommand(m.migrationOptions())
}

func (m Model) handlePullMessage(message pullMsg) (tea.Model, tea.Cmd) {
	m.migrationFlow.pulling = false
	if message.err != nil || !message.report.Ok {
		m.notice = "取回失败：" + operationError(message.err)
		m.noticeIsWarn = true
		return m, nil
	}
	m.deployOptions.Package = message.report.Package
	m.deployOptions.DryRun = false
	m.packageFromBuild = false
	m.screen = screenDeployment
	m.diffStatus = statusLoading
	m.deployErr = nil
	m.applyResult = nil
	m.diffCursor = 0
	m.notice = "已取回 " + message.report.Name + " " + message.report.Version +
		" (" + message.report.Profile + ")，已进入变更预览"
	m.noticeIsWarn = false
	return m, diffCommand(m.deployOptions)
}

func operationError(err error) string {
	if err == nil {
		return "Core 未返回成功状态"
	}
	return err.Error()
}

func (m Model) startMigrationPublish() (tea.Model, tea.Cmd) {
	if m.migrationFlow.status != statusReady {
		m.notice = "迁移状态尚未可用；先刷新，或按 c 重新选择通道"
		m.noticeIsWarn = true
		return m, nil
	}
	if strings.TrimSpace(m.migrationFlow.channel) == "" {
		return m.startChannelInputFor(migrationActionPublish)
	}
	m.migrationFlow.publishPackage = ""
	m.notice = "选择要生成并发布的资产组合"
	m.noticeIsWarn = false
	return m.startProfileInputFor(buildForPublish)
}

func (m Model) startPublishConfirmation(packagePath string) (tea.Model, tea.Cmd) {
	m.screen = screenMigration
	m.migrationFlow.mode = migrationModeStatus
	m.migrationFlow.publishPackage = packagePath
	m.migrationFlow.publishConfirming = true
	m.migrationFlow.publishInput.SetValue("")
	m.migrationFlow.publishInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updatePublishConfirmation(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(message, m.keys.Collapse) {
		m.migrationFlow.publishConfirming = false
		m.migrationFlow.publishPackage = ""
		m.migrationFlow.publishInput.SetValue("")
		m.migrationFlow.publishInput.Blur()
		m.notice = "已取消发布；生成的安装包保留在资产库 dist 目录"
		m.noticeIsWarn = false
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		if m.migrationFlow.publishInput.Value() != "publish" {
			m.notice = "确认文本不匹配；必须完整输入 publish"
			m.noticeIsWarn = true
			m.migrationFlow.publishInput.SetValue("")
			return m, nil
		}
		m.migrationFlow.publishConfirming = false
		m.migrationFlow.publishInput.Blur()
		m.migrationFlow.publishing = true
		m.notice = ""
		m.noticeIsWarn = false
		return m, publishCommand(channel.PublishOptions{
			Package: m.migrationFlow.publishPackage,
			Channel: m.migrationFlow.channel,
		})
	}
	var command tea.Cmd
	m.migrationFlow.publishInput, command = m.migrationFlow.publishInput.Update(message)
	return m, command
}

func (m Model) startVersions() (tea.Model, tea.Cmd) {
	if m.migrationFlow.status != statusReady {
		m.notice = "迁移状态尚未可用；先刷新，或按 c 重新选择通道"
		m.noticeIsWarn = true
		return m, nil
	}
	if strings.TrimSpace(m.migrationFlow.channel) == "" {
		return m.startChannelInputFor(migrationActionVersions)
	}
	if strings.TrimSpace(m.migrationFlow.report.Library.Name) == "" {
		m.notice = "无法确定资产库名称；先刷新迁移状态"
		m.noticeIsWarn = true
		return m, nil
	}
	m.screen = screenMigration
	m.migrationFlow.mode = migrationModeVersions
	m.migrationFlow.versionsStatus = statusLoading
	m.migrationFlow.versionsErr = nil
	m.migrationFlow.versionsCursor = 0
	m.notice = ""
	m.noticeIsWarn = false
	return m, versionsCommand(channel.ListOptions{
		Channel: m.migrationFlow.channel,
		Name:    m.migrationFlow.report.Library.Name,
	})
}

func (m Model) startPullOutput() (tea.Model, tea.Cmd) {
	if m.migrationFlow.versionsStatus != statusReady ||
		m.migrationFlow.versionsCursor < 0 ||
		m.migrationFlow.versionsCursor >= len(m.migrationFlow.versionsReport.Releases) {
		m.notice = "先选择一个可取回版本"
		m.noticeIsWarn = true
		return m, nil
	}
	m.migrationFlow.selectedRelease =
		m.migrationFlow.versionsReport.Releases[m.migrationFlow.versionsCursor]
	m.migrationFlow.choosingPullOut = true
	m.migrationFlow.pullOutInput.SetValue("")
	m.migrationFlow.pullOutInput.Focus()
	m.notice = ""
	m.noticeIsWarn = false
	return m, nil
}

func (m Model) updatePullOutput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(message, m.keys.Collapse) {
		m.migrationFlow.choosingPullOut = false
		m.migrationFlow.pullOutInput.SetValue("")
		m.migrationFlow.pullOutInput.Blur()
		return m, nil
	}
	if message.Type == tea.KeyEnter {
		candidate := expandHomePath(
			strings.TrimSpace(m.migrationFlow.pullOutInput.Value()),
			m.options.Home,
		)
		if candidate == "" {
			m.notice = "必须输入已有的取回目录；本页不会创建目录"
			m.noticeIsWarn = true
			return m, nil
		}
		m.migrationFlow.choosingPullOut = false
		m.migrationFlow.pullOutInput.Blur()
		m.migrationFlow.pulling = true
		m.notice = ""
		m.noticeIsWarn = false
		return m, pullCommand(channel.PullOptions{
			Channel: m.migrationFlow.channel,
			Name:    m.migrationFlow.selectedRelease.Name,
			Version: m.migrationFlow.selectedRelease.Version,
			Profile: m.migrationFlow.selectedRelease.Profile,
			Out:     candidate,
		})
	}
	var command tea.Cmd
	m.migrationFlow.pullOutInput, command = m.migrationFlow.pullOutInput.Update(message)
	return m, command
}

func (m Model) updateMigrationKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.migrationFlow.mode == migrationModeVersions {
		switch {
		case key.Matches(message, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(message, m.keys.Help):
			m.showHelp = true
		case key.Matches(message, m.keys.Up):
			if m.migrationFlow.versionsCursor > 0 {
				m.migrationFlow.versionsCursor--
			}
		case key.Matches(message, m.keys.Down):
			if m.migrationFlow.versionsCursor+1 < len(m.migrationFlow.versionsReport.Releases) {
				m.migrationFlow.versionsCursor++
			}
		case key.Matches(message, m.keys.First):
			m.migrationFlow.versionsCursor = 0
		case key.Matches(message, m.keys.Last):
			if len(m.migrationFlow.versionsReport.Releases) > 0 {
				m.migrationFlow.versionsCursor = len(m.migrationFlow.versionsReport.Releases) - 1
			}
		case key.Matches(message, m.keys.Expand):
			return m.startPullOutput()
		case key.Matches(message, m.keys.Reload):
			return m.startVersions()
		case key.Matches(message, m.keys.CheckUpdate):
			return m.startChannelInputFor(migrationActionVersions)
		case key.Matches(message, m.keys.Collapse):
			m.migrationFlow.mode = migrationModeStatus
			m.notice = ""
			m.noticeIsWarn = false
		}
		return m, nil
	}
	switch {
	case key.Matches(message, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(message, m.keys.Help):
		m.showHelp = true
	case key.Matches(message, m.keys.CheckUpdate):
		return m.startChannelInput()
	case key.Matches(message, m.keys.Publish):
		return m.startMigrationPublish()
	case key.Matches(message, m.keys.Version):
		return m.startVersions()
	case key.Matches(message, m.keys.Reload):
		return m.startMigration()
	case key.Matches(message, m.keys.Collapse):
		m.screen = screenHome
	}
	return m, nil
}
