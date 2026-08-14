package s3compat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	storageclient "github.com/rin721/go-scaffold-template/pkg/storage/client"
)

func TestClientExerciseWithPathStyleEndpoint(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/test-bucket/")
		if r.URL.Path == "/test-bucket" || r.URL.Path == "/test-bucket/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if key == "" || key == r.URL.Path {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			mu.Lock()
			objects[key] = body
			mu.Unlock()
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			data, ok := objects[key]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(data)
		case http.MethodHead:
			mu.Lock()
			_, ok := objects[key]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			mu.Lock()
			delete(objects, key)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(context.Background(), Config{
		Provider:        ProviderMinIO,
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "test-bucket",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		PathStyle:       true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	const key = "setup/storage-healthcheck.txt"
	if err := client.Put(context.Background(), key, []byte("ok"), storageclient.PutOptions{ContentType: "text/plain"}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	exists, err := client.Exists(context.Background(), key)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Fatal("Exists() = false, want true")
	}
	data, _, err := client.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("Get() data = %q, want ok", data)
	}
	if err := client.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	exists, err = client.Exists(context.Background(), key)
	if err != nil {
		t.Fatalf("Exists() after delete error = %v", err)
	}
	if exists {
		t.Fatal("Exists() after delete = true, want false")
	}
}

func TestClientRejectsInvalidObjectKey(t *testing.T) {
	t.Parallel()

	client, err := New(context.Background(), Config{
		Provider:        ProviderR2,
		Endpoint:        "http://127.0.0.1:1",
		Region:          "auto",
		Bucket:          "bucket",
		AccessKeyID:     "access",
		SecretAccessKey: "secret",
		PathStyle:       true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.Put(context.Background(), "../escape", []byte("x"), storageclient.PutOptions{}); err == nil {
		t.Fatal("Put() with invalid key error = nil")
	}
}
