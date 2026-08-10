package codec

import (
	"strings"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	var out map[string]string
	data, err := JSON().Encode(map[string]string{"name": "rin"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if err := JSON().Decode(data, &out); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if out["name"] != "rin" {
		t.Fatalf("decoded = %#v", out)
	}
}

func TestDecodeLimitedRejectsLargePayload(t *testing.T) {
	var out map[string]string
	err := DecodeLimited(strings.NewReader(`{"name":"rin"}`), 4, JSON(), &out)
	if err == nil {
		t.Fatal("DecodeLimited() error = nil, want size error")
	}
}
