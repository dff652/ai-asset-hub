package tui

import (
	"strings"

	"github.com/dff652/ai-asset-hub/internal/readiness"
)

func (m Model) readinessView(style styles) string {
	levelLabel := m.readinessLevelLabel(m.readinessFlow.report.Level)
	header := joinEdges(
		style.header.Render(m.text(msgReadinessTitle)),
		levelLabel,
		max(20, m.width),
	)

	if m.readinessFlow.choosingEvidence != readinessEvidenceNone {
		return header + "\n" + m.readinessEvidenceInputView(style)
	}

	switch m.readinessFlow.status {
	case statusLoading:
		return header + "\n\n" + m.text(msgReadinessChecking)
	case statusFailed:
		message := m.text(msgReadinessFailed)
		if m.readinessFlow.err != nil {
			message += ": " + m.readinessFlow.err.Error()
		}
		return header + "\n\n" + style.error.Render(message) + "\n" + m.text(msgReadinessRetry)
	}

	report := m.readinessFlow.report
	lines := []string{
		"",
		m.text(
			msgReadinessSubject,
			displayCLIValue(report.Subject.Name),
			displayCLIValue(report.Subject.Version),
			displayCLIValue(report.Subject.Profile),
		),
	}
	if digest := strings.TrimSpace(report.Subject.SelectionSHA256); digest != "" {
		short := digest
		if len(short) > 16 {
			short = short[:16] + "…"
		}
		lines = append(lines, m.text(msgReadinessSelection, short))
	}
	lines = append(lines, "")

	packageLine := m.text(
		msgReadinessPackage,
		report.PackageReadiness.Status,
		m.readinessStatusLabel(report.PackageReadiness.Status),
		report.PackageReadiness.AssetCount,
		report.PackageReadiness.FileCount,
	)
	preflightLine := m.text(
		msgReadinessPreflight,
		report.MigrationPreflight.Status,
		m.readinessStatusLabel(report.MigrationPreflight.Status),
		displayCLIList(report.MigrationPreflight.Targets),
		report.MigrationPreflight.MissingSecrets,
		report.MigrationPreflight.DegradedItems,
	)
	backupLine := m.text(
		msgReadinessBackup,
		report.BackupEvidence.Status,
		m.readinessStatusLabel(report.BackupEvidence.Status),
	)
	restoreLine := m.text(
		msgReadinessRestore,
		report.RestoreExercise.Status,
		m.readinessStatusLabel(report.RestoreExercise.Status),
	)

	// Status is always textual (enum code + localized label). Style may add
	// color for attention/blocked, but meaning survives plain mode and width.
	lines = append(lines,
		m.readinessStyledLine(style, report.PackageReadiness.Status, packageLine),
		m.readinessStyledLine(style, report.MigrationPreflight.Status, preflightLine),
		m.readinessStyledLine(style, report.BackupEvidence.Status, backupLine),
	)
	if report.BackupEvidence.Status == readiness.StatusRecorded {
		lines = append(lines, m.text(
			msgReadinessBackupDetail,
			valueOrDash(report.BackupEvidence.Type),
			valueOrDash(report.BackupEvidence.RecordedAt),
			valueOrDash(report.BackupEvidence.ReferenceDigest),
		))
	}
	lines = append(lines, m.readinessStyledLine(style, report.RestoreExercise.Status, restoreLine))
	if report.RestoreExercise.Status == readiness.StatusPassed ||
		report.RestoreExercise.Status == readiness.StatusFailed {
		lines = append(lines, m.text(
			msgReadinessRestoreDetail,
			valueOrDash(report.RestoreExercise.PackageSHA256),
			displayCLIList(report.RestoreExercise.Targets),
			valueOrDash(report.RestoreExercise.CompletedAt),
		))
	}

	lines = append(lines,
		"",
		style.muted.Render(m.text(msgReadinessEvidencePaths)),
		m.text(msgReadinessEvidenceBackup, m.readinessEvidenceDisplay(m.readinessFlow.backupEvidencePath)),
		m.text(msgReadinessEvidenceRestore, m.readinessEvidenceDisplay(m.readinessFlow.restoreExercisePath)),
		"",
		m.readinessNextStep(report),
	)

	for _, finding := range report.Findings {
		lines = append(lines, m.text(
			msgReadinessFinding,
			finding.Severity,
			finding.Code,
			finding.Message,
		))
	}

	footer := style.muted.Render(m.text(msgReadinessFooter))
	if m.notice != "" {
		noticeStyle := style.muted
		if m.noticeIsWarn {
			noticeStyle = style.warning
		}
		footer = noticeStyle.Render(m.notice) + "\n" + footer
	}

	body := strings.Join(lines, "\n")
	if m.width > 0 && m.width < 80 {
		// Narrow terminals: keep full lines but allow wrapping via truncation
		// of only non-essential trailing whitespace; status codes stay first.
		wrapped := make([]string, 0, len(lines))
		for _, line := range lines {
			wrapped = append(wrapped, truncate(line, max(20, m.width)))
		}
		body = strings.Join(wrapped, "\n")
	}
	return header + "\n" + body + "\n" + footer
}

