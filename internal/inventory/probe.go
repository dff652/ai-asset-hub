package inventory

// probePath is one relative tree or file scanned under a logical root.
type probePath struct {
	// Relative is a path under home or project (forward slashes).
	Relative string
	// Roots limits where this probe applies. Empty means both home and project.
	Roots []RootID
}

// scanProbes is the minimal Phase 0/2C.1 path table (not a full pluggable Probe
// interface yet). Paths are relative to a scan root (home or project).
// Classification and exclusion policy still live in classify.go; per-tool
// exclusion co-location can land when a third harness needs the same treatment.
var scanProbes = []probePath{
	// Shared T0
	{Relative: ".agents"},

	// Claude Code
	{Relative: ".claude"},
	{Relative: ".claude.json", Roots: []RootID{RootHome}},
	{Relative: "CLAUDE.md", Roots: []RootID{RootProject}},

	// Codex
	{Relative: ".codex"},
	{Relative: "AGENTS.md", Roots: []RootID{RootProject}},

	// Grok Build (read-only inventory; no apply side effects)
	{Relative: ".grok"},

	// Project-level MCP and TS-platform style setup
	{Relative: ".mcp.json", Roots: []RootID{RootProject}},
	{Relative: "scripts/claude-setup", Roots: []RootID{RootProject}},
}

func probesFor(rootID RootID) []string {
	out := make([]string, 0, len(scanProbes))
	for _, probe := range scanProbes {
		if len(probe.Roots) == 0 {
			out = append(out, probe.Relative)
			continue
		}
		for _, allowed := range probe.Roots {
			if allowed == rootID {
				out = append(out, probe.Relative)
				break
			}
		}
	}
	return out
}
