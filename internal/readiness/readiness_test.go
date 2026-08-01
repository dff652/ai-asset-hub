package readiness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestInspectReportsAttentionWithoutEvidenceAndWritesNothing(t *testing.T) {
	workspaceRoot := copyWorkspaceFixture(t)
	home := t.TempDir()
	project := t.TempDir()
	before := map[string]map[string]string{
		"workspace": snapshotReadinessTree(t, workspaceRoot),
		"home":      snapshotReadinessTree(t, home),
		"project":   snapshotReadinessTree(t, project),
	}

	report, err := Inspect(Options{
		WorkspaceRoot: workspaceRoot,
		Profile:       "personal",
		Home:          home,
		Project:       project,
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !report.Ok || report.Level != LevelAttention {
		t.Fatalf("status = ok:%v level:%s, want attention", report.Ok, report.Level)
	}
	if report.PackageReadiness.Status != StatusReady ||
		report.PackageReadiness.AssetCount != 2 || report.PackageReadiness.FileCount != 2 {
		t.Fatalf("package readiness = %#v", report.PackageReadiness)
	}
	if report.MigrationPreflight.Status != StatusReady ||
		!reflect.DeepEqual(report.MigrationPreflight.Targets, []string{"claude", "codex", "shared"}) {
		t.Fatalf("migration preflight = %#v", report.MigrationPreflight)
	}
	if len(report.Subject.SelectionSHA256) != 64 ||
		report.BackupEvidence.Status != StatusMissing ||
		report.RestoreExercise.Status != StatusMissing {
		t.Fatalf("unexpected subject/evidence: %#v %#v %#v",
			report.Subject, report.BackupEvidence, report.RestoreExercise)
	}
	for name, root := range map[string]string{
		"workspace": workspaceRoot,
		"home":      home,
		"project":   project,
	} {
		if after := snapshotReadinessTree(t, root); !reflect.DeepEqual(before[name], after) {
			t.Fatalf("%s changed during read-only inspection\nbefore=%#v\nafter=%#v", name, before[name], after)
		}
	}
}

func TestInspectValidEvidenceReachesReadyWithoutLeakingReference(t *testing.T) {
	workspaceRoot := copyWorkspaceFixture(t)
	home := t.TempDir()
	project := t.TempDir()
	base := inspectFixture(t, workspaceRoot, home, project, "", "")
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}
	reference := "git-commit:0123456789abcdef"
	writeEvidence(t, filepath.Join(evidenceRoot, "backup.yaml"), backupEvidenceYAML(
		base.Subject, base.Subject.SelectionSHA256, reference,
	))
	writeEvidence(t, filepath.Join(evidenceRoot, "restore.yaml"), restoreEvidenceYAML(
		base.Subject, base.Subject.SelectionSHA256, StatusPassed,
	))
	before := snapshotReadinessTree(t, workspaceRoot)

	report := inspectFixture(t, workspaceRoot, home, project, "backup.yaml", "restore.yaml")
	if !report.Ok || report.Level != LevelReady ||
		report.BackupEvidence.Status != StatusRecorded ||
		report.RestoreExercise.Status != StatusPassed {
		t.Fatalf("ready report = %#v", report)
	}
	if report.BackupEvidence.ReferenceDigest == "" {
		t.Fatal("backup reference digest is empty")
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(body), reference) {
		t.Fatal("report leaked the full external-copy reference")
	}
	if after := snapshotReadinessTree(t, workspaceRoot); !reflect.DeepEqual(before, after) {
		t.Fatal("readiness inspection modified evidence or workspace files")
	}
}

func TestInspectDistinguishesMismatchFailureAndInvalidEvidence(t *testing.T) {
	workspaceRoot := copyWorkspaceFixture(t)
	home := t.TempDir()
	project := t.TempDir()
	base := inspectFixture(t, workspaceRoot, home, project, "", "")
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	writeEvidence(t, filepath.Join(evidenceRoot, "mismatch.yaml"), backupEvidenceYAML(
		base.Subject, strings.Repeat("0", 64), "snapshot:fixture",
	))
	writeEvidence(t, filepath.Join(evidenceRoot, "failed.yaml"), restoreEvidenceYAML(
		base.Subject, base.Subject.SelectionSHA256, StatusFailed,
	))
	wrongTargets := bytes.Replace(
		restoreEvidenceYAML(base.Subject, base.Subject.SelectionSHA256, StatusPassed),
		[]byte("targets: [claude, codex, shared]"),
		[]byte("targets: [claude]"),
		1,
	)
	writeEvidence(t, filepath.Join(evidenceRoot, "wrong-targets.yaml"), wrongTargets)
	unknown := append(
		backupEvidenceYAML(base.Subject, base.Subject.SelectionSHA256, "snapshot:fixture"),
		[]byte("unknown: true\n")...,
	)
	writeEvidence(t, filepath.Join(evidenceRoot, "unknown.yaml"), unknown)

	report := inspectFixture(t, workspaceRoot, home, project, "mismatch.yaml", "failed.yaml")
	if report.Level != LevelAttention || report.BackupEvidence.Status != StatusMismatch ||
		report.RestoreExercise.Status != StatusFailed {
		t.Fatalf("mismatch/failed statuses = %#v %#v level=%s",
			report.BackupEvidence, report.RestoreExercise, report.Level)
	}
	invalid := inspectFixture(t, workspaceRoot, home, project, "unknown.yaml", "")
	if invalid.BackupEvidence.Status != StatusInvalid || invalid.Level != LevelAttention {
		t.Fatalf("invalid evidence status = %#v level=%s", invalid.BackupEvidence, invalid.Level)
	}
	targetMismatch := inspectFixture(t, workspaceRoot, home, project, "", "wrong-targets.yaml")
	if targetMismatch.RestoreExercise.Status != StatusMismatch {
		t.Fatalf("target-mismatched exercise status = %#v", targetMismatch.RestoreExercise)
	}
}

func TestInspectEvidencePathBoundaryFailsClosed(t *testing.T) {
	workspaceRoot := copyWorkspaceFixture(t)
	home := t.TempDir()
	project := t.TempDir()
	base := inspectFixture(t, workspaceRoot, home, project, "", "")
	evidenceRoot := filepath.Join(workspaceRoot, ".aiah", "evidence")
	if err := os.MkdirAll(evidenceRoot, 0o700); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "private-evidence-name.yaml")
	privateReference := "private-evidence-must-not-be-read"
	writeEvidence(t, outside, backupEvidenceYAML(base.Subject, base.Subject.SelectionSHA256, privateReference))
	if err := os.Symlink(outside, filepath.Join(evidenceRoot, "escape.yaml")); err != nil {
		t.Fatalf("symlink evidence: %v", err)
	}
	insideWrongDirectory := filepath.Join(workspaceRoot, "outside-evidence.yaml")
	writeEvidence(t, insideWrongDirectory, backupEvidenceYAML(
		base.Subject, base.Subject.SelectionSHA256, privateReference,
	))
	writeEvidence(t, filepath.Join(evidenceRoot, "secret.yaml"), []byte(fmt.Sprintf(
		"schemaVersion: 1\nkind: backup-evidence\nsubject:\n  name: %s\n  version: %s\n  profile: %s\n  selectionSHA256: %s\ncopy:\n  type: object\n  reference: 'token: abcdefghijklmnop'\nrecordedAt: '2026-08-01T00:00:00Z'\n",
		base.Subject.Name, base.Subject.Version, base.Subject.Profile, base.Subject.SelectionSHA256,
	)))

	for _, path := range []string{
		outside,
		insideWrongDirectory,
		"../private-evidence-name.yaml",
		"escape.yaml",
		"secret.yaml",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			report := inspectFixture(t, workspaceRoot, home, project, path, "")
			if report.BackupEvidence.Status != StatusInvalid {
				t.Fatalf("status = %s, want invalid", report.BackupEvidence.Status)
			}
			body, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if strings.Contains(string(body), privateReference) || strings.Contains(string(body), outside) {
				t.Fatal("invalid evidence leaked path or content")
			}
		})
	}
}

