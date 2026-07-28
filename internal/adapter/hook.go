package adapter

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/content"
)

// HookPolicy is the install decision for one staged hook path.
type HookPolicy struct {
	Mode      uint32
	Notes     []string // non-fatal
	Lifecycle string   // best-effort lifecycle label when known
}

// EvaluateHookPolicy validates hook content and chooses install mode.
// Scripts get +x; JSON/config hooks stay 0644. Raw secrets and binary fail closed.
func EvaluateHookPolicy(relPath string, body []byte) (HookPolicy, error) {
	base := path.Base(relPath)
	ext := strings.ToLower(path.Ext(base))
	trimmed := bytes.TrimSpace(body)

	if len(trimmed) == 0 {
		return HookPolicy{Mode: ModeFile}, fmt.Errorf("hook is empty")
	}
	if content.IsBinary(body) {
		return HookPolicy{Mode: ModeFile}, fmt.Errorf("hook looks binary")
	}
	if content.ContainsSecret(body) {
		return HookPolicy{Mode: ModeFile}, fmt.Errorf("hook content looks like a raw secret")
	}

	lifecycle := inferHookLifecycle(base)
	policy := HookPolicy{Mode: ModeFile, Lifecycle: lifecycle}

	// JSON/config-style hooks are registration payloads, not shell entrypoints.
	if ext == ".json" || ext == ".toml" || ext == ".yaml" || ext == ".yml" {
		policy.Notes = append(policy.Notes,
			fmt.Sprintf("%s: config-style hook installed as non-executable; tool-native registration/trust may still be required", relPath))
		return policy, nil
	}

	// Script-like hooks: require shebang and install executable.
	scriptExt := ext == ".sh" || ext == ".bash" || ext == ".zsh" ||
		ext == ".py" || ext == ".js" || ext == ".mjs" || ext == ".ts" ||
		ext == ".rb" || ext == ".pl"
	hasShebang := bytes.HasPrefix(trimmed, []byte("#!"))

	if scriptExt || hasShebang {
		if !hasShebang {
			return HookPolicy{
				Mode:      ModeFile,
				Lifecycle: lifecycle,
			}, fmt.Errorf("script hook missing shebang (#!)")
		}
		policy.Mode = ModeExecutable
		note := fmt.Sprintf("%s: will install executable (0755)", relPath)
		if lifecycle != "" {
			note += "; lifecycle hint=" + lifecycle
		}
		note += "; harness may still require trust or settings registration"
		policy.Notes = append(policy.Notes, note)
		return policy, nil
	}

	// Other text under hooks/: non-executable with lifecycle note.
	policy.Notes = append(policy.Notes,
		fmt.Sprintf("%s: installed non-executable; unknown hook format for automatic +x", relPath))
	return policy, nil
}

// IsHookInstallPath reports tool hook destinations (.claude/hooks/..., etc.).
func IsHookInstallPath(rel string) bool {
	_, section, ok := toolInstallPath(rel)
	return ok && section == "hooks"
}

// inferHookLifecycle maps common filenames to a portable lifecycle hint.
// This is advisory only — harness event models differ (ADR-0002 hook.lifecycle).
func inferHookLifecycle(base string) string {
	name := strings.ToLower(strings.TrimSuffix(base, path.Ext(base)))
	name = strings.ReplaceAll(name, "_", "-")
	switch {
	case strings.Contains(name, "pre-tool"), strings.Contains(name, "pretool"),
		strings.Contains(name, "before-tool"):
		return "PreToolUse"
	case strings.Contains(name, "post-tool"), strings.Contains(name, "posttool"),
		strings.Contains(name, "after-tool"):
		return "PostToolUse"
	case strings.Contains(name, "session-start"), strings.Contains(name, "sessionstart"):
		return "SessionStart"
	case strings.Contains(name, "session-end"), strings.Contains(name, "sessionend"):
		return "SessionEnd"
	case strings.Contains(name, "user-prompt"), strings.Contains(name, "userprompt"),
		strings.Contains(name, "prompt-submit"):
		return "UserPromptSubmit"
	case name == "stop":
		return "Stop"
	case name == "subagent-start":
		return "SubagentStart"
	case name == "subagent-stop":
		return "SubagentStop"
	default:
		return ""
	}
}

// AnnotateHookPolicies sets Mode on staged hook files and collects notes/errors.
// Non-hook files are returned unchanged.
func AnnotateHookPolicies(staged []StagedFile) ([]StagedFile, []string, error) {
	out := make([]StagedFile, len(staged))
	copy(out, staged)
	notes := make([]string, 0)
	for i := range out {
		if !IsHookInstallPath(out[i].RelPath) {
			continue
		}
		policy, err := EvaluateHookPolicy(out[i].RelPath, out[i].Body)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", out[i].RelPath, err)
		}
		out[i].Mode = policy.Mode
		notes = append(notes, policy.Notes...)
	}
	return out, notes, nil
}
