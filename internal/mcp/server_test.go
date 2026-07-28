package mcp

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dff652/ai-asset-hub/internal/build"
)

// readOnlySurface is the whole contract of this package. Anything that writes
// belongs on the CLI; adding a name here is a boundary change and must be
// argued in ADR-0005, not slipped in with a feature.
var readOnlySurface = []string{
	"aiah_diff",
	"aiah_doctor",
	"aiah_scan",
	"aiah_validate",
	"aiah_version",
}

func TestToolsExposeOnlyTheReadOnlySurface(t *testing.T) {
	names := make([]string, 0)
	for _, tool := range Tools() {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != strings.Join(readOnlySurface, ",") {
		t.Fatalf("tool surface changed:\n got: %v\nwant: %v", names, readOnlySurface)
	}
	for _, tool := range Tools() {
		if tool.Handler == nil {
			t.Fatalf("tool %s has no handler", tool.Name)
		}
		if tool.Description == "" {
			t.Fatalf("tool %s has no description", tool.Name)
		}
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("tool %s input schema is not an object", tool.Name)
		}
	}
}

// TestToolCallsWriteNothing is the load-bearing test of this package. Every
// registered tool runs against a real home and project, and both trees must be
// byte-identical afterwards. A tool that writes -- or a write-capable tool
// added to the registry -- fails here regardless of what it is called.
func TestToolCallsWriteNothing(t *testing.T) {
	home := copyTree(t, filepath.Join("..", "..", "testdata", "home-basic"))
	project := t.TempDir()
	pkg := buildFixturePackage(t)

	arguments := map[string]map[string]any{
		"aiah_scan":     {"home": home, "project": project},
		"aiah_validate": {"manifest": filepath.Join("..", "..", "testdata", "workspace-2b", "manifest.yaml")},
		"aiah_diff":     {"package": pkg, "home": home, "project": project},
		"aiah_doctor":   {"home": home, "project": project},
		"aiah_version":  {},
	}

	beforeHome := snapshotTree(t, home)
	beforeProject := snapshotTree(t, project)

	for _, tool := range Tools() {
		args, ok := arguments[tool.Name]
		if !ok {
			t.Fatalf("tool %s has no write-safety coverage; add arguments for it", tool.Name)
		}
		encoded, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("encode arguments for %s: %v", tool.Name, err)
		}
		if _, err := tool.Handler(encoded); err != nil {
			t.Fatalf("%s returned an error, so it never exercised the write path: %v", tool.Name, err)
		}
	}

	if diff := treeDiff(beforeHome, snapshotTree(t, home)); diff != "" {
		t.Fatalf("home changed after read-only tool calls:\n%s", diff)
	}
	if diff := treeDiff(beforeProject, snapshotTree(t, project)); diff != "" {
		t.Fatalf("project changed after read-only tool calls:\n%s", diff)
	}
}

func TestInitializeAdvertisesToolsAndReadOnlyIntent(t *testing.T) {
	responses := serve(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(responses) != 1 {
		t.Fatalf("want 1 response, got %d", len(responses))
	}
	result := resultOf(t, responses[0])
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion = %v, want %s", result["protocolVersion"], protocolVersion)
	}
	capabilities, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("capabilities missing")
	}
	if _, ok := capabilities["tools"]; !ok {
		t.Fatal("tools capability not advertised")
	}
	instructions, _ := result["instructions"].(string)
	if !strings.Contains(instructions, "cannot apply") {
		t.Fatalf("instructions do not state the read-only boundary: %q", instructions)
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != serverName {
		t.Fatalf("serverInfo = %v", result["serverInfo"])
	}
}

