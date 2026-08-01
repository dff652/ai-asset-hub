package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/version"
)

// ErrInitBlocked means the scaffold was refused and nothing was written.
var ErrInitBlocked = errors.New("workspace init blocked")

// DefaultInitVersion is the starting manifest version. It is a fixed string
// rather than today's date so two runs of init produce identical output.
const DefaultInitVersion = "0.1.0"

// DefaultInitProfile is the one profile the scaffold declares.
const DefaultInitProfile = "personal"

// assetContainers are the directories the adapter actually reads. The scaffold
// creates exactly these, so "where do I put this file?" has one answer per
// asset type and no directory exists that nothing consumes.
var assetContainers = []string{"agents", "hooks", "mcp", "rules", "skills"}

// namePattern mirrors manifest.schema.json's constraint on `name`.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// versionPattern is stricter than the schema, which only requires a non-empty
// string. Two reasons. The value is emitted into a double-quoted YAML scalar,
// so a quote or newline would produce a manifest that does not parse -- and a
// scaffold whose output fails the next command is worse than no scaffold. It
// also becomes a path segment and a filename component downstream
// (packages/<name>/<version>/<profile>/), where internal/channel already
// enforces exactly this character set; rejecting it here fails fast instead of
// three commands later at publish. The pattern is duplicated rather than
// imported because channel depends on build, which depends on this package.
var versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// InitOptions describes one scaffold request.
type InitOptions struct {
	// Directory is the workspace root. It is created if missing.
	Directory string
	// Home and Project identify managed AI-tool directories that the workspace
	// must not overlap. The CLI supplies both from the current environment.
	Home    string
	Project string
	// Name defaults to the directory's base name, normalized.
	Name string
	// Version defaults to DefaultInitVersion.
	Version string
}

