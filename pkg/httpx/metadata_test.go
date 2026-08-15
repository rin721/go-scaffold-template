package httpx

import "testing"

func TestRequestLanguageContextUsesTypedPrivateKey(t *testing.T) {
	ctx := WithRequestLanguage(t.Context(), "zh-CN")
	if language := RequestLanguageFromContext(ctx); language != "zh-CN" {
		t.Fatalf("RequestLanguageFromContext() = %q", language)
	}
	if language := RequestLanguageFromContext(t.Context()); language != "" {
		t.Fatalf("empty RequestLanguageFromContext() = %q", language)
	}
	if language := RequestLanguageFromContext(nil); language != "" {
		t.Fatalf("nil RequestLanguageFromContext() = %q", language)
	}
}