func TestToolsListReportsSchemas(t *testing.T) {
	responses := serve(t, `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`)
	result := resultOf(t, responses[0])
	listed, ok := result["tools"].([]any)
	if !ok || len(listed) != len(readOnlySurface) {
		t.Fatalf("tools/list returned %v", result["tools"])
	}
	first, _ := listed[0].(map[string]any)
	if first["name"] != readOnlySurface[0] {
		t.Fatalf("tools are not ordered by name: %v", first["name"])
	}
	if _, ok := first["inputSchema"].(map[string]any); !ok {
		t.Fatal("inputSchema missing from tools/list")
	}
}

func TestToolsCallReturnsReportAsTextContent(t *testing.T) {
	home := filepath.Join("..", "..", "testdata", "home-basic")
	request := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"aiah_scan","arguments":{"home":%q}}}`,
		home,
	)
	result := resultOf(t, serve(t, request)[0])
	if isError, ok := result["isError"].(bool); ok && isError {
		t.Fatalf("scan reported an error: %v", result)
	}
	text := textContentOf(t, result)
	var report map[string]any
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("tool content is not JSON: %v", err)
	}
	if report["kind"] != "inventory" {
		t.Fatalf("kind = %v, want inventory", report["kind"])
	}
}

func TestUnknownToolIsToolErrorNotProtocolError(t *testing.T) {
	// The model should be able to see and recover from this, so it must not
	// arrive as a JSON-RPC error.
	result := resultOf(t, serve(t,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"aiah_apply"}}`)[0])
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("unknown tool did not report isError: %v", result)
	}
	if !strings.Contains(textContentOf(t, result), "unknown tool") {
		t.Fatalf("unhelpful message: %v", result)
	}
}

func TestUnknownArgumentIsRejected(t *testing.T) {
	// A misspelled argument must fail loudly: silently ignoring it would change
	// which paths a tool reads without the caller noticing.
	result := resultOf(t, serve(t,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"aiah_scan","arguments":{"hoem":"/tmp"}}}`)[0])
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("unknown argument was accepted: %v", result)
	}
}

func TestMissingRequiredArgumentIsRejected(t *testing.T) {
	result := resultOf(t, serve(t,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"aiah_validate","arguments":{}}}`)[0])
	if isError, _ := result["isError"].(bool); !isError {
		t.Fatalf("missing manifest was accepted: %v", result)
	}
}

func TestNotificationsGetNoResponse(t *testing.T) {
	responses := serve(t,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
	)
	if len(responses) != 1 {
		t.Fatalf("want exactly the ping response, got %d: %v", len(responses), responses)
	}
	if idOf(t, responses[0]) != "1" {
		t.Fatalf("the notification was answered: %v", responses[0])
	}
}

func TestProtocolErrors(t *testing.T) {
	cases := []struct {
		name    string
		request string
		code    float64
	}{
		{"malformed json", `{"jsonrpc":"2.0","id":1,`, codeParseError},
		{"wrong jsonrpc version", `{"jsonrpc":"1.0","id":1,"method":"ping"}`, codeInvalidRequest},
		{"unknown method", `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, codeMethodNotFound},
		{"missing tool name", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`, codeInvalidParams},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			responses := serve(t, testCase.request)
			if len(responses) != 1 {
				t.Fatalf("want 1 response, got %d", len(responses))
			}
			rpcError, ok := responses[0]["error"].(map[string]any)
			if !ok {
				t.Fatalf("want an error response, got %v", responses[0])
			}
			if rpcError["code"] != testCase.code {
				t.Fatalf("code = %v, want %v", rpcError["code"], testCase.code)
			}
		})
	}
}

