package readiness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dff652/ai-asset-hub/internal/content"
	"github.com/dff652/ai-asset-hub/internal/workspace"
	"gopkg.in/yaml.v3"
)

const maxEvidenceFileSize = 1 << 20

var errInvalidEvidence = errors.New("invalid evidence")

var targetIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type evidenceSubject struct {
	Name            string `yaml:"name"`
	Version         string `yaml:"version"`
	Profile         string `yaml:"profile"`
	SelectionSHA256 string `yaml:"selectionSHA256"`
}

type backupEvidenceDocument struct {
	SchemaVersion int             `yaml:"schemaVersion"`
	Kind          string          `yaml:"kind"`
	Subject       evidenceSubject `yaml:"subject"`
	Copy          struct {
		Type      string `yaml:"type"`
		Reference string `yaml:"reference"`
	} `yaml:"copy"`
	RecordedAt string `yaml:"recordedAt"`
}

type restoreExerciseDocument struct {
	SchemaVersion int    `yaml:"schemaVersion"`
	Kind          string `yaml:"kind"`
	Subject       struct {
		Name            string `yaml:"name"`
		Version         string `yaml:"version"`
		Profile         string `yaml:"profile"`
		SelectionSHA256 string `yaml:"selectionSHA256"`
		PackageSHA256   string `yaml:"packageSHA256"`
	} `yaml:"subject"`
	Targets  []string `yaml:"targets"`
	Isolated struct {
		Home    bool `yaml:"home"`
		Project bool `yaml:"project"`
	} `yaml:"isolated"`
	Steps struct {
		Pull      exerciseStep `yaml:"pull"`
		Preflight exerciseStep `yaml:"preflight"`
		Diff      exerciseStep `yaml:"diff"`
		Apply     exerciseStep `yaml:"apply"`
		Doctor    exerciseStep `yaml:"doctor"`
		Rollback  exerciseStep `yaml:"rollback"`
	} `yaml:"steps"`
	ProducedBy  string `yaml:"producedBy"`
	CompletedAt string `yaml:"completedAt"`
}

type exerciseStep struct {
	Status string `yaml:"status"`
}

func inspectBackupEvidence(root, path string, subject Subject) (BackupEvidence, error) {
	status := BackupEvidence{Status: StatusMissing}
	if path == "" {
		return status, nil
	}
	var document backupEvidenceDocument
	if err := readEvidence(root, path, &document); err != nil {
		status.Status = StatusInvalid
		return status, nil
	}
	if !validBackupEvidence(document) {
		status.Status = StatusInvalid
		return status, nil
	}
	status.Type = document.Copy.Type
	status.RecordedAt = document.RecordedAt
	sum := sha256.Sum256([]byte(document.Copy.Reference))
	status.ReferenceDigest = hex.EncodeToString(sum[:6])
	if !subjectMatches(document.Subject, subject) {
		status.Status = StatusMismatch
		return status, nil
	}
	status.Status = StatusRecorded
	return status, nil
}

func inspectRestoreExercise(
	root, path string,
	subject Subject,
	expectedTargets []string,
) (RestoreExercise, error) {
	status := RestoreExercise{Status: StatusMissing, Targets: []string{}}
	if path == "" {
		return status, nil
	}
	var document restoreExerciseDocument
	if err := readEvidence(root, path, &document); err != nil {
		status.Status = StatusInvalid
		return status, nil
	}
	allPassed, valid := validRestoreExercise(document)
	if !valid {
		status.Status = StatusInvalid
		return status, nil
	}
	status.PackageSHA256 = document.Subject.PackageSHA256
	status.Targets = append([]string(nil), document.Targets...)
	status.ProducedBy = document.ProducedBy
	status.CompletedAt = document.CompletedAt
	documentSubject := evidenceSubject{
		Name:            document.Subject.Name,
		Version:         document.Subject.Version,
		Profile:         document.Subject.Profile,
		SelectionSHA256: document.Subject.SelectionSHA256,
	}
	if !subjectMatches(documentSubject, subject) {
		status.Status = StatusMismatch
		return status, nil
	}
	if !sameTargets(document.Targets, expectedTargets) {
		status.Status = StatusMismatch
		return status, nil
	}
	if !allPassed {
		status.Status = StatusFailed
		return status, nil
	}
	status.Status = StatusPassed
	return status, nil
}