func (m Model) readinessEvidenceInputView(style styles) string {
	title := m.text(msgReadinessEvidenceInputBackupTitle)
	if m.readinessFlow.choosingEvidence == readinessEvidenceRestore {
		title = m.text(msgReadinessEvidenceInputRestoreTitle)
	}
	return strings.Join([]string{
		"",
		style.header.Render(title),
		m.text(msgReadinessEvidenceInputPrompt),
		m.readinessFlow.evidenceInput.View(),
		"",
		m.text(msgReadinessEvidenceInputBoundary),
		m.text(msgReadinessEvidenceInputFooter),
	}, "\n")
}

func (m Model) readinessHelpView(style styles) string {
	return strings.Join([]string{
		style.header.Render(m.text(msgReadinessHelpTitle)),
		"",
		m.text(msgReadinessHelpIntro),
		m.text(msgReadinessHelpCore),
		m.text(msgReadinessHelpEvidence),
		"",
		m.text(msgReadinessHelpBackupKey),
		m.text(msgReadinessHelpRestoreKey),
		m.text(msgReadinessHelpReload),
		m.text(msgReadinessHelpHome),
		m.text(msgReadinessHelpN104),
		m.text(msgReadinessHelpClose),
		m.text(msgReadinessHelpQuit),
	}, "\n")
}

func (m Model) readinessNextStep(report readiness.Report) string {
	switch {
	case report.Level == readiness.LevelBlocked:
		return m.text(msgReadinessNextBlocked)
	case report.BackupEvidence.Status != readiness.StatusRecorded:
		return m.text(msgReadinessNextBackup)
	case report.RestoreExercise.Status != readiness.StatusPassed:
		return m.text(msgReadinessNextRestore)
	default:
		return m.text(msgReadinessNextReady)
	}
}

func (m Model) readinessEvidenceDisplay(path string) string {
	if strings.TrimSpace(path) == "" {
		return m.text(msgReadinessEvidenceUnset)
	}
	return path
}

func (m Model) readinessStatusLabel(status string) string {
	switch status {
	case readiness.StatusReady:
		return m.text(msgReadinessStatusReady)
	case readiness.StatusAttention:
		return m.text(msgReadinessStatusAttention)
	case readiness.StatusBlocked:
		return m.text(msgReadinessStatusBlocked)
	case readiness.StatusMissing:
		return m.text(msgReadinessStatusMissing)
	case readiness.StatusRecorded:
		return m.text(msgReadinessStatusRecorded)
	case readiness.StatusMismatch:
		return m.text(msgReadinessStatusMismatch)
	case readiness.StatusInvalid:
		return m.text(msgReadinessStatusInvalid)
	case readiness.StatusUnchecked:
		return m.text(msgReadinessStatusUnchecked)
	case readiness.StatusPassed:
		return m.text(msgReadinessStatusPassed)
	case readiness.StatusFailed:
		return m.text(msgReadinessStatusFailed)
	default:
		return status
	}
}

func (m Model) readinessLevelLabel(level string) string {
	switch level {
	case readiness.LevelReady:
		return m.text(msgReadinessLevelReady)
	case readiness.LevelBlocked:
		return m.text(msgReadinessLevelBlocked)
	default:
		return m.text(msgReadinessLevelAttention)
	}
}

func (m Model) readinessStyledLine(style styles, status, line string) string {
	switch status {
	case readiness.StatusBlocked, readiness.StatusFailed, readiness.StatusInvalid:
		return style.error.Render(line)
	case readiness.StatusMismatch, readiness.StatusMissing, readiness.StatusAttention, readiness.StatusUnchecked:
		return style.warning.Render(line)
	default:
		return line
	}
}

// displayCLIValue and displayCLIList keep empty machine values readable without
// inventing product state. They mirror the CLI readiness formatter intentionally.
func displayCLIValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func displayCLIList(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ",")
}
