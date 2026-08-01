package tui

import (
	"embed"
	"encoding/json"
	"fmt"
)

// Translations live in locales/*.json rather than Go map literals. They are
// data, not logic: keeping them out of the source lets the catalogs be edited
// without touching Go, and stops ~1000 lines of copy from being counted as TUI
// complexity. The message IDs stay in messages.go because they are the
// vocabulary the view code compiles against.
//
// Moving them costs the compile-time check that a catalog key is a declared
// messageID. TestEveryDeclaredMessageIDHasCatalogEntries and
// TestMessageCatalogsHaveParity replace it: they assert both catalogs cover
// exactly the declared IDs, carry no empty values, and use identical format
// verbs across languages.
//
//go:embed locales/en.json locales/zh-CN.json
var localeFiles embed.FS

var (
	messagesEnglish = mustLoadCatalog("locales/en.json")
	messagesZhCN    = mustLoadCatalog("locales/zh-CN.json")
)

// mustLoadCatalog panics because the catalogs are compiled into the binary: a
// failure here cannot depend on the user's machine, so it is a build defect
// that the test suite sees long before any release.
func mustLoadCatalog(name string) map[messageID]string {
	body, err := localeFiles.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("tui: embedded catalog %s is missing: %v", name, err))
	}
	// No DisallowUnknownFields here: it only applies to structs, so on a map it
	// would read as a guard while doing nothing. An undeclared key is caught by
	// TestEveryDeclaredMessageIDHasCatalogEntries, which compares catalog size
	// against the IDs declared in messages.go.
	var plain map[string]string
	if err := json.Unmarshal(body, &plain); err != nil {
		panic(fmt.Sprintf("tui: embedded catalog %s does not parse: %v", name, err))
	}
	catalog := make(map[messageID]string, len(plain))
	for id, value := range plain {
		catalog[messageID(id)] = value
	}
	return catalog
}
