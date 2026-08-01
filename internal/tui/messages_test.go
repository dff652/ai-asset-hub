package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/dff652/ai-asset-hub/internal/inventory"
)

var formatVerbPattern = regexp.MustCompile(
	`%(?:\[[0-9]+\])?[-+#0 ]*[0-9]*(?:\.[0-9]+)?[a-zA-Z]`,
)

func TestMessageCatalogsHaveParity(t *testing.T) {
	if len(messagesZhCN) != len(messagesEnglish) {
		t.Fatalf(
			"catalog size differs: zh-CN=%d en=%d",
			len(messagesZhCN),
			len(messagesEnglish),
		)
	}
	for id, english := range messagesEnglish {
		chinese, ok := messagesZhCN[id]
		if !ok {
			t.Errorf("zh-CN catalog is missing %q", id)
			continue
		}
		if strings.TrimSpace(english) == "" {
			t.Errorf("en catalog has an empty value for %q", id)
		}
		if strings.TrimSpace(chinese) == "" {
			t.Errorf("zh-CN catalog has an empty value for %q", id)
		}
		if got, want := formatVerbPattern.FindAllString(chinese, -1),
			formatVerbPattern.FindAllString(english, -1); !reflect.DeepEqual(got, want) {
			t.Errorf("format verbs differ for %q: zh-CN=%v en=%v", id, got, want)
		}
	}
	for id := range messagesZhCN {
		if _, ok := messagesEnglish[id]; !ok {
			t.Errorf("en catalog is missing %q", id)
		}
	}
}

func declaredMessageIDs(t *testing.T) map[messageID]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "messages.go", nil, 0)
	if err != nil {
		t.Fatalf("parse messages.go: %v", err)
	}
	declared := make(map[messageID]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || !strings.HasPrefix(spec.Names[0].Name, "msg") ||
			len(spec.Values) != 1 {
			return true
		}
		literal, ok := spec.Values[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Errorf("message ID %s does not have a string literal value", spec.Names[0].Name)
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Errorf("unquote message ID %s: %v", spec.Names[0].Name, err)
			return true
		}
		declared[messageID(value)] = true
		return true
	})
	if len(declared) == 0 {
		t.Fatal("no message IDs found in messages.go")
	}
	return declared
}

func TestEveryDeclaredMessageIDHasCatalogEntries(t *testing.T) {
	declared := declaredMessageIDs(t)

	for id := range declared {
		if _, ok := messagesZhCN[id]; !ok {
			t.Errorf("declared message %q is missing from zh-CN", id)
		}
		if _, ok := messagesEnglish[id]; !ok {
			t.Errorf("declared message %q is missing from en", id)
		}
	}
	if len(messagesZhCN) != len(declared) || len(messagesEnglish) != len(declared) {
		t.Errorf(
			"declared/catalog size differs: declared=%d zh-CN=%d en=%d",
			len(declared),
			len(messagesZhCN),
			len(messagesEnglish),
		)
	}
}

func TestCatalogsRejectUndeclaredKeys(t *testing.T) {
	// The loader cannot reject these itself -- DisallowUnknownFields only applies
	// to structs, so on a map[string]string it silently accepts anything. Catalog
	// size against the declared IDs is the actual guard.
	declared := declaredMessageIDs(t)
	for name, catalog := range map[string]map[messageID]string{
		"en": messagesEnglish, "zh-CN": messagesZhCN,
	} {
		for id := range catalog {
			if !declared[id] {
				t.Errorf("%s catalog has undeclared key %q", name, id)
			}
		}
	}
}

func TestDefaultLanguageRemainsChinese(t *testing.T) {
	model := NewModel(inventory.Options{})
	if model.language != languageZhCN {
		t.Fatalf("default language = %q, want %q", model.language, languageZhCN)
	}
	if got := model.text(msgHomeAppTitle); got != "AI 编程资产管理器" {
		t.Fatalf("default title = %q", got)
	}
}

func TestUnknownLanguageFallsBackToEnglish(t *testing.T) {
	model := NewModel(inventory.Options{}).withLanguage(language("unsupported"))
	if got := model.text(msgHomeAppTitle); got != "AI Coding Asset Manager" {
		t.Fatalf("fallback title = %q", got)
	}
	if got := model.text(messageID("unknown")); got != "[missing:unknown]" {
		t.Fatalf("missing marker = %q", got)
	}
}

func TestLanguageUpdatesLocalizedInputPlaceholders(t *testing.T) {
	model := NewModel(inventory.Options{}).withLanguage(languageEnglish)
	if got := model.filterInput.Placeholder; got != "Filter asset details, state, or risk" {
		t.Fatalf("English filter placeholder = %q", got)
	}
}

func TestMigratedTUIFilesHaveNoHanStringLiterals(t *testing.T) {
	files := []string{
		"home_view.go",
		"inventory_view.go",
		"inventory_rows.go",
		"compose.go",
		"manage.go",
		"workflow.go",
		"model.go",
		"diff.go",
		"diff_view.go",
		"health.go",
		"health_view.go",
		"migration.go",
		"migration_actions.go",
		"migration_view.go",
		"version.go",
		"version_view.go",
		"settings.go",
	}
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Errorf("unquote %s in %s: %v", literal.Value, path, err)
				return true
			}
			for _, character := range value {
				if unicode.In(character, unicode.Han) {
					t.Errorf(
						"%s contains migrated Han string literal %q; use a message ID",
						path,
						value,
					)
					break
				}
			}
			return true
		})
	}
}