func TestOversizedRequestStopsTheStream(t *testing.T) {
	// Resyncing after an oversized line would mean parsing from an arbitrary
	// offset, so the server stops instead.
	huge := `{"jsonrpc":"2.0","id":1,"method":"ping","pad":"` +
		strings.Repeat("a", maxRequestBytes+16) + `"}`
	var out, errOut strings.Builder
	err := Run(Options{In: strings.NewReader(huge + "\n"), Out: &out, ErrOut: &errOut})
	if !errors.Is(err, errRequestTooLarge) {
		t.Fatalf("err = %v, want errRequestTooLarge", err)
	}
	if out.Len() != 0 {
		t.Fatalf("oversized request produced output: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "size limit") {
		t.Fatalf("no diagnostic on stderr: %q", errOut.String())
	}
}

func TestRunRequiresStreams(t *testing.T) {
	if err := Run(Options{Out: io.Discard}); err == nil {
		t.Fatal("missing input stream was accepted")
	}
	if err := Run(Options{In: strings.NewReader("")}); err == nil {
		t.Fatal("missing output stream was accepted")
	}
}

func TestFinalLineWithoutNewlineIsServed(t *testing.T) {
	var out strings.Builder
	if err := Run(Options{
		In:  strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"ping"}`),
		Out: &out,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"id":9`) {
		t.Fatalf("unterminated final request was dropped: %q", out.String())
	}
}

func TestHomeDefaultsToUserHomeDir(t *testing.T) {
	original := userHomeDir
	t.Cleanup(func() { userHomeDir = original })

	userHomeDir = func() (string, error) {
		return filepath.Join("..", "..", "testdata", "home-basic"), nil
	}
	if _, err := handleScan(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("scan with defaulted home: %v", err)
	}

	userHomeDir = func() (string, error) { return "", errors.New("no home") }
	if _, err := handleScan(json.RawMessage(`{}`)); !errors.Is(err, errInvalidArguments) {
		t.Fatalf("err = %v, want errInvalidArguments", err)
	}
}

// --- helpers ---

func serve(t *testing.T, requests ...string) []map[string]any {
	t.Helper()
	var out strings.Builder
	input := strings.Join(requests, "\n") + "\n"
	if err := Run(Options{In: strings.NewReader(input), Out: &out, ErrOut: io.Discard}); err != nil {
		t.Fatalf("run: %v", err)
	}
	responses := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("response is not JSON: %q", line)
		}
		if decoded["jsonrpc"] != "2.0" {
			t.Fatalf("response is not JSON-RPC 2.0: %q", line)
		}
		responses = append(responses, decoded)
	}
	return responses
}

func resultOf(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", response)
	}
	return result
}

func idOf(t *testing.T, response map[string]any) string {
	t.Helper()
	return fmt.Sprintf("%v", response["id"])
}

func textContentOf(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in %v", result)
	}
	first, ok := content[0].(map[string]any)
	if !ok || first["type"] != "text" {
		t.Fatalf("first content block is not text: %v", content[0])
	}
	text, _ := first["text"].(string)
	return text
}

// buildFixturePackage produces a real package outside the snapshotted trees so
// aiah_diff exercises its full path.
func buildFixturePackage(t *testing.T) string {
	t.Helper()
	outDir := t.TempDir()
	if _, err := build.Build(build.Options{
		Manifest: filepath.Join("..", "..", "testdata", "workspace-2b", "manifest.yaml"),
		Profile:  "personal",
		OutDir:   outDir,
	}); err != nil {
		t.Fatalf("build fixture package: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "*.tar"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("want exactly one package tar in %s, got %v (%v)", outDir, matches, err)
	}
	return matches[0]
}

func copyTree(t *testing.T, source string) string {
	t.Helper()
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("copy %s: %v", source, err)
	}
	return destination
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
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
		value := fmt.Sprintf("%s:%04o", info.Mode().Type(), info.Mode().Perm())
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(body))
		}
		snapshot[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

func treeDiff(before, after map[string]string) string {
	var lines []string
	for path, value := range after {
		previous, existed := before[path]
		switch {
		case !existed:
			lines = append(lines, "created: "+path)
		case previous != value:
			lines = append(lines, "modified: "+path)
		}
	}
	for path := range before {
		if _, still := after[path]; !still {
			lines = append(lines, "removed: "+path)
		}
	}
	return strings.Join(lines, "\n")
}
