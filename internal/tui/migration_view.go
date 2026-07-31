package tui

import (
	"fmt"
	"strings"
)

func (m Model) migrationView(style styles) string {
	if m.migrationFlow.mode == migrationModeVersions {
		return m.versionsView(style)
	}
	if m.migrationFlow.mode == migrationModePreflight {
		return m.preflightView(style)
	}
	header := joinEdges(
		style.header.Render(m.text(msgMigrationTitle)),
		m.text(msgMigrationAlignmentTitle),
		max(20, m.width),
	)
	if m.migrationFlow.publishing {
		return header + "\n\n" + style.warning.Render(m.text(msgMigrationPublishing))
	}
	if m.migrationFlow.pulling {
		return header + "\n\n" + style.warning.Render(m.text(msgMigrationPulling))
	}
	switch m.migrationFlow.status {
	case statusLoading:
		return header + "\n\n" + m.text(msgMigrationLoading)
	case statusFailed:
		message := m.text(msgMigrationFailed)
		if m.migrationFlow.err != nil {
			message += ": " + m.migrationFlow.err.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n" + m.text(msgMigrationFailedFooter)
	}

	report := m.migrationFlow.report
	lines := []string{
		header,
		style.muted.Render(m.text(msgMigrationReadOnlyIntro)),
		"",
		style.header.Render(m.text(msgMigrationLibraryTitle)),
		m.text(msgMigrationLibraryPath, report.Library.Root),
		m.text(
			msgMigrationLibraryNameVersion,
			valueOrDash(report.Library.Name),
			valueOrDash(report.Library.Version),
		),
		m.text(
			msgMigrationLibraryAssetsProfiles,
			report.Library.AssetCount,
			len(report.Library.Profiles),
			valueOrDash(strings.Join(report.Library.Profiles, m.text(msgCommonListSeparator))),
		),
		m.text(msgMigrationStatus, m.okLabel(report.Library.Ok)),
		"",
		style.header.Render(m.text(msgMigrationInstallationTitle)),
	}
	if !report.Installation.Present {
		lines = append(lines, m.text(msgMigrationInstallationNone))
	} else {
		lines = append(lines,
			m.text(
				msgMigrationInstallationNameVersion,
				report.Installation.Package,
				report.Installation.Version,
			),
			m.text(msgMigrationInstallationProfile, valueOrDash(report.Installation.Profile)),
			m.text(
				msgMigrationInstallationTargets,
				valueOrDash(strings.Join(
					report.Installation.Targets,
					m.text(msgCommonListSeparator),
				)),
			),
			m.text(msgMigrationStatus, m.okLabel(report.Installation.Ok)),
		)
	}
	lines = append(lines, "", style.header.Render(m.text(msgMigrationChannelTitle)))
	if !report.Channel.Selected {
		lines = append(lines, m.text(msgMigrationChannelNone))
	} else {
		lines = append(lines,
			m.text(msgMigrationChannelPath, report.Channel.Path),
			m.text(msgMigrationChannelReleases, report.Channel.ReleaseCount),
		)
		if report.Channel.Latest == nil {
			lines = append(lines, m.text(msgMigrationChannelLatestNone))
		} else {
			lines = append(lines,
				m.text(
					msgMigrationChannelLatest,
					report.Channel.Latest.Version,
					report.Channel.Latest.Profile,
				),
				m.text(msgMigrationChannelSHA, report.Channel.Latest.SHA256),
			)
		}
	}
	lines = append(lines,
		"",
		style.header.Render(m.text(msgMigrationAlignmentTitle)),
		m.text(
			msgMigrationAlignmentInstallation,
			m.alignmentLabel(report.Alignment.Installation),
		),
		m.text(msgMigrationAlignmentChannel, m.alignmentLabel(report.Alignment.Channel)),
	)
	if len(report.Findings) > 0 {
		lines = append(lines, m.text(msgMigrationFindings, len(report.Findings)))
	}
	lines = append(lines,
		"",
		style.muted.Render(m.text(msgMigrationFooter)),
	)
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		lines = append(lines, noticeStyle.Render(m.notice))
	}
	latestSHA := ""
	if report.Channel.Latest != nil {
		latestSHA = report.Channel.Latest.SHA256
	}
	for index := range lines {
		if containsNonEmptyValue(
			lines[index],
			report.Library.Root,
			report.Channel.Path,
			latestSHA,
		) {
			continue
		}
		lines[index] = truncate(lines[index], max(20, m.width))
	}
	return strings.Join(lines, "\n")
}

