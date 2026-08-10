package cache

import (
	"context"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/pkg/concurrency"
)

func TestGetOrLoadStoresLoadedValue(t *testing.T) {
	client := mustNewClient[string](t, newFakeRemoteStore(), &Config{DefaultTTL: time.Minute})
	calls := 0
	value, err := GetOrLoad(context.Background(), &concurrency.SingleFlight{}, client, "k", func(context.Context) (string, error) {
		calls++
		return "loaded", nil
	})
	if err != nil || value != "loaded" || calls != 1 {
		t.Fatalf("GetOrLoad() value=%q err=%v calls=%d", value, err, calls)
	}
}