func TestSelectionDigestChangesWithSelectedAssetContent(t *testing.T) {
	firstRoot := copyWorkspaceFixture(t)
	secondRoot := copyWorkspaceFixture(t)
	first := inspectFixture(t, firstRoot, t.TempDir(), t.TempDir(), "", "")
	second := inspectFixture(t, secondRoot, t.TempDir(), t.TempDir(), "", "")
	if first.Subject.SelectionSHA256 != second.Subject.SelectionSHA256 {
		t.Fatal("identical selections produced different digests")
	}
	asset := filepath.Join(secondRoot, "assets", "rules", "common.md")
	body, err := os.ReadFile(asset)
	if err != nil {
		t.Fatalf("read selected asset: %v", err)
	}
	if err := os.WriteFile(asset, append(body, []byte("\ncontent changed\n")...), 0o600); err != nil {
		t.Fatalf("change selected asset: %v", err)
	}
	changed := inspectFixture(t, secondRoot, t.TempDir(), t.TempDir(), "", "")
	if first.Subject.SelectionSHA256 == changed.Subject.SelectionSHA256 {
		t.Fatal("selected asset content changed without invalidating selection digest")
	}
}

func TestInspectDoesNotReadEvidenceWhenPackagePreparationIsBlocked(t *testing.T) {
	workspaceRoot := copyWorkspaceFixture(t)
	report, err := Inspect(Options{
		WorkspaceRoot:       workspaceRoot,
		Profile:             "missing-profile",
		Home:                t.TempDir(),
		BackupEvidencePath:  "does-not-exist.yaml",
		RestoreExercisePath: "does-not-exist.json",
	})
	if err != nil {
		t.Fatalf("inspect blocked workspace: %v", err)
	}
	if report.Ok || report.Level != LevelBlocked ||
		report.PackageReadiness.Status != StatusBlocked ||
		report.BackupEvidence.Status != StatusUnchecked ||
		report.RestoreExercise.Status != StatusUnchecked {
		t.Fatalf("blocked report = %#v", report)
	}
}

