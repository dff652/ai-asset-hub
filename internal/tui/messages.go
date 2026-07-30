package tui

import "fmt"

type language string

const (
	languageZhCN    language = "zh-CN"
	languageEnglish language = "en"
)

type messageID string

const (
	msgHomeAppTitle           messageID = "home.app_title"
	msgHomeSummary            messageID = "home.summary"
	msgHomeLibraryLabel       messageID = "home.library_label"
	msgHomeAssetStatusLabel   messageID = "home.asset_status_label"
	msgHomeInstallLabel       messageID = "home.install_label"
	msgHomeTaskPrompt         messageID = "home.task_prompt"
	msgHomeOrganizeTitle      messageID = "home.organize.title"
	msgHomeOrganizeDesc       messageID = "home.organize.description"
	msgHomeApplyTitle         messageID = "home.apply.title"
	msgHomeApplyDesc          messageID = "home.apply.description"
	msgHomeHealthTitle        messageID = "home.health.title"
	msgHomeHealthDesc         messageID = "home.health.description"
	msgHomeMigrationTitle     messageID = "home.migration.title"
	msgHomeMigrationDesc      messageID = "home.migration.description"
	msgHomeVersionTitle       messageID = "home.version.title"
	msgHomeVersionDesc        messageID = "home.version.description"
	msgHomeFooter             messageID = "home.footer"
	msgHomeWorkspaceUnset     messageID = "home.workspace_unset"
	msgHomeScanLoading        messageID = "home.scan_loading"
	msgHomeScanFailed         messageID = "home.scan_failed"
	msgHomeCatalogUnavailable messageID = "home.catalog_unavailable"
	msgHomeCatalogSummary     messageID = "home.catalog_summary"
	msgHomeDiscovered         messageID = "home.discovered"
	msgHomeNoManagedInstall   messageID = "home.no_managed_install"
	msgHomeManagedInstall     messageID = "home.managed_install"
	msgHomeInstallHealthy     messageID = "home.install_healthy"
	msgHomeInstallRisk        messageID = "home.install_risk"
	msgHomeDoctorFailed       messageID = "home.doctor_failed"
	msgHomeDoctorChecking     messageID = "home.doctor_checking"
	msgHomeOrganizeNotice     messageID = "home.organize_notice"
	msgHomeHelpTitle          messageID = "home.help.title"
	msgHomeHelpIntroFirst     messageID = "home.help.intro_first"
	msgHomeHelpIntroSecond    messageID = "home.help.intro_second"
	msgHomeHelpOrganize       messageID = "home.help.organize"
	msgHomeHelpApply          messageID = "home.help.apply"
	msgHomeHelpHealth         messageID = "home.help.health"
	msgHomeHelpMigration      messageID = "home.help.migration"
	msgHomeHelpVersion        messageID = "home.help.version"
	msgHomeHelpLibrary        messageID = "home.help.library"
	msgHomeHelpFooter         messageID = "home.help.footer"
)

var messageCatalogs = map[language]map[messageID]string{
	languageZhCN:    messagesZhCN,
	languageEnglish: messagesEnglish,
}

func (m Model) text(id messageID, args ...any) string {
	template, ok := messageCatalogs[m.language][id]
	if !ok {
		template, ok = messagesEnglish[id]
	}
	if !ok {
		return fmt.Sprintf("[missing:%s]", id)
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// withLanguage is deliberately package-private until the Settings screen and
// persisted UI preferences are implemented. It lets tests exercise every
// catalog without exposing an incomplete product setting.
func (m Model) withLanguage(value language) Model {
	m.language = value
	return m
}
