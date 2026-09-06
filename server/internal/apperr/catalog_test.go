package apperr

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
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

func TestEveryCatalogEntryIsTranslatedInEverySupportedLanguage(t *testing.T) {
	for code, msg := range ErrorCatalog {
		for _, lang := range SupportedLanguages {
			text, ok := msg[lang]
			if !ok {
				t.Errorf("code %q has no %s message", code, lang)
				continue
			}
			if strings.TrimSpace(text) == "" {
				t.Errorf("code %q has an empty %s message", code, lang)
			}
		}
		for lang := range msg {
			if !slices.Contains(SupportedLanguages, lang) {
				t.Errorf("code %q carries an unsupported language %q", code, lang)
			}
		}
	}
}

func TestCatalogPlaceholdersMatchAcrossLanguages(t *testing.T) {
	// A {name} present in one language only leaves a literal "{name}" on screen
	// for users of the other, since GetMessage substitutes per-language text.
	for code, msg := range ErrorCatalog {
		want := placeholderSet(msg[DefaultLanguage])
		for _, lang := range SupportedLanguages {
			got := placeholderSet(msg[lang])
			if len(got) != len(want) {
				t.Errorf("code %q: %s placeholders %v differ from %s %v", code, lang, got, DefaultLanguage, want)
				continue
			}
			for name := range want {
				if !got[name] {
					t.Errorf("code %q: placeholder {%s} missing from the %s message", code, name, lang)
				}
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

func TestGetMessageFallsBackToTheDefaultLanguage(t *testing.T) {
	for _, lang := range []string{"de", "", "   ", "zz-ZZ"} {
		got := GetMessage(CodeRunbookNotFound, lang, nil)
		if got != ErrorCatalog[CodeRunbookNotFound][DefaultLanguage] {
			t.Errorf("GetMessage(%q) = %q, want the %s message", lang, got, DefaultLanguage)
		}
	}
}

func TestGetMessageIsCaseInsensitiveAboutTheLanguage(t *testing.T) {
	for _, lang := range []string{"fr", "FR", " Fr "} {
		if got := GetMessage(CodeRunbookNotFound, lang, nil); got != ErrorCatalog[CodeRunbookNotFound]["fr"] {
			t.Errorf("GetMessage(%q) = %q, want the fr message", lang, got)
		}
	}
}

func TestGetMessageDoesNotEchoAnUnknownCodeToTheUser(t *testing.T) {
	// respondError still puts the machine-readable code in the response's
	// `code` field; the prose must not become "error: SOME_TYPO".
	for _, lang := range SupportedLanguages {
		got := GetMessage("NOT_A_REAL_CODE", lang, nil)
		if strings.Contains(got, "NOT_A_REAL_CODE") {
			t.Errorf("GetMessage leaked the raw code for %s: %q", lang, got)
		}
		if got != ErrorCatalog[CodeInternalError][lang] {
			t.Errorf("GetMessage(%s) = %q, want the generic internal-error message", lang, got)
		}
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

func TestGetLanguageFromAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", "en"},
		{"whitespace only", "   ", "en"},
		{"simple French", "fr", "fr"},
		{"region-qualified", "fr-FR", "fr"},
		{"browser-style list", "fr-FR,fr;q=0.9,en;q=0.8", "fr"},
		{"English preferred", "en-US,en;q=0.9", "en"},
		// The reason this function exists: ranking by position rather than by
		// quality reads this backwards and answers "en".
		{"quality outranks position", "en;q=0.3, fr;q=0.9", "fr"},
		{"quality outranks position, reversed", "fr;q=0.2, en;q=0.8", "en"},
		{"explicitly rejected language", "fr;q=0", "en"},
		{"unsupported language", "de-DE,de;q=0.9", "en"},
		{"unsupported first, supported second", "de;q=1.0, fr;q=0.5", "fr"},
		{"wildcard", "*", "en"},
		{"malformed", ";;;", "en"},
		{"case insensitive", "FR-FR", "fr"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetLanguageFromAcceptLanguage(tc.header); got != tc.want {
				t.Errorf("GetLanguageFromAcceptLanguage(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestGetLanguageFromAcceptLanguageAlwaysReturnsASupportedLanguage(t *testing.T) {
	for _, header := range []string{"", "*", "de", "zh-Hant", "fr;q=0", "not a header at all", "en;q=0,fr;q=0"} {
		got := GetLanguageFromAcceptLanguage(header)
		if !slices.Contains(SupportedLanguages, got) {
			t.Errorf("GetLanguageFromAcceptLanguage(%q) = %q, which is not a supported language", header, got)
		}
	}
}
