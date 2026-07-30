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

	msgCommonListSeparator     messageID = "common.list_separator"
	msgCommonFilterPlaceholder messageID = "common.filter_placeholder"
	msgCommonWriteInProgress   messageID = "common.write_in_progress"
	msgValidationFailed        messageID = "validation.failed"
	msgValidationDetails       messageID = "validation.details"

	msgInventoryTitle              messageID = "inventory.title"
	msgInventoryCountsScanned      messageID = "inventory.counts_scanned"
	msgInventoryCountsManaged      messageID = "inventory.counts_managed"
	msgInventoryWorkspaceUnset     messageID = "inventory.workspace_unset"
	msgInventoryWorkspaceSelected  messageID = "inventory.workspace_selected"
	msgInventoryFilterHint         messageID = "inventory.filter_hint"
	msgInventoryFindingsOnly       messageID = "inventory.findings_only"
	msgInventoryScanning           messageID = "inventory.scanning"
	msgInventoryScanFailed         messageID = "inventory.scan_failed"
	msgInventoryScanRetry          messageID = "inventory.scan_retry"
	msgInventoryFooterReadOnly     messageID = "inventory.footer_read_only"
	msgInventoryFooterManaged      messageID = "inventory.footer_managed"
	msgInventoryFooterDiff         messageID = "inventory.footer_diff"
	msgInventoryFooterMaintenance  messageID = "inventory.footer_maintenance"
	msgInventorySelected           messageID = "inventory.selected"
	msgInventoryEmpty              messageID = "inventory.empty"
	msgInventoryDetailSeverity     messageID = "inventory.detail.severity"
	msgInventoryDetailDescription  messageID = "inventory.detail.description"
	msgInventoryDetailPaths        messageID = "inventory.detail.paths"
	msgInventoryGroupSource        messageID = "inventory.group.source"
	msgInventoryGroupType          messageID = "inventory.group.type"
	msgInventoryGroupFindings      messageID = "inventory.group.findings"
	msgInventoryGroupLibraryStatus messageID = "inventory.group.library_status"
	msgInventoryDetailGroup        messageID = "inventory.detail.group"
	msgInventoryDetailFindings     messageID = "inventory.detail.findings"
	msgInventoryDetailNoFindings   messageID = "inventory.detail.no_findings"
	msgInventoryDetailExpand       messageID = "inventory.detail.expand"
	msgInventoryDetailAssetState   messageID = "inventory.detail.asset_state"
	msgInventoryDetailType         messageID = "inventory.detail.type"
	msgInventoryDetailLibraryPath  messageID = "inventory.detail.library_path"
	msgInventoryDetailTargets      messageID = "inventory.detail.targets"
	msgInventoryDetailRemove       messageID = "inventory.detail.remove"
	msgInventoryDetailSource       messageID = "inventory.detail.source"
	msgInventoryDetailScope        messageID = "inventory.detail.scope"
	msgInventoryDetailPortability  messageID = "inventory.detail.portability"
	msgInventoryDetailSensitivity  messageID = "inventory.detail.sensitivity"
	msgInventoryDetailStatus       messageID = "inventory.detail.status"
	msgInventoryDetailFiles        messageID = "inventory.detail.files"
	msgInventoryStateUnmanaged     messageID = "inventory.state.unmanaged"
	msgInventoryStateManaged       messageID = "inventory.state.managed"
	msgInventoryStateSourceChanged messageID = "inventory.state.source_changed"
	msgInventoryStateLibraryOnly   messageID = "inventory.state.library_only"
	msgInventoryStateBlocked       messageID = "inventory.state.blocked"
	msgInventoryLibraryOnlyGroup   messageID = "inventory.library_only_group"
	msgInventoryUnattachedGroup    messageID = "inventory.unattached_group"
	msgInventoryNotSelectable      messageID = "inventory.not_selectable"
	msgInventoryHelpTitle          messageID = "inventory.help.title"
	msgInventoryHelpLibrary        messageID = "inventory.help.library"
	msgInventoryHelpTargets        messageID = "inventory.help.targets"
	msgInventoryHelpReadOnly       messageID = "inventory.help.read_only"
	msgInventoryHelpNoDefault      messageID = "inventory.help.no_default"
	msgInventoryHelpSelect         messageID = "inventory.help.select"
	msgInventoryHelpAdd            messageID = "inventory.help.add"
	msgInventoryHelpUpdate         messageID = "inventory.help.update"
	msgInventoryHelpRemove         messageID = "inventory.help.remove"
	msgInventoryHelpPreview        messageID = "inventory.help.preview"
	msgInventoryHelpApply          messageID = "inventory.help.apply"
	msgInventoryHelpWorkspace      messageID = "inventory.help.workspace"
	msgInventoryHelpManagedTargets messageID = "inventory.help.managed_targets"
	msgInventoryHelpLibraryWrites  messageID = "inventory.help.library_writes"
	msgInventoryHelpWizard         messageID = "inventory.help.wizard"
	msgInventoryHelpBackups        messageID = "inventory.help.backups"
	msgInventoryHelpSkipped        messageID = "inventory.help.skipped"
	msgInventoryHelpDiff           messageID = "inventory.help.diff"
	msgInventoryHelpApplySafety    messageID = "inventory.help.apply_safety"
	msgInventoryHelpNoApply        messageID = "inventory.help.no_apply"
	msgInventoryHelpDoctor         messageID = "inventory.help.doctor"
	msgInventoryHelpVersion        messageID = "inventory.help.version"
	msgInventoryHelpRollback       messageID = "inventory.help.rollback"

	msgHelpMove         messageID = "help.move"
	msgHelpFirstLast    messageID = "help.first_last"
	msgHelpExpand       messageID = "help.expand"
	msgHelpCollapse     messageID = "help.collapse"
	msgHelpCollapseOnly messageID = "help.collapse_only"
	msgHelpFilter       messageID = "help.filter"
	msgHelpFindingsOnly messageID = "help.findings_only"
	msgHelpRescan       messageID = "help.rescan"
	msgHelpHome         messageID = "help.home"
	msgHelpClose        messageID = "help.close"
	msgHelpQuit         messageID = "help.quit"

	msgWorkspaceInputTitle      messageID = "workspace_input.title"
	msgWorkspaceInputDefinition messageID = "workspace_input.definition"
	msgWorkspaceInputContents   messageID = "workspace_input.contents"
	msgWorkspaceInputTargets    messageID = "workspace_input.targets"
	msgWorkspaceInputPrompt     messageID = "workspace_input.prompt"
	msgWorkspaceInputExplicit   messageID = "workspace_input.explicit"
	msgWorkspaceInputFlow       messageID = "workspace_input.flow"
	msgWorkspaceInputFooter     messageID = "workspace_input.footer"
	msgWorkspacePathRequired    messageID = "workspace.path_required"
	msgWorkspaceOpening         messageID = "workspace.opening"
	msgWorkspaceUnavailable     messageID = "workspace.unavailable"
	msgWorkspaceCreated         messageID = "workspace.created"
	msgWorkspaceOpened          messageID = "workspace.opened"
	msgWorkspaceSelectFirst     messageID = "workspace.select_first"

	msgProfileInputApplyTitle      messageID = "profile_input.apply_title"
	msgProfileInputApplyNext       messageID = "profile_input.apply_next"
	msgProfileInputPublishTitle    messageID = "profile_input.publish_title"
	msgProfileInputPublishNext     messageID = "profile_input.publish_next"
	msgProfileInputPreflightTitle  messageID = "profile_input.preflight_title"
	msgProfileInputPreflightNext   messageID = "profile_input.preflight_next"
	msgProfileInputLibrary         messageID = "profile_input.library"
	msgProfileInputPrompt          messageID = "profile_input.prompt"
	msgProfileInputFooter          messageID = "profile_input.footer"
	msgProfileInputAvailable       messageID = "profile_input.available"
	msgProfileRequired             messageID = "profile.required"
	msgProfileManifestReadFailed   messageID = "profile.manifest_read_failed"
	msgProfileComposeBusy          messageID = "profile.compose_busy"
	msgProfileBuildPublishing      messageID = "profile.build_publishing"
	msgProfileBuildPreparing       messageID = "profile.build_preparing"
	msgProfileBuildCommandFailed   messageID = "profile.build_command_failed"
	msgProfileBuildFailedFinding   messageID = "profile.build_failed_finding"
	msgProfileBuildFailedNoPackage messageID = "profile.build_failed_no_package"
	msgProfileBuildReady           messageID = "profile.build_ready"

	msgComposeWorkspaceRequired messageID = "compose.workspace_required"
	msgComposeSelectUnmanaged   messageID = "compose.select_unmanaged"
	msgComposeAdding            messageID = "compose.adding"
	msgComposeFailed            messageID = "compose.failed"
	msgComposeNoneFinding       messageID = "compose.none_finding"
	msgComposeNoneSelected      messageID = "compose.none_selected"
	msgComposeSucceeded         messageID = "compose.succeeded"
	msgComposeSkipped           messageID = "compose.skipped"

	msgManageSelectUpdate       messageID = "manage.select_update"
	msgManageSelectRemove       messageID = "manage.select_remove"
	msgManageConfirmMismatch    messageID = "manage.confirm_mismatch"
	msgManageUpdating           messageID = "manage.updating"
	msgManageRemoving           messageID = "manage.removing"
	msgManageFailed             messageID = "manage.failed"
	msgManageUpdated            messageID = "manage.updated"
	msgManageRemoved            messageID = "manage.removed"
	msgManageUpdateAction       messageID = "manage.update_action"
	msgManageUpdateWarning      messageID = "manage.update_warning"
	msgManageRemoveAction       messageID = "manage.remove_action"
	msgManageRemoveWarning      messageID = "manage.remove_warning"
	msgManageConfirmationTitle  messageID = "manage.confirmation_title"
	msgManageConfirmationPrompt messageID = "manage.confirmation_prompt"
	msgManageConfirmationFooter messageID = "manage.confirmation_footer"

	msgDeploymentHelpTitle    messageID = "deployment.help.title"
	msgDeploymentHelpApply    messageID = "deployment.help.apply"
	msgDeploymentHelpRefresh  messageID = "deployment.help.refresh"
	msgDeploymentHelpSafety   messageID = "deployment.help.safety"
	msgDeploymentHelpResult   messageID = "deployment.help.result"
	msgDeploymentHelpDoctor   messageID = "deployment.help.doctor"
	msgDeploymentHelpVersion  messageID = "deployment.help.version"
	msgHealthHelpTitle        messageID = "health.help.title"
	msgHealthHelpRerun        messageID = "health.help.rerun"
	msgHealthHelpRollback     messageID = "health.help.rollback"
	msgHealthHelpVersion      messageID = "health.help.version"
	msgHealthHelpAvailability messageID = "health.help.availability"
	msgHealthHelpTyped        messageID = "health.help.typed"
	msgVersionHelpTitle       messageID = "version.help.title"
	msgVersionHelpCheck       messageID = "version.help.check"
	msgVersionHelpHealth      messageID = "version.help.health"
	msgVersionHelpOffline     messageID = "version.help.offline"
	msgVersionHelpOnline      messageID = "version.help.online"
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
	m.syncLocalizedInputs()
	return m
}

func (m *Model) syncLocalizedInputs() {
	m.filterInput.Placeholder = m.text(msgCommonFilterPlaceholder)
}
