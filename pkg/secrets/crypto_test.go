package secrets

import "testing"

func TestHMACAndDeriveKey(t *testing.T) {
	signature := HMACSHA256(New("secret"), []byte("message"))
	if signature == "" {
		t.Fatal("signature is empty")
	}
	key, err := DeriveKey(New("secret"), []byte("salt"), 1, 16)
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}
	if key.Value() == "" || key.String() == key.Value() {
		t.Fatal("derived key is invalid or leaked")
	}
}
