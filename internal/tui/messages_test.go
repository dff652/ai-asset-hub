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

func TestMigratedHomeViewHasNoHanStringLiterals(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "home_view.go", nil, 0)
	if err != nil {
		t.Fatalf("parse home_view.go: %v", err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Errorf("unquote %s: %v", literal.Value, err)
			return true
		}
		for _, character := range value {
			if unicode.In(character, unicode.Han) {
				t.Errorf(
					"home_view.go contains migrated Han string literal %q; use a message ID",
					value,
				)
				break
			}
		}
		return true
	})
}
