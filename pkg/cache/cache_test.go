package cache

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"
)

type profile struct {
	ID   int
	Name string
}

func TestNewRejectsInvalidInput(t *testing.T) {
	if _, err := New[profile](nil, nil); !errors.Is(err, ErrNilRemoteStore) {
		t.Fatalf("New nil remote error = %v, want %v", err, ErrNilRemoteStore)
	}

	remote := newFakeRemoteStore()
	if _, err := New[profile](remote, &Config{DefaultTTL: -time.Second}); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("New negative ttl error = %v, want %v", err, ErrInvalidTTL)
	}
}

func TestClientCloseIsIdempotentAndRejectsFurtherUse(t *testing.T) {
	client, err := New[profile](newFakeRemoteStore(), &Config{
		DefaultTTL:      time.Minute,
		CleanupInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if _, err := client.Get(context.Background(), "profile:1"); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("Get() after Close error = %v, want ErrClientClosed", err)
	}
}

func TestSetRequiresExplicitTTL(t *testing.T) {
	client := mustNewClient[profile](t, newFakeRemoteStore(), nil)

	err := client.Set(context.Background(), "profile:1", profile{ID: 1})
	if !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("Set error = %v, want %v", err, ErrInvalidTTL)
	}
}

func TestSetRejectsInvalidInput(t *testing.T) {
	client := mustNewClient[profile](t, newFakeRemoteStore(), &Config{DefaultTTL: time.Minute})

	tests := []struct {
		name string
		ctx  context.Context
		key  string
		want error
	}{
		{name: "nil context", ctx: nil, key: "profile:1", want: ErrNilContext},
		{name: "empty key", ctx: context.Background(), key: " ", want: ErrEmptyKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.Set(tt.ctx, tt.key, profile{ID: 1})
			if !errors.Is(err, tt.want) {
				t.Fatalf("Set error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSetGetStructFromLocalCache(t *testing.T) {
	remote := newFakeRemoteStore()
	client := mustNewClient[profile](t, remote, &Config{DefaultTTL: time.Minute})

	want := profile{ID: 1, Name: "Rin"}
	if err := client.Set(context.Background(), "profile:1", want); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	got, err := client.Get(context.Background(), "profile:1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Get = %+v, want %+v", got, want)
	}
	if remote.getCount != 0 {
		t.Fatalf("remote get count = %d, want 0", remote.getCount)
	}
}

func TestGetBackfillsLocalCacheFromRemote(t *testing.T) {
	remote := newFakeRemoteStore()
	want := profile{ID: 2, Name: "Remote"}
	bytes := mustEncode(t, want)
	remote.values["profile:2"] = fakeRemoteValue{bytes: bytes, ttl: time.Minute}

	client := mustNewClient[profile](t, remote, &Config{DefaultTTL: time.Minute})

	got, err := client.Get(context.Background(), "profile:2")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Get = %+v, want %+v", got, want)
	}

	delete(remote.values, "profile:2")
	got, err = client.Get(context.Background(), "profile:2")
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if got != want {
		t.Fatalf("second Get = %+v, want %+v", got, want)
	}
	if remote.getCount != 1 {
		t.Fatalf("remote get count = %d, want 1", remote.getCount)
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	client := mustNewClient[profile](t, newFakeRemoteStore(), &Config{DefaultTTL: time.Minute})

	_, err := client.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want %v", err, ErrNotFound)
	}
}

func TestSetAndGetWrapInvalidCachedValue(t *testing.T) {
	client := mustNewClient[chan int](t, newFakeRemoteStore(), &Config{DefaultTTL: time.Minute})
	err := client.Set(context.Background(), "chan", make(chan int))
	if !errors.Is(err, ErrInvalidCachedValue) {
		t.Fatalf("Set marshal error = %v, want %v", err, ErrInvalidCachedValue)
	}

	remote := newFakeRemoteStore()
	remote.values["broken"] = fakeRemoteValue{bytes: []byte("not-msgpack"), ttl: time.Minute}
	brokenClient := mustNewClient[profile](t, remote, &Config{DefaultTTL: time.Minute})
	_, err = brokenClient.Get(context.Background(), "broken")
	if !errors.Is(err, ErrInvalidCachedValue) {
		t.Fatalf("Get decode error = %v, want %v", err, ErrInvalidCachedValue)
	}
}

func TestDeleteRemovesLocalAndReturnsRemoteError(t *testing.T) {
	wantErr := errors.New("redis delete failed")
	remote := newFakeRemoteStore()
	client := mustNewClient[profile](t, remote, &Config{DefaultTTL: time.Minute})

	if err := client.Set(context.Background(), "profile:1", profile{ID: 1}); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	remote.deleteErr = wantErr
	err := client.Delete(context.Background(), "profile:1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Delete error = %v, want %v", err, wantErr)
	}

	delete(remote.values, "profile:1")
	_, err = client.Get(context.Background(), "profile:1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after failed delete = %v, want %v", err, ErrNotFound)
	}
}