type preflightRow struct {
	label   string
	status  string
	details []string
}

func (m Model) preflightRows() []preflightRow {
	report := m.migrationFlow.preflightReport
	rows := make([]preflightRow, 0,
		len(report.Targets)+len(report.Secrets)+len(report.DevicePrivate)+len(report.Findings))
	for _, target := range report.Targets {
		status := m.text(msgPreflightStatusNormal)
		switch {
		case !target.Supported:
			status = m.text(msgPreflightStatusUnsupported)
		case len(target.Dropped) > 0:
			status = m.text(msgPreflightStatusBlocked)
		case len(target.Degraded) > 0:
			status = m.text(msgPreflightStatusDegraded)
		}
		details := []string{
			m.text(msgPreflightCheckTarget),
			m.text(msgPreflightTarget, target.Target),
			m.text(msgPreflightSupport, status),
			m.text(msgPreflightEmitted, target.Emitted),
			m.text(msgPreflightDroppedDegraded, len(target.Dropped), len(target.Degraded)),
		}
		for _, item := range target.Dropped {
			details = append(details, m.text(msgPreflightDroppedItem, item))
		}
		for _, item := range target.Degraded {
			details = append(details, m.text(msgPreflightDegradedItem, item))
		}
		rows = append(rows, preflightRow{
			label: m.text(msgPreflightTargetRow, target.Target), status: status, details: details,
		})
	}
	for _, secret := range report.Secrets {
		status := m.text(msgPreflightStatusAvailable)
		if !secret.Available {
			status = m.text(msgPreflightStatusMissing)
		}
		rows = append(rows, preflightRow{
			label:  fmt.Sprintf("secret · %s:%s", secret.Provider, secret.Name),
			status: status,
			details: []string{
				m.text(msgPreflightCheckSecret),
				m.text(msgPreflightReference, secret.Provider, secret.Name),
				m.text(msgPreflightLocalStatus, status),
				m.text(
					msgPreflightAffectedTargets,
					valueOrDash(strings.Join(secret.Targets, m.text(msgCommonListSeparator))),
				),
				"",
				m.text(msgPreflightSecretSafety),
			},
		})
	}
	for _, item := range report.DevicePrivate {
		rows = append(rows, preflightRow{
			label:  m.text(msgPreflightDeviceRow, item.LogicalPath),
			status: m.text(msgPreflightStatusExcluded),
			details: []string{
				m.text(msgPreflightCheckDevice),
				m.text(msgPreflightPath, item.LogicalPath),
				m.text(msgPreflightSource, item.Source),
				m.text(msgPreflightType, item.Type),
				m.text(msgPreflightItemStatus, item.Status),
				"",
				m.text(msgPreflightDeviceSafety),
			},
		})
	}
	for _, finding := range report.Findings {
		details := []string{
			m.text(msgPreflightCheckFinding),
			m.text(msgPreflightCode, finding.Code),
			m.text(msgPreflightSeverity, finding.Severity),
			m.text(msgPreflightDescription, finding.Message),
		}
		for _, path := range finding.Paths {
			details = append(details, "  "+path)
		}
		rows = append(rows, preflightRow{
			label:  m.text(msgPreflightFindingRow, finding.Code),
			status: finding.Severity, details: details,
		})
	}
	return rows
}

