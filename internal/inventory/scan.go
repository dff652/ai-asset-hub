package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dff652/ai-asset-hub/internal/content"
	"github.com/dff652/ai-asset-hub/internal/version"
)

var ErrInvalidRoot = errors.New("invalid scan root")

type Options struct {
	Home    string
	Project string
}

type scanRoot struct {
	id   RootID
	path string
}

type inspection struct {
	entry  Entry
	issues []FindingCode
}

type detectedIssue struct {
	code FindingCode
	path string
}

func Scan(options Options) (Report, error) {
	home, err := validateRoot(options.Home)
	if err != nil {
		return Report{}, err
	}
	roots := []scanRoot{{id: RootHome, path: home}}
	reportRoots := []Root{{ID: RootHome, Kind: RootHome, Present: true}}

	if options.Project != "" {
		project, err := validateRoot(options.Project)
		if err != nil {
			return Report{}, err
		}
		roots = append(roots, scanRoot{id: RootProject, path: project})
		reportRoots = append(reportRoots, Root{ID: RootProject, Kind: RootProject, Present: true})
	}

	inspections := discover(roots)
	assets, entries, issues := assembleInventory(inspections)
	findings := deriveFindings(issues, entries, assets)
	summary := summarize(entries, assets, findings, len(roots) == 2)

	return Report{
		SchemaVersion: 1,
		Kind:          "inventory",
		ProducedBy:    version.ProducedBy(),
		Roots:         reportRoots,
		Summary:       summary,
		Assets:        assets,
		Entries:       entries,
		Findings:      findings,
	}, nil
}

func validateRoot(input string) (string, error) {
	if input == "" {
		return "", ErrInvalidRoot
	}
	absolute, err := filepath.Abs(input)
	if err != nil {
		return "", ErrInvalidRoot
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", ErrInvalidRoot
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", ErrInvalidRoot
	}
	return filepath.Clean(resolved), nil
}

func discover(roots []scanRoot) []inspection {
	inspections := make([]inspection, 0)
	for _, root := range roots {
		for _, relative := range probesFor(root.id) {
			inspections = append(inspections, discoverPath(root, relative)...)
		}
	}
	sort.Slice(inspections, func(i, j int) bool {
		return inspections[i].entry.LogicalPath < inspections[j].entry.LogicalPath
	})
	return inspections
}

func discoverPath(root scanRoot, relative string) []inspection {
	full := filepath.Join(root.path, filepath.FromSlash(relative))
	info, err := os.Lstat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []inspection{inaccessibleInspection(root.id, relative)}
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		if result, _ := inspectEntry(root, full, relative, info); result != nil {
			return []inspection{*result}
		}
		return nil
	}

	var inspections []inspection
	_ = filepath.WalkDir(full, func(current string, dirEntry fs.DirEntry, walkErr error) error {
		relativePath, relErr := filepath.Rel(root.path, current)
		if relErr != nil {
			return nil
		}
		logicalRelative := filepath.ToSlash(relativePath)
		if walkErr != nil {
			inspections = append(inspections, inaccessibleInspection(root.id, logicalRelative))
			if dirEntry != nil && dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if current == full {
			return nil
		}
		currentInfo, infoErr := dirEntry.Info()
		if infoErr != nil {
			inspections = append(inspections, inaccessibleInspection(root.id, logicalRelative))
			return nil
		}
		result, skipDir := inspectEntry(root, current, logicalRelative, currentInfo)
		if result != nil {
			inspections = append(inspections, *result)
		}
		if dirEntry.IsDir() && skipDir {
			return filepath.SkipDir
		}
		return nil
	})
	return inspections
}