func TestInspectRejectsManifestOutsideTheAssetLibrary(t *testing.T) {
	workspaceRoot := copyWorkspaceFixture(t)
	outside := filepath.Join(t.TempDir(), "private-manifest.yaml")
	if err := os.WriteFile(outside, []byte("schemaVersion: 1\n"), 0o600); err != nil {
		t.Fatalf("write outside manifest: %v", err)
	}
	_, err := Inspect(Options{
		WorkspaceRoot: workspaceRoot,
		ManifestPath:  outside,
		Profile:       "personal",
		Home:          t.TempDir(),
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error = %v, want ErrInvalidOptions", err)
	}
}

func TestReportsValidateAgainstMigrationReadinessSchema(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "spec", "migration-readiness.schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schemaDocument any
	if err := json.Unmarshal(schemaData, &schemaDocument); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("migration-readiness.schema.json", schemaDocument); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	schema, err := compiler.Compile("migration-readiness.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	workspaceRoot := copyWorkspaceFixture(t)
	reports := []Report{
		inspectFixture(t, workspaceRoot, t.TempDir(), t.TempDir(), "", ""),
	}
	unsupportedRoot := copyWorkspaceFixture(t)
	manifestPath := filepath.Join(unsupportedRoot, "manifest.yaml")
	manifestBody, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read unsupported-target manifest: %v", err)
	}
	manifestBody = bytes.ReplaceAll(
		manifestBody,
		[]byte("targets: [claude, codex]"),
		[]byte("targets: [claude, codex, cursor]"),
	)
	if err := os.WriteFile(manifestPath, manifestBody, 0o600); err != nil {
		t.Fatalf("write unsupported-target manifest: %v", err)
	}
	unsupported := inspectFixture(t, unsupportedRoot, t.TempDir(), t.TempDir(), "", "")
	if unsupported.Ok || unsupported.MigrationPreflight.UnsupportedTargets != 1 {
		t.Fatalf("unsupported-target report = %#v", unsupported.MigrationPreflight)
	}
	reports = append(reports, unsupported)
	blocked, err := Inspect(Options{
		WorkspaceRoot: workspaceRoot,
		Profile:       "missing",
		Home:          t.TempDir(),
	})
	if err != nil {
		t.Fatalf("blocked report: %v", err)
	}
	reports = append(reports, blocked)
	for _, report := range reports {
		body, err := json.Marshal(report)
		if err != nil {
			t.Fatalf("marshal report: %v", err)
		}
		var document any
		if err := json.Unmarshal(body, &document); err != nil {
			t.Fatalf("unmarshal report: %v", err)
		}
		if err := schema.Validate(document); err != nil {
			t.Fatalf("report failed schema: %v\n%s", err, body)
		}
	}
}

func inspectFixture(
	t *testing.T,
	workspaceRoot, home, project, backupPath, restorePath string,
) Report {
	t.Helper()
	report, err := Inspect(Options{
		WorkspaceRoot:       workspaceRoot,
		Profile:             "personal",
		Home:                home,
		Project:             project,
		BackupEvidencePath:  backupPath,
		RestoreExercisePath: restorePath,
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	return report
}

func copyWorkspaceFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "workspace-valid")
	destination := t.TempDir()
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy workspace fixture: %v", err)
	}
	return destination
}

func writeEvidence(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
}

func backupEvidenceYAML(subject Subject, digest, reference string) []byte {
	return []byte(fmt.Sprintf(
		"schemaVersion: 1\nkind: backup-evidence\nsubject:\n  name: %s\n  version: %s\n  profile: %s\n  selectionSHA256: %s\ncopy:\n  type: git-commit\n  reference: %q\nrecordedAt: '2026-08-01T00:00:00Z'\n",
		subject.Name, subject.Version, subject.Profile, digest, reference,
	))
}

func restoreEvidenceYAML(subject Subject, digest, finalStep string) []byte {
	return []byte(fmt.Sprintf(
		"schemaVersion: 1\nkind: restore-exercise\nsubject:\n  name: %s\n  version: %s\n  profile: %s\n  selectionSHA256: %s\n  packageSHA256: %s\ntargets: [claude, codex, shared]\nisolated:\n  home: true\n  project: true\nsteps:\n  pull: {status: passed}\n  preflight: {status: passed}\n  diff: {status: passed}\n  apply: {status: passed}\n  doctor: {status: passed}\n  rollback: {status: %s}\nproducedBy: aiah-test\ncompletedAt: '2026-08-01T00:00:00Z'\n",
		subject.Name,
		subject.Version,
		subject.Profile,
		digest,
		strings.Repeat("a", 64),
		finalStep,
	))
}

func snapshotReadinessTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(body)
			value += ":" + hex.EncodeToString(sum[:])
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		}
		result[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return result
}
