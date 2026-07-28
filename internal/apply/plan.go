package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"

	"github.com/dff652/ai-asset-hub/internal/adapter"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

type plannedFile struct {
	file    adapter.StagedFile
	root    string // absolute install root
	logical string // home/rel or project/rel
	abs     string // absolute destination
	action  string // create | update | unchanged
}

func planInstall(file adapter.StagedFile, home, project string) (plannedFile, *workspace.Finding) {
	scope := file.Scope
	if scope == "" {
		scope = "global"
	}
	switch scope {
	case "device":
		f := workspace.Finding{
			Code:     codeDeviceScopeBlocked,
			Severity: workspace.SeverityError,
			Message:  "Device-scoped assets are never applied.",
			Paths:    []string{file.AssetID, file.RelPath},
		}
		return plannedFile{}, &f
	case "project":
		if project == "" {
			f := workspace.Finding{
				Code:     codeMissingProjectRoot,
				Severity: workspace.SeverityError,
				Message:  "Project-scoped asset requires --project.",
				Paths:    []string{file.AssetID, file.RelPath},
			}
			return plannedFile{}, &f
		}
		return finalizePlan(file, project, "project/"+file.RelPath)
	case "global":
		if home == "" {
			f := workspace.Finding{
				Code:     codeMissingHomeRoot,
				Severity: workspace.SeverityError,
				Message:  "Global asset requires --home.",
				Paths:    []string{file.AssetID, file.RelPath},
			}
			return plannedFile{}, &f
		}
		return finalizePlan(file, home, "home/"+file.RelPath)
	default:
		f := workspace.Finding{
			Code:     codeUnknownScope,
			Severity: workspace.SeverityError,
			Message:  "Asset scope is not supported for apply.",
			Paths:    []string{file.AssetID, scope},
		}
		return plannedFile{}, &f
	}
}

func finalizePlan(file adapter.StagedFile, root, logical string) (plannedFile, *workspace.Finding) {
	abs, err := securePath(root, file.RelPath)
	if err != nil {
		f := workspace.Finding{
			Code:     codePathEscape,
			Severity: workspace.SeverityError,
			Message:  "Install path escapes the install root.",
			Paths:    []string{file.AssetID, file.RelPath},
		}
		// A symlink sitting where a file goes is the common case of a skill
		// installed by linking its source repo. Refusing to write through it is
		// right; reporting it as a path escape tells the user nothing they can
		// act on.
		if errors.Is(err, errSymlinkInPath) {
			f.Code = codeSymlinkTarget
			f.Message = "Install path contains an existing symlink and is never written through. " +
				"Replace it with a real file or directory, or remove it, then apply again."
		}
		return plannedFile{}, &f
	}
	return plannedFile{
		file:    file,
		root:    root,
		logical: logical,
		abs:     abs,
	}, nil
}

// collapsePlans rejects conflicting content for the same logical path.
// Identical hashes are treated as a single plan entry.
func collapsePlans(planned []plannedFile) ([]plannedFile, []workspace.Finding) {
	byLogical := make(map[string]plannedFile, len(planned))
	findings := make([]workspace.Finding, 0)
	for _, item := range planned {
		prev, ok := byLogical[item.logical]
		if !ok {
			byLogical[item.logical] = item
			continue
		}
		if prev.file.SHA256 != "" && item.file.SHA256 != "" && prev.file.SHA256 == item.file.SHA256 {
			continue
		}
		if string(prev.file.Body) == string(item.file.Body) {
			continue
		}
		findings = append(findings, workspace.Finding{
			Code:     codePathCollision,
			Severity: workspace.SeverityError,
			Message:  "Multiple assets map to the same install path with different content.",
			Paths:    []string{item.logical, prev.file.AssetID, item.file.AssetID},
		})
	}
	if len(findings) > 0 {
		return nil, findings
	}
	out := make([]plannedFile, 0, len(byLogical))
	for _, item := range byLogical {
		out = append(out, item)
	}
	return out, nil
}

func classifyChange(abs string, body []byte, mode uint32) (string, error) {
	existing, err := os.ReadFile(abs)
	if errors.Is(err, os.ErrNotExist) {
		return actionCreate, nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(existing)
	want := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != hex.EncodeToString(want[:]) {
		return actionUpdate, nil
	}
	// Content matches; still update when permission bits differ (e.g. hook +x).
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	wantMode := adapter.FileMode(mode)
	if info.Mode().Perm() != os.FileMode(wantMode).Perm() {
		return actionUpdate, nil
	}
	return actionUnchanged, nil
}

func logicalPath(file adapter.StagedFile) string {
	if file.Scope == "project" {
		return "project/" + file.RelPath
	}
	return "home/" + file.RelPath
}

func rootKind(logical string) string {
	if len(logical) >= 8 && logical[:8] == "project/" {
		return "project"
	}
	return "home"
}