// InitReport says what the scaffold created and what it left alone.
type InitReport struct {
	SchemaVersion int    `json:"schemaVersion"`
	Kind          string `json:"kind"`
	ProducedBy    string `json:"producedBy"`
	Ok            bool   `json:"ok"`
	Root          string `json:"root"`
	Manifest      string `json:"manifest"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	// Created and Existing are workspace-relative and sorted. Re-running init
	// reports everything under Existing and creates nothing.
	Created  []string `json:"created"`
	Existing []string `json:"existing"`
}

// Init scaffolds an asset workspace: the manifest plus one directory per asset
// type the adapter reads.
//
// It is create-only. An existing manifest is never rewritten -- the user's
// manifest is the source of truth for their whole library, and a scaffold has
// nothing worth losing it for. Re-running is therefore safe and idempotent:
// missing directories are filled in, everything present is reported as-is.
//
// It deliberately does not register the workspace anywhere or make later
// commands able to find it. Manifest discovery stays explicit (`--manifest`),
// so there is no hidden state deciding which library a command operates on.
func Init(options InitOptions) (InitReport, error) {
	report := InitReport{
		SchemaVersion: 1,
		Kind:          "init",
		ProducedBy:    version.ProducedBy(),
		Created:       []string{},
		Existing:      []string{},
	}

	if strings.TrimSpace(options.Directory) == "" {
		return report, fmt.Errorf("%w: a workspace directory is required", ErrInitBlocked)
	}
	root, err := validateRootCandidate(options.Directory, options.Home, options.Project)
	if err != nil {
		return report, fmt.Errorf("%w: %v", ErrInitBlocked, err)
	}
	report.Root = root

	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = normalizeName(filepath.Base(root))
		if name == "" {
			return report, fmt.Errorf(
				"%w: cannot derive a manifest name from %q; pass --name",
				ErrInitBlocked, filepath.Base(root))
		}
	}
	if !namePattern.MatchString(name) {
		return report, fmt.Errorf(
			"%w: name %q must match %s", ErrInitBlocked, name, namePattern.String())
	}
	report.Name = name

	manifestVersion := strings.TrimSpace(options.Version)
	if manifestVersion == "" {
		manifestVersion = DefaultInitVersion
	}
	if !versionPattern.MatchString(manifestVersion) {
		return report, fmt.Errorf(
			"%w: version %q must match %s", ErrInitBlocked, manifestVersion, versionPattern.String())
	}
	report.Version = manifestVersion
	report.Profile = DefaultInitProfile

	// Every path is checked before anything is written, so a refusal cannot
	// leave a half-built workspace behind.
	// Checked outermost first. Probing a container before its parent would hit
	// ENOTDIR and report the container, when the thing the user has to fix is
	// the parent.
	if err := checkDirectory(root); err != nil {
		return report, err
	}
	if err := checkDirectory(filepath.Join(root, "assets")); err != nil {
		return report, err
	}
	for _, container := range assetContainers {
		if err := checkDirectory(filepath.Join(root, "assets", container)); err != nil {
			return report, err
		}
	}
	manifestPath := filepath.Join(root, "manifest.yaml")
	report.Manifest = manifestPath
	manifestExists, err := regularFileExists(manifestPath)
	if err != nil {
		return report, err
	}

	created, existing, err := createDirectories(root)
	if err != nil {
		return report, err
	}
	report.Created, report.Existing = created, existing

	if manifestExists {
		report.Existing = append(report.Existing, "manifest.yaml")
	} else {
		if err := writeManifest(manifestPath, name, manifestVersion); err != nil {
			return report, err
		}
		report.Created = append(report.Created, "manifest.yaml")
	}
	sort.Strings(report.Created)
	sort.Strings(report.Existing)

	report.Ok = true
	return report, nil
}

// checkDirectory refuses a path occupied by anything that is not a directory.
// Creating "assets/skills" over a regular file or a symlink would either fail
// mid-run or quietly write through the link.
func checkDirectory(candidate string) error {
	info, err := os.Lstat(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: cannot inspect %s", ErrInitBlocked, candidate)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf(
			"%w: %s is a symlink; replace it with a real directory before init",
			ErrInitBlocked, candidate)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s exists and is not a directory", ErrInitBlocked, candidate)
	}
	return nil
}

func regularFileExists(candidate string) (bool, error) {
	info, err := os.Lstat(candidate)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: cannot inspect manifest.yaml", ErrInitBlocked)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf(
			"%w: manifest.yaml is a symlink; replace it with a real file before init",
			ErrInitBlocked)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%w: manifest.yaml is not a regular file", ErrInitBlocked)
	}
	return true, nil
}

func createDirectories(root string) ([]string, []string, error) {
	created := make([]string, 0, len(assetContainers)+2)
	existing := make([]string, 0, len(assetContainers)+2)

	for _, relative := range append([]string{".", "assets"}, assetPaths()...) {
		full := filepath.Join(root, filepath.FromSlash(relative))
		_, err := os.Stat(full)
		switch {
		case errors.Is(err, os.ErrNotExist):
			if err := os.MkdirAll(full, 0o755); err != nil {
				return nil, nil, fmt.Errorf("%w: cannot create %s", ErrInitBlocked, relative)
			}
			if relative != "." {
				created = append(created, relative+"/")
			}
		case err != nil:
			return nil, nil, fmt.Errorf("%w: cannot inspect %s", ErrInitBlocked, relative)
		default:
			if relative != "." {
				existing = append(existing, relative+"/")
			}
		}
	}
	return created, existing, nil
}

func assetPaths() []string {
	paths := make([]string, 0, len(assetContainers))
	for _, container := range assetContainers {
		paths = append(paths, "assets/"+container)
	}
	return paths
}

// writeManifest stages the manifest and renames it into place, so an
// interrupted init never leaves a partial manifest that validate would reject.
func writeManifest(manifestPath, name, manifestVersion string) error {
	body := manifestTemplate(name, manifestVersion)
	temporary, err := os.CreateTemp(filepath.Dir(manifestPath), ".manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("%w: cannot stage manifest.yaml", ErrInitBlocked)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	if _, err := temporary.WriteString(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%w: cannot stage manifest.yaml", ErrInitBlocked)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%w: cannot stage manifest.yaml", ErrInitBlocked)
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		return fmt.Errorf("%w: cannot set manifest.yaml permissions", ErrInitBlocked)
	}
	if err := os.Rename(temporaryPath, manifestPath); err != nil {
		return fmt.Errorf("%w: cannot install manifest.yaml", ErrInitBlocked)
	}
	return nil
}

// manifestTemplate produces a manifest that validates as written. An empty
// asset list is a valid v1 document, so `aiah validate` succeeds immediately
// after init and the first failure a user sees is about their own content.
func manifestTemplate(name, manifestVersion string) string {
	var builder strings.Builder
	builder.WriteString("schemaVersion: 1\n")
	builder.WriteString("name: " + name + "\n")
	builder.WriteString("version: \"" + manifestVersion + "\"\n\n")
	builder.WriteString("# Every asset needs four decisions:\n")
	builder.WriteString("#   targets      which tools it installs to (claude, codex, grok)\n")
	builder.WriteString("#   scope        global writes HOME, project writes a project root,\n")
	builder.WriteString("#                device is never applied\n")
	builder.WriteString("#   portability  portable installs as-is, adapter-required is compiled per target\n")
	builder.WriteString("#   sensitivity  sensitive values must be ${ENV:NAME} or ${secret:path} references\n")
	builder.WriteString("assets: []\n")
	builder.WriteString("#  - id: skill.review\n")
	builder.WriteString("#    type: skill\n")
	builder.WriteString("#    path: assets/skills/review\n")
	builder.WriteString("#    targets: [claude, codex]\n")
	builder.WriteString("#    scope: global\n")
	builder.WriteString("#    portability: adapter-required\n")
	builder.WriteString("#    sensitivity: private\n\n")
	builder.WriteString("profiles:\n")
	builder.WriteString("  " + DefaultInitProfile + ":\n")
	builder.WriteString("    include: []\n")
	return builder.String()
}

// normalizeName maps a directory name onto the manifest's name pattern:
// lowercase, with every unsupported run collapsed to a single separator.
func normalizeName(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	previousSeparator := false
	for _, character := range lowered {
		switch {
		case (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9'):
			builder.WriteRune(character)
			previousSeparator = false
		case character == '.' || character == '_' || character == '-':
			if !previousSeparator && builder.Len() > 0 {
				builder.WriteRune('-')
				previousSeparator = true
			}
		default:
			if !previousSeparator && builder.Len() > 0 {
				builder.WriteRune('-')
				previousSeparator = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
