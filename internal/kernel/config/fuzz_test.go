package config

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzJSONObject(f *testing.F) {
	f.Add([]byte(`{"database":{"dsn":"app.db","pingTimeout":"5s"}}`))
	f.Add([]byte(`{"database":{"dsn":"one","dsn":"two"}}`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 64*1024 {
			t.Skip()
		}
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatalf("write fuzz config: %v", err)
		}
		_, _ = New(FileSource(path)).Load(t.Context())
	})
}