func inspectEntry(root scanRoot, full, relative string, info fs.FileInfo) (*inspection, bool) {
	policy := policyFor(root.id, relative)
	result := &inspection{entry: Entry{
		LogicalPath: filepath.ToSlash(filepath.Join(string(root.id), relative)),
		Source:      policy.source,
		Scope:       policy.scope,
		Type:        policy.assetType,
		FileType:    fileType(info.Mode()),
		Size:        entrySize(info),
		Mode:        fmt.Sprintf("%04o", info.Mode().Perm()),
		Portability: policy.portability,
		Sensitivity: policy.sensitivity,
	}}
	entry := &result.entry

	if policy.exclusionReason != "" {
		entry.Status = EntryExcluded
		entry.ExclusionReason = policy.exclusionReason
		return result, info.IsDir()
	}
	if info.Mode()&os.ModeSymlink != 0 {
		issue := inspectSymlink(entry, root.path, full)
		if issue == "" && policy.assetType != TypeUnknown {
			// Not following the link is correct, but staying silent is not:
			// a skill installed by symlink from its source repo would vanish
			// from the inventory with nothing saying anything was skipped.
			issue = FindingSymlinkedAsset
		}
		if issue != "" {
			result.issues = append(result.issues, issue)
		}
		return result, false
	}
	if info.IsDir() {
		if info.Mode().Perm()&0444 == 0 {
			result.issues = append(result.issues, markUnreadable(entry))
			return result, true
		}
		return nil, false
	}
	if !info.Mode().IsRegular() {
		entry.Status = EntryExcluded
		entry.Portability = PortabilityExcluded
		entry.ExclusionReason = ExcludeSpecialFile
		return result, false
	}
	if info.Mode().Perm()&0444 == 0 {
		result.issues = append(result.issues, markUnreadable(entry))
		return result, false
	}
	if info.Size() > content.MaxScannedFileSize {
		entry.Status = EntryExcluded
		entry.Portability = PortabilityExcluded
		entry.ExclusionReason = ExcludeTooLarge
		result.issues = append(result.issues, FindingLargeFile)
		return result, false
	}

	body, err := os.ReadFile(full)
	if err != nil {
		result.issues = append(result.issues, markUnreadable(entry))
		return result, false
	}
	if content.IsBinary(body) {
		entry.Status = EntryExcluded
		entry.Portability = PortabilityExcluded
		entry.ExclusionReason = ExcludeBinary
		return result, false
	}
	if content.ContainsSecret(body) {
		entry.Status = EntryExcluded
		entry.Portability = PortabilityExcluded
		entry.Sensitivity = SensitivitySecret
		entry.ExclusionReason = ExcludeSuspectedSecret
		result.issues = append(result.issues, FindingSuspectedSecret)
		return result, false
	}

	sum := sha256.Sum256(body)
	entry.SHA256 = hex.EncodeToString(sum[:])
	if policy.assetType == TypeUnknown {
		entry.Status = EntryReported
	} else {
		entry.Status = EntryIncluded
	}
	result.issues = append(result.issues, inspectConfig(entry, body)...)
	return result, false
}

func inspectSymlink(entry *Entry, boundary, full string) FindingCode {
	entry.Status = EntryExcluded
	entry.Portability = PortabilityExcluded
	entry.ExclusionReason = ExcludeSymlink
	entry.Sensitivity = SensitivityPrivate

	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		entry.SymlinkState = SymlinkBroken
		entry.ExclusionReason = ExcludeBrokenSymlink
		return FindingBrokenSymlink
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		entry.SymlinkState = SymlinkBroken
		entry.ExclusionReason = ExcludeBrokenSymlink
		return FindingBrokenSymlink
	}
	if !within(boundary, resolved) {
		entry.SymlinkState = SymlinkEscape
		entry.ExclusionReason = ExcludeSymlinkEscape
		return FindingSymlinkEscape
	}
	entry.SymlinkState = SymlinkInternal
	return ""
}

func within(boundary, candidate string) bool {
	relative, err := filepath.Rel(boundary, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func inaccessibleInspection(rootID RootID, relative string) inspection {
	return inspection{entry: Entry{
		LogicalPath:     filepath.ToSlash(filepath.Join(string(rootID), relative)),
		Source:          sourceFor(rootID, relative),
		Scope:           scopeFor(rootID),
		Type:            TypeUnknown,
		FileType:        FileUnknown,
		Size:            0,
		Mode:            "0000",
		Portability:     PortabilityExcluded,
		Sensitivity:     SensitivityUnknown,
		Status:          EntryUnreadable,
		ExclusionReason: ExcludeUnreadable,
	}, issues: []FindingCode{FindingUnreadable}}
}

func markUnreadable(entry *Entry) FindingCode {
	entry.Status = EntryUnreadable
	entry.Portability = PortabilityExcluded
	entry.ExclusionReason = ExcludeUnreadable
	return FindingUnreadable
}

func entrySize(info fs.FileInfo) int64 {
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 0
	}
	return info.Size()
}

func fileType(mode fs.FileMode) FileType {
	switch {
	case mode&os.ModeSymlink != 0:
		return FileSymlink
	case mode.IsDir():
		return FileDirectory
	case mode.IsRegular():
		return FileRegular
	default:
		return FileSpecial
	}
}