func (m Model) preflightView(style styles) string {
	title := m.text(msgPreflightTitle)
	context := valueOrDash(m.migrationFlow.preflightProfile)
	intro := m.text(msgPreflightIntro)
	loading := m.text(msgPreflightLoading)
	failed := m.text(msgPreflightFailed)
	footer := m.text(msgPreflightFooter)
	emptyFooter := m.text(msgPreflightEmptyFooter)
	failedFooter := m.text(msgPreflightFailedFooter)
	if m.hasPackagePreflight() {
		title = m.text(msgPackagePreflightTitle)
		release := m.migrationFlow.pulledReport
		context = valueOrDash(release.Name + " " + release.Version + " / " + release.Profile)
		intro = m.text(msgPackagePreflightIntro)
		loading = m.text(msgPackagePreflightLoading)
		failed = m.text(msgPackagePreflightFailed)
		footer = m.text(msgPackagePreflightFooter)
		emptyFooter = m.text(msgPackagePreflightEmptyFooter)
		failedFooter = m.text(msgPackagePreflightFailFooter)
	}
	header := joinEdges(
		style.header.Render(title),
		context,
		max(20, m.width),
	)
	switch m.migrationFlow.preflightStatus {
	case statusLoading:
		return header + "\n\n" + loading
	case statusFailed:
		message := failed
		if m.migrationFlow.preflightErr != nil {
			message += ": " + m.migrationFlow.preflightErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n" + failedFooter
	}

	report := m.migrationFlow.preflightReport
	result := m.text(msgPreflightResultOK)
	resultStyle := style.header
	if !report.Ok {
		result = m.text(msgPreflightResultBlocked)
		resultStyle = style.error
		if m.hasPackagePreflight() {
			footer = m.text(msgPackagePreflightBlockFooter)
			emptyFooter = m.text(msgPackagePreflightBlockEmpty)
		}
	} else if report.Summary.DegradedItems > 0 {
		result = m.text(msgPreflightResultDegraded)
		resultStyle = style.warning
	}
	summary := m.text(
		msgPreflightSummary,
		report.Summary.TargetCount,
		report.Summary.UnsupportedTargets,
		report.Summary.DroppedItems,
		report.Summary.DegradedItems,
		report.Summary.MissingSecrets,
		report.Summary.SecretReferences,
		report.Summary.DevicePrivateItems,
	)
	rows := m.preflightRows()
	if len(rows) == 0 {
		return strings.Join([]string{
			header,
			style.muted.Render(intro),
			"",
			resultStyle.Render(result),
			summary,
			"",
			m.text(msgPreflightEmpty),
			"",
			style.muted.Render(emptyFooter),
		}, "\n")
	}

	cursor := m.migrationFlow.preflightCursor
	if cursor < 0 || cursor >= len(rows) {
		cursor = 0
	}
	if m.width < 80 || !linesFitWidth(rows[cursor].details, max(24, m.width-min(60, m.width*48/100)-3)) {
		return m.narrowPreflightView(
			header,
			intro,
			resultStyle.Render(result),
			summary,
			footer,
			rows,
			cursor,
			style,
		)
	}
	bodyHeight := max(5, m.height-9)
	start, end := visibleRange(len(rows), cursor, bodyHeight)
	leftWidth := max(32, min(60, m.width*48/100))
	rightWidth := max(24, m.width-leftWidth-3)
	leftLines := make([]string, 0, bodyHeight)
	for index := start; index < end; index++ {
		prefix := "  "
		if index == cursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s [%s]", prefix, rows[index].label, rows[index].status)
		line = truncate(line, leftWidth)
		if index == cursor {
			line = style.selected.Render(line)
		}
		leftLines = append(leftLines, line)
	}
	for len(leftLines) < bodyHeight {
		leftLines = append(leftLines, "")
	}
	detailLines := append([]string(nil), rows[cursor].details...)
	for index := range detailLines {
		detailLines[index] = truncate(detailLines[index], rightWidth)
	}
	for len(detailLines) < bodyHeight {
		detailLines = append(detailLines, "")
	}
	if len(detailLines) > bodyHeight {
		detailLines = detailLines[:bodyHeight]
	}
	combined := make([]string, 0, bodyHeight)
	for index := 0; index < bodyHeight; index++ {
		left := padRight(truncate(leftLines[index], leftWidth), leftWidth)
		right := truncate(detailLines[index], rightWidth)
		line := left + " " + style.border.Render("│")
		if right != "" {
			line += " " + right
		}
		combined = append(combined, line)
	}

	lines := []string{
		header,
		style.muted.Render(intro),
		resultStyle.Render(result),
		truncate(summary, max(20, m.width)),
		strings.Join(combined, "\n"),
		style.muted.Render(footer),
	}
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		lines = append(lines, noticeStyle.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) narrowPreflightView(
	header, intro, result, summary, footer string,
	rows []preflightRow,
	cursor int,
	style styles,
) string {
	listHeight := max(3, min(6, m.height/3))
	start, end := visibleRange(len(rows), cursor, listHeight)
	lines := []string{
		header,
		style.muted.Render(intro),
		result,
		summary,
	}
	for index := start; index < end; index++ {
		prefix := "  "
		if index == cursor {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s [%s]", prefix, rows[index].label, rows[index].status)
		line = truncate(line, max(20, m.width))
		if index == cursor {
			line = style.selected.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, style.border.Render(strings.Repeat("─", max(20, m.width))))
	lines = append(lines, rows[cursor].details...)
	lines = append(lines, style.muted.Render(footer))
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		lines = append(lines, noticeStyle.Render(m.notice))
	}
	return strings.Join(lines, "\n")
}

func (m Model) channelInputView(style styles) string {
	lines := []string{
		style.header.Render(m.text(msgChannelInputTitle)),
		"",
		m.text(msgChannelInputDefinition),
		m.text(msgChannelInputReadOnly),
		"",
		m.migrationFlow.channelInput.View(),
		"",
		m.text(msgChannelInputFooter),
	}
	return strings.Join(appendInputNotice(lines, m.notice, m.noticeIsWarn, style), "\n")
}

func (m Model) publishConfirmationView(style styles) string {
	lines := []string{
		style.header.Render(m.text(msgPublishConfirmTitle)),
		"",
		m.text(msgPublishConfirmPackage, m.migrationFlow.publishPackage),
		m.text(msgPublishConfirmChannel, m.migrationFlow.channel),
		"",
		style.warning.Render(m.text(msgPublishConfirmWarning)),
		m.text(msgPublishConfirmInit),
		m.text(msgPublishConfirmPrompt),
		m.migrationFlow.publishInput.View(),
		"",
		m.text(msgPublishConfirmFooter),
	}
	return strings.Join(appendInputNotice(lines, m.notice, m.noticeIsWarn, style), "\n")
}

func (m Model) versionsView(style styles) string {
	header := joinEdges(
		style.header.Render(m.text(msgVersionsTitle)),
		filepathBaseOrDash(m.migrationFlow.channel),
		max(20, m.width),
	)
	switch m.migrationFlow.versionsStatus {
	case statusLoading:
		return header + "\n\n" + m.text(msgVersionsLoading)
	case statusFailed:
		message := m.text(msgVersionsFailed)
		if m.migrationFlow.versionsErr != nil {
			message += ": " + m.migrationFlow.versionsErr.Error()
		}
		return header + "\n\n" + style.error.Render(message) +
			"\n\n" + m.text(msgVersionsFailedFooter)
	}

	lines := []string{
		header,
		style.muted.Render(m.text(msgVersionsOrdering)),
		"",
	}
	if len(m.migrationFlow.versionsReport.Releases) == 0 {
		lines = append(lines,
			m.text(msgVersionsEmpty),
			"",
			style.muted.Render(m.text(msgVersionsEmptyFooter)),
		)
		return strings.Join(lines, "\n")
	}
	for index, release := range m.migrationFlow.versionsReport.Releases {
		prefix := "  "
		if index == m.migrationFlow.versionsCursor {
			prefix = "> "
		}
		line := fmt.Sprintf(
			"%s%-18s %-16s %s",
			prefix,
			release.Version,
			release.Profile,
			shortSHA(release.SHA256),
		)
		if index == m.migrationFlow.versionsCursor {
			line = style.selected.Render(line)
		}
		lines = append(lines, line)
	}
	selected := m.migrationFlow.versionsReport.Releases[m.migrationFlow.versionsCursor]
	selectedLine := m.text(
		msgVersionsSelected,
		selected.Name,
		selected.Version,
		selected.Profile,
	)
	shaLine := m.text(msgVersionsSHA, selected.SHA256)
	lines = append(lines,
		"",
		selectedLine,
		shaLine,
		"",
		style.muted.Render(m.text(msgVersionsFooter)),
	)
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		lines = append(lines, noticeStyle.Render(m.notice))
	}
	for index := range lines {
		if lines[index] == selectedLine || lines[index] == shaLine {
			continue
		}
		lines[index] = truncate(lines[index], max(20, m.width))
	}
	return strings.Join(lines, "\n")
}

func containsNonEmptyValue(line string, values ...string) bool {
	for _, value := range values {
		if value != "" && strings.Contains(line, value) {
			return true
		}
	}
	return false
}

func (m Model) pullOutputView(style styles) string {
	release := m.migrationFlow.selectedRelease
	lines := []string{
		style.header.Render(m.text(msgPullOutputTitle)),
		"",
		m.text(msgPullOutputRelease, release.Name, release.Version, release.Profile),
		m.text(msgPullOutputChannel, m.migrationFlow.channel),
		"",
		m.text(msgPullOutputBoundary),
		m.text(msgPullOutputConflict),
		"",
		m.migrationFlow.pullOutInput.View(),
		"",
		m.text(msgPullOutputFooter),
	}
	return strings.Join(appendInputNotice(lines, m.notice, m.noticeIsWarn, style), "\n")
}

func (m Model) migrationHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render(m.text(msgMigrationHelpTitle)),
		"",
		m.text(msgMigrationHelpIntro),
		m.text(msgMigrationHelpVersions),
		"",
		m.text(msgMigrationHelpPreflight),
		m.text(msgMigrationHelpPublish),
		m.text(msgMigrationHelpPull),
		m.text(msgMigrationHelpChannel),
		m.text(msgMigrationHelpReload),
		m.text(msgMigrationHelpHome),
		m.text(msgMigrationHelpClose),
		m.text(msgMigrationHelpQuit),
		"",
		m.text(msgMigrationHelpChecks),
		m.text(msgMigrationHelpApply),
		m.text(msgMigrationHelpCore),
		m.text(msgMigrationHelpBoundary),
	}, "\n")
}

func filepathBaseOrDash(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "—"
	}
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func shortSHA(value string) string {
	if len(value) <= 12 {
		return valueOrDash(value)
	}
	return value[:12]
}

func appendInputNotice(lines []string, notice string, warning bool, style styles) []string {
	if notice == "" {
		return lines
	}
	noticeStyle := style.muted
	if warning {
		noticeStyle = style.warning
	}
	return append(lines, "", noticeStyle.Render(notice))
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func (m Model) okLabel(ok bool) string {
	if ok {
		return m.text(msgMigrationStatusOK)
	}
	return m.text(msgMigrationStatusRisks)
}

func (m Model) alignmentLabel(value string) string {
	switch value {
	case "same-version":
		return m.text(msgMigrationAlignmentSame)
	case "different-version":
		return m.text(msgMigrationAlignmentDifferent)
	case "different-package":
		return m.text(msgMigrationAlignmentOtherLibrary)
	case "not-installed":
		return m.text(msgMigrationAlignmentNotInstalled)
	case "not-published":
		return m.text(msgMigrationAlignmentNotPublished)
	case "channel-not-selected":
		return m.text(msgMigrationAlignmentNoChannel)
	default:
		return valueOrDash(value)
	}
}
