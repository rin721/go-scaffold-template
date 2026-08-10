package i18n

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestNewWithNilConfigCreatesTranslator(t *testing.T) {
	translator, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	if translator == nil {
		t.Fatal("New(nil) returned nil translator")
	}
}

func TestTranslateLoadsJSONAndYAML(t *testing.T) {
	translator := newTestTranslator(t)

	tests := []struct {
		name     string
		language string
		message  Message
		want     string
	}{
		{
			name:     "yaml zh-CN",
			language: "zh-CN",
			message:  Text("hello", WithData(map[string]any{"Name": "小林"})),
			want:     "你好，小林",
		},
		{
			name:     "json en",
			language: "en",
			message:  Text("hello", WithData(map[string]any{"Name": "Rin"})),
			want:     "Hello, Rin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translator.Translate(tt.language, tt.message)
			if err != nil {
				t.Fatalf("Translate returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Translate = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTranslateMatchesLanguagePreferences(t *testing.T) {
	translator := newTestTranslator(t)

	tests := []struct {
		name     string
		language string
		want     string
	}{
		{name: "zh matches zh-CN resource", language: "zh", want: "你好，小林"},
		{name: "en-US matches en resource", language: "en-US", want: "Hello, 小林"},
		{name: "empty language uses default", language: "", want: "你好，小林"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := translator.Translate(tt.language, Text("hello", WithData(map[string]any{"Name": "小林"})))
			if err != nil {
				t.Fatalf("Translate returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Translate = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTranslateSupportsPluralCount(t *testing.T) {
	translator := newTestTranslator(t)

	one, err := translator.Translate("en", Text("user_count", WithCount(1)))
	if err != nil {
		t.Fatalf("Translate one returned error: %v", err)
	}
	if one != "1 user" {
		t.Fatalf("one = %q, want %q", one, "1 user")
	}

	other, err := translator.Translate("en", Text("user_count", WithCount(2)))
	if err != nil {
		t.Fatalf("Translate other returned error: %v", err)
	}
	if other != "2 users" {
		t.Fatalf("other = %q, want %q", other, "2 users")
	}
}

func TestTranslateMergesCountIntoTemplateData(t *testing.T) {
	translator := newTestTranslator(t)

	got, err := translator.Translate("zh-CN", Text(
		"cart_summary",
		WithData(map[string]any{"Name": "小林"}),
		WithCount(3),
	))
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if got != "小林有 3 件商品" {
		t.Fatalf("Translate = %q, want %q", got, "小林有 3 件商品")
	}
}

func TestTranslateMissingMessageBehavior(t *testing.T) {
	t.Run("default returns error", func(t *testing.T) {
		translator := newTestTranslator(t)
		_, err := translator.Translate("zh-CN", Text("missing"))
		if err == nil {
			t.Fatal("Translate returned nil error")
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Fatalf("error %q does not contain message id", err.Error())
		}
	})

	t.Run("use id returns message id", func(t *testing.T) {
		translator, err := New(&Config{
			MessageFS:       testMessageFS(),
			MessageFiles:    []string{"locales/active.zh-CN.yaml"},
			MissingBehavior: MissingBehaviorUseID,
		})
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}

		got, err := translator.Translate("zh-CN", Text("missing"))
		if err != nil {
			t.Fatalf("Translate returned error: %v", err)
		}
		if got != "missing" {
			t.Fatalf("Translate = %q, want %q", got, "missing")
		}
	})
}

func TestTranslateUsesDefaultMessage(t *testing.T) {
	translator := newTestTranslator(t)

	got, err := translator.Translate("zh-CN", Text("missing", WithDefault("默认文案")))
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if got != "默认文案" {
		t.Fatalf("Translate = %q, want %q", got, "默认文案")
	}
}

func TestMustTranslatePanicsOnError(t *testing.T) {
	translator := newTestTranslator(t)

	defer func() {
		if recover() == nil {
			t.Fatal("MustTranslate did not panic")
		}
	}()

	translator.MustTranslate("zh-CN", Text("missing"))
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want string
	}{
		{name: "invalid language", cfg: &Config{DefaultLanguage: "not a language"}, want: "default language"},
		{name: "invalid missing behavior", cfg: &Config{MissingBehavior: MissingBehavior("fallback")}, want: "missing behavior"},
		{name: "empty file path", cfg: &Config{MessageFiles: []string{""}}, want: "empty"},
		{name: "unsupported extension", cfg: &Config{MessageFiles: []string{"locales/active.zh-CN.toml"}}, want: "extension"},
		{name: "missing file", cfg: &Config{MessageFiles: []string{"locales/missing.zh-CN.yaml"}}, want: "load message file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Fatal("New returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestDefaultConfigCanBeCustomized(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MessageFS = testMessageFS()
	cfg.MessageFiles = []string{"locales/active.en.json"}

	translator, err := New(&cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	got, err := translator.Translate("en-US", Text("hello", WithData(map[string]any{"Name": "Rin"})))
	if err != nil {
		t.Fatalf("Translate returned error: %v", err)
	}
	if got != "Hello, Rin" {
		t.Fatalf("Translate = %q, want %q", got, "Hello, Rin")
	}
}

func newTestTranslator(t *testing.T) Translator {
	t.Helper()

	translator, err := New(&Config{
		DefaultLanguage: DefaultLanguage,
		MessageFS:       testMessageFS(),
		MessageFiles: []string{
			"locales/active.zh-CN.yaml",
			"locales/active.en.json",
		},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	return translator
}

func testMessageFS() fstest.MapFS {
	return fstest.MapFS{
		"locales/active.zh-CN.yaml": {
			Data: []byte(`
hello:
  other: "你好，{{.Name}}"
user_count:
  one: "{{.Count}} 个用户"
  other: "{{.Count}} 个用户"
cart_summary:
  other: "{{.Name}}有 {{.Count}} 件商品"
`),
		},
		"locales/active.en.json": {
			Data: []byte(`{
  "hello": {
    "other": "Hello, {{.Name}}"
  },
  "user_count": {
    "one": "{{.Count}} user",
    "other": "{{.Count}} users"
  }
}`),
		},
	}
}