func TestInvalidateTagsDeletesLocalAndRemoteKeys(t *testing.T) {
	remote := newFakeRemoteStore()
	bytes := mustEncode(t, profile{ID: 3, Name: "Tagged"})
	remote.values["profile:3"] = fakeRemoteValue{bytes: bytes, ttl: time.Minute}
	remote.tags["user"] = map[string]struct{}{"profile:3": {}}

	client := mustNewClient[profile](t, remote, &Config{DefaultTTL: time.Minute})
	if _, err := client.Get(context.Background(), "profile:3"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if err := client.InvalidateTags(context.Background(), "user"); err != nil {
		t.Fatalf("InvalidateTags returned error: %v", err)
	}

	_, err := client.Get(context.Background(), "profile:3")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after invalidate = %v, want %v", err, ErrNotFound)
	}
}

func TestKeyPrefixScopesKeysAndTags(t *testing.T) {
	remote := newFakeRemoteStore()
	client := mustNewClient[profile](t, remote, &Config{
		DefaultTTL: time.Minute,
		KeyPrefix:  "app:",
	})

	if err := client.Set(context.Background(), "profile:4", profile{ID: 4}, WithTags("user")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	if _, exists := remote.values["app:profile:4"]; !exists {
		t.Fatal("remote value was not written with key prefix")
	}
	if _, exists := remote.tags["app:user"]["app:profile:4"]; !exists {
		t.Fatal("remote tag was not written with key prefix")
	}
}

func mustNewClient[T any](t *testing.T, remote RemoteStore, cfg *Config) Client[T] {
	t.Helper()

	client, err := New[T](remote, cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return client
}

func mustEncode[T any](t *testing.T, value T) []byte {
	t.Helper()

	bytes, err := encodeValue(value)
	if err != nil {
		t.Fatalf("encodeValue returned error: %v", err)
	}
	return bytes
}

type fakeRemoteValue struct {
	bytes []byte
	ttl   time.Duration
}

type fakeRemoteStore struct {
	values    map[string]fakeRemoteValue
	tags      map[string]map[string]struct{}
	getCount  int
	setErr    error
	deleteErr error
	tagErr    error
}

func newFakeRemoteStore() *fakeRemoteStore {
	return &fakeRemoteStore{
		values: make(map[string]fakeRemoteValue),
		tags:   make(map[string]map[string]struct{}),
	}
}

func (s *fakeRemoteStore) Get(_ context.Context, key string) ([]byte, time.Duration, error) {
	s.getCount++
	value, exists := s.values[key]
	if !exists {
		return nil, 0, ErrNotFound
	}
	return value.bytes, value.ttl, nil
}

func (s *fakeRemoteStore) Set(_ context.Context, key string, value []byte, ttl time.Duration, tags []string, tagsTTL time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}

	s.values[key] = fakeRemoteValue{bytes: cloneBytes(value), ttl: ttl}
	for _, tag := range tags {
		keys, exists := s.tags[tag]
		if !exists {
			keys = make(map[string]struct{})
			s.tags[tag] = keys
		}
		keys[key] = struct{}{}
	}
	if tagsTTL < 0 {
		return ErrInvalidTTL
	}
	return nil
}

func (s *fakeRemoteStore) Delete(_ context.Context, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.values, key)
	for tag, keys := range s.tags {
		delete(keys, key)
		if len(keys) == 0 {
			delete(s.tags, tag)
		}
	}
	return nil
}

func (s *fakeRemoteStore) InvalidateTags(_ context.Context, tags []string) ([]string, error) {
	keys := make([]string, 0)
	for _, tag := range tags {
		for key := range s.tags[tag] {
			keys = append(keys, key)
			delete(s.values, key)
		}
		delete(s.tags, tag)
	}
	sort.Strings(keys)
	if s.tagErr != nil {
		return keys, s.tagErr
	}
	return keys, nil
}

var _ RemoteStore = (*fakeRemoteStore)(nil)

func TestFakeRemoteStoreInvalidatesDeterministically(t *testing.T) {
	store := newFakeRemoteStore()
	store.tags["tag"] = map[string]struct{}{"b": {}, "a": {}}

	got, err := store.InvalidateTags(context.Background(), []string{"tag"})
	if err != nil {
		t.Fatalf("InvalidateTags returned error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("keys = %v, want [a b]", got)
	}
}
