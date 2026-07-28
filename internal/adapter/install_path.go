package adapter

import "strings"

// toolInstallPath recognizes staged files below one harness-owned section.
func toolInstallPath(rel string) (target, section string, ok bool) {
	parts := strings.Split(rel, "/")
	if len(parts) < 3 {
		return "", "", false
	}
	switch parts[0] {
	case ".claude":
		target = TargetClaude
	case ".codex":
		target = TargetCodex
	case ".grok":
		target = TargetGrok
	default:
		return "", "", false
	}
	return target, parts[1], true
}