func sameTargets(actual, expected []string) bool {
	actual = append([]string(nil), actual...)
	expected = append([]string(nil), expected...)
	sort.Strings(actual)
	sort.Strings(expected)
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func readEvidence(root, path string, out any) error {
	absolute, err := secureEvidenceFile(root, path)
	if err != nil {
		return errInvalidEvidence
	}
	expected, err := os.Lstat(absolute)
	if err != nil || !expected.Mode().IsRegular() || expected.Mode()&os.ModeSymlink != 0 {
		return errInvalidEvidence
	}
	file, err := os.Open(absolute)
	if err != nil {
		return errInvalidEvidence
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(expected, info) ||
		info.Size() <= 0 || info.Size() > maxEvidenceFileSize {
		return errInvalidEvidence
	}
	body, err := io.ReadAll(io.LimitReader(file, maxEvidenceFileSize+1))
	if err != nil || len(body) == 0 || len(body) > maxEvidenceFileSize || content.ContainsSecret(body) {
		return errInvalidEvidence
	}
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return errInvalidEvidence
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errInvalidEvidence
	}
	return nil
}

func secureEvidenceFile(root, value string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", errInvalidEvidence
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", errInvalidEvidence
	}
	evidenceRoot := filepath.Join(root, ".aiah", "evidence")
	candidate := value
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Clean(filepath.FromSlash(candidate))
		prefix := filepath.Join(".aiah", "evidence")
		if candidate == prefix || strings.HasPrefix(candidate, prefix+string(filepath.Separator)) {
			candidate = filepath.Join(root, candidate)
		} else {
			candidate = filepath.Join(evidenceRoot, candidate)
		}
	}
	candidate = filepath.Clean(candidate)
	if candidate == evidenceRoot || !workspace.Within(evidenceRoot, candidate) {
		return "", errInvalidEvidence
	}

	relative, err := filepath.Rel(root, candidate)
	if err != nil || !workspace.SafeRelativePath(relative) {
		return "", errInvalidEvidence
	}
	current := root
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for index, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return "", errInvalidEvidence
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errInvalidEvidence
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", errInvalidEvidence
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return "", errInvalidEvidence
		}
	}
	return candidate, nil
}

func validBackupEvidence(document backupEvidenceDocument) bool {
	if document.SchemaVersion != 1 || document.Kind != "backup-evidence" ||
		!validEvidenceSubject(document.Subject) ||
		!oneOf(document.Copy.Type, "git-commit", "snapshot", "object", "offline-media") ||
		!validText(document.Copy.Reference, 1024) || !validTimestamp(document.RecordedAt) {
		return false
	}
	if parsed, err := url.Parse(document.Copy.Reference); err == nil && parsed.User != nil {
		return false
	}
	return true
}

func validRestoreExercise(document restoreExerciseDocument) (bool, bool) {
	subject := evidenceSubject{
		Name:            document.Subject.Name,
		Version:         document.Subject.Version,
		Profile:         document.Subject.Profile,
		SelectionSHA256: document.Subject.SelectionSHA256,
	}
	if document.SchemaVersion != 1 || document.Kind != "restore-exercise" ||
		!validEvidenceSubject(subject) || !validSHA256(document.Subject.PackageSHA256) ||
		!document.Isolated.Home || !document.Isolated.Project ||
		!validText(document.ProducedBy, 256) || !validTimestamp(document.CompletedAt) ||
		len(document.Targets) == 0 {
		return false, false
	}
	targets := append([]string(nil), document.Targets...)
	sort.Strings(targets)
	for index, target := range targets {
		if !targetIDPattern.MatchString(target) ||
			(index > 0 && target == targets[index-1]) {
			return false, false
		}
	}
	allPassed := true
	for _, step := range []exerciseStep{
		document.Steps.Pull,
		document.Steps.Preflight,
		document.Steps.Diff,
		document.Steps.Apply,
		document.Steps.Doctor,
		document.Steps.Rollback,
	} {
		if !oneOf(step.Status, StatusPassed, StatusFailed) {
			return false, false
		}
		if step.Status != StatusPassed {
			allPassed = false
		}
	}
	return allPassed, true
}

func validEvidenceSubject(subject evidenceSubject) bool {
	return validText(subject.Name, 128) && validText(subject.Version, 128) &&
		validText(subject.Profile, 128) && validSHA256(subject.SelectionSHA256)
}

func subjectMatches(evidence evidenceSubject, current Subject) bool {
	return evidence.Name == current.Name && evidence.Version == current.Version &&
		evidence.Profile == current.Profile && evidence.SelectionSHA256 == current.SelectionSHA256
}

func validText(value string, maxLength int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maxLength &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validTimestamp(value string) bool {
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
