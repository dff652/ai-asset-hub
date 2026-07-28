package apply

import "strings"

// RollbackCommand returns a copyable CLI command for one deployment backup.
func RollbackCommand(options RollbackOptions) string {
	parts := []string{"aiah", "rollback"}
	if options.Home != "" {
		parts = append(parts, "--home", shellQuote(options.Home))
	}
	if options.Project != "" {
		parts = append(parts, "--project", shellQuote(options.Project))
	}
	parts = append(parts, "--backup", shellQuote(options.BackupID))
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && !strings.ContainsRune("_-./:", r)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
