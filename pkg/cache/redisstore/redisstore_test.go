package redisstore

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	scaffoldcache "github.com/rin721/go-scaffold-template/pkg/cache"
)

func TestNewRejectsNilClient(t *testing.T) {
	_, err := New(nil, nil)
	if !errors.Is(err, scaffoldcache.ErrNilRemoteStore) {
		t.Fatalf("New nil client error = %v, want %v", err, scaffoldcache.ErrNilRemoteStore)
	}
}

func TestSetGetAndTTL(t *testing.T) {
	store, closeStore := newRedisStore(t)
	defer closeStore()

	if err := store.Set(context.Background(), "profile:1", []byte("value"), time.Minute, nil, 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, ttl, err := store.Get(context.Background(), "profile:1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if string(got) != "value" {
		t.Fatalf("Get value = %q, want value", got)
	}
	if ttl <= 0 {
		t.Fatalf("ttl = %s, want positive", ttl)
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	store, closeStore := newRedisStore(t)
	defer closeStore()

	_, _, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, scaffoldcache.ErrNotFound) {
		t.Fatalf("Get error = %v, want %v", err, scaffoldcache.ErrNotFound)
	}
}

func TestDeleteRemovesKey(t *testing.T) {
	store, closeStore := newRedisStore(t)
	defer closeStore()

	if err := store.Set(context.Background(), "profile:2", []byte("value"), time.Minute, nil, 0); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := store.Delete(context.Background(), "profile:2"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	_, _, err := store.Get(context.Background(), "profile:2")
	if !errors.Is(err, scaffoldcache.ErrNotFound) {
		t.Fatalf("Get after delete = %v, want %v", err, scaffoldcache.ErrNotFound)
	}
}

func TestInvalidateTagsDeletesTaggedKeys(t *testing.T) {
	store, closeStore := newRedisStore(t)
	defer closeStore()

	if err := store.Set(context.Background(), "profile:3", []byte("one"), time.Minute, []string{"user"}, time.Hour); err != nil {
		t.Fatalf("Set first returned error: %v", err)
	}
	if err := store.Set(context.Background(), "profile:4", []byte("two"), time.Minute, []string{"user"}, time.Hour); err != nil {
		t.Fatalf("Set second returned error: %v", err)
	}

	keys, err := store.InvalidateTags(context.Background(), []string{"user"})
	if err != nil {
		t.Fatalf("InvalidateTags returned error: %v", err)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"profile:3", "profile:4"}) {
		t.Fatalf("invalidated keys = %v, want [profile:3 profile:4]", keys)
	}

	for _, key := range keys {
		_, _, err := store.Get(context.Background(), key)
		if !errors.Is(err, scaffoldcache.ErrNotFound) {
			t.Fatalf("Get %q after invalidate = %v, want %v", key, err, scaffoldcache.ErrNotFound)
		}
	}
}

func TestSetRejectsInvalidInput(t *testing.T) {
	store, closeStore := newRedisStore(t)
	defer closeStore()

	tests := []struct {
		name string
		ctx  context.Context
		key  string
		ttl  time.Duration
		want error
	}{
		{name: "nil context", ctx: nil, key: "key", ttl: time.Minute, want: scaffoldcache.ErrNilContext},
		{name: "empty key", ctx: context.Background(), key: " ", ttl: time.Minute, want: scaffoldcache.ErrEmptyKey},
		{name: "invalid ttl", ctx: context.Background(), key: "key", ttl: 0, want: scaffoldcache.ErrInvalidTTL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Set(tt.ctx, tt.key, []byte("value"), tt.ttl, nil, 0)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Set error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestContextCancellationIsReturned(t *testing.T) {
	store, closeStore := newRedisStore(t)
	defer closeStore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := store.Get(ctx, "profile:5")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get canceled error = %v, want %v", err, context.Canceled)
	}
}

func newRedisStore(t *testing.T) (*Store, func()) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{server.Addr()}})

	remote, err := New(client, nil)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	store, ok := remote.(*Store)
	if !ok {
		t.Fatalf("New returned %T, want *Store", remote)
	}

	return store, func() {
		_ = client.Close()
		server.Close()
	}
}
