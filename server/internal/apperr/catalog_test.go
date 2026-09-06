package apperr

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// catalogCodeConstants parses catalog.go and returns the value of every
// exported Code* constant. Reading the source rather than a hand-maintained
// list is what makes this test catch a constant added without a catalog entry.
func catalogCodeConstants(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "catalog.go", nil, 0)
	if err != nil {
		t.Fatalf("parse catalog.go: %v", err)
	}

	var codes []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if !strings.HasPrefix(name.Name, "Code") || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", name.Name, err)
				}
				codes = append(codes, value)
			}
		}
	}
	if len(codes) == 0 {
		t.Fatal("no Code* constants found — the parser or catalog.go layout changed")
	}
	return codes
}

func TestEveryCodeConstantHasACatalogEntry(t *testing.T) {
	for _, code := range catalogCodeConstants(t) {
		if _, ok := ErrorCatalog[code]; !ok {
			t.Errorf("code %q has no ErrorCatalog entry — GetMessage would render %q to the user", code, "error: "+code)
		}
	}
}

func TestEveryCatalogEntryIsTranslatedInBothLanguages(t *testing.T) {
	for code, msg := range ErrorCatalog {
		if strings.TrimSpace(msg.EN) == "" {
			t.Errorf("code %q has an empty EN message", code)
		}
		if strings.TrimSpace(msg.FR) == "" {
			t.Errorf("code %q has an empty FR message", code)
		}
	}
}

func TestCatalogPlaceholdersMatchAcrossLanguages(t *testing.T) {
	// A {name} present in one language only leaves a literal "{name}" on screen
	// for users of the other, since GetMessage substitutes per-language text.
	for code, msg := range ErrorCatalog {
		en, fr := placeholderSet(msg.EN), placeholderSet(msg.FR)
		if len(en) != len(fr) {
			t.Errorf("code %q: placeholders differ (EN %v, FR %v)", code, en, fr)
			continue
		}
		for name := range en {
			if !fr[name] {
				t.Errorf("code %q: placeholder {%s} missing from the FR message", code, name)
			}
		}
	}
}

func placeholderSet(msg string) map[string]bool {
	out := map[string]bool{}
	for {
		open := strings.Index(msg, "{")
		if open < 0 {
			return out
		}
		close := strings.Index(msg[open:], "}")
		if close < 0 {
			return out
		}
		out[msg[open+1:open+close]] = true
		msg = msg[open+close:]
	}
}

func TestGetMessageFallsBackToEnglishForAnUnknownLanguage(t *testing.T) {
	got := GetMessage(CodeRunbookNotFound, "de", nil)
	if got != ErrorCatalog[CodeRunbookNotFound].EN {
		t.Errorf("GetMessage(de) = %q, want the EN message", got)
	}
}

func TestGetMessageSubstitutesParams(t *testing.T) {
	got := GetMessage(CodeGitProviderError, "en", map[string]string{"status": "503"})
	if !strings.Contains(got, "503") {
		t.Errorf("GetMessage did not substitute {status}: %q", got)
	}
	if strings.Contains(got, "{status}") {
		t.Errorf("GetMessage left the placeholder in place: %q", got)
	}
}

// TestEveryCodeHasAFrontendTranslation keeps the catalog and the SPA's
// errors.json in step. respondError sends `i18nKey` so the frontend can render
// the message in whatever language the *UI* is set to; a code with no key
// there silently degrades to the server's own Accept-Language text, which
// follows the browser instead and can disagree with the user's choice.
func TestEveryCodeHasAFrontendTranslation(t *testing.T) {
	for _, lang := range []string{"en", "fr"} {
		path := filepath.Join("..", "..", "..", "frontend", "src", "locales", lang, "errors.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("frontend locales not available (%v)", err)
		}
		var messages map[string]any
		if err := json.Unmarshal(raw, &messages); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, code := range catalogCodeConstants(t) {
			if _, ok := messages[code]; !ok {
				t.Errorf("code %q has no key in frontend/src/locales/%s/errors.json", code, lang)
			}
		}
	}
}
