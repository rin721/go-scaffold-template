package storage

import "testing"

func TestObjectPolicyValidatesContentType(t *testing.T) {
	policy := ObjectPolicy{AllowedMIMETypes: map[string]struct{}{"text/plain": {}}}
	if err := policy.ValidateObject(ObjectInfo{Key: "a", ContentType: "text/plain"}); err != nil {
		t.Fatalf("ValidateObject() error = %v", err)
	}
	if err := policy.ValidateObject(ObjectInfo{Key: "a", ContentType: "application/octet-stream"}); err == nil {
		t.Fatal("ValidateObject() error = nil, want MIME error")
	}
}
