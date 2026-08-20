package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rin721/go-scaffold-template/pkg/coordination"
)

const minimumLeaseTTL = time.Millisecond

var (
	renewLeaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)
	releaseLeaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)
)

type coordinationAccess struct{ backend Access }

func (a *coordinationAccess) Acquire(ctx context.Context, key coordination.Key, options coordination.LeaseOptions) (coordination.Lease, error) {
	if err := coordination.ValidateAcquire(ctx, key, options); err != nil {
		return nil, err
	}
	var acquired coordination.Lease
	err := a.backend.useCoordination(ctx, func(manager coordination.Manager) error {
		var err error
		acquired, err = manager.Acquire(ctx, key, options)
		return err
	})
	if err != nil {
		return nil, normalizeAccessError("acquire", err)
	}
	return &coordinationLeaseAccess{backend: a.backend, lease: acquired, key: key}, nil
}

type coordinationLeaseAccess struct {
	backend Access
	lease   coordination.Lease
	key     coordination.Key
}

func (l *coordinationLeaseAccess) Renew(ctx context.Context, options coordination.LeaseOptions) error {
	if err := coordination.ValidateAcquire(ctx, l.key, options); err != nil {
		return err
	}
	err := l.backend.useCoordination(ctx, func(coordination.Manager) error {
		return l.lease.Renew(ctx, options)
	})
	return normalizeAccessError("renew", err)
}

func (l *coordinationLeaseAccess) Release(ctx context.Context) error {
	if ctx == nil {
		return coordination.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err := l.backend.useCoordination(ctx, func(coordination.Manager) error {
		return l.lease.Release(ctx)
	})
	return normalizeAccessError("release", err)
}

func normalizeAccessError(operation string, err error) error {
	if err == nil || errors.Is(err, coordination.ErrNotAcquired) || errors.Is(err, coordination.ErrLeaseLost) ||
		errors.Is(err, coordination.ErrInvalidKey) || errors.Is(err, coordination.ErrInvalidTTL) ||
		errors.Is(err, coordination.ErrNilContext) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: cache coordination %s: %w", coordination.ErrUnavailable, operation, err)
}

type redisCoordination struct{ client redis.UniversalClient }

func newRedisCoordination(client redis.UniversalClient) coordination.Manager {
	if client == nil {
		return coordination.Unavailable()
	}
	return &redisCoordination{client: client}
}

func (m *redisCoordination) Acquire(ctx context.Context, key coordination.Key, options coordination.LeaseOptions) (coordination.Lease, error) {
	if err := coordination.ValidateAcquire(ctx, key, options); err != nil {
		return nil, err
	}
	if options.TTL < minimumLeaseTTL {
		return nil, fmt.Errorf("%w: ttl must be at least %s", coordination.ErrInvalidTTL, minimumLeaseTTL)
	}
	token := uuid.NewString()
	acquired, err := m.client.SetNX(ctx, string(key), token, options.TTL).Result()
	if err != nil {
		return nil, backendError("acquire", err)
	}
	if !acquired {
		return nil, coordination.ErrNotAcquired
	}
	return &redisLease{client: m.client, key: string(key), token: token}, nil
}

type redisLease struct {
	client redis.UniversalClient
	key    string
	token  string
}

func (l *redisLease) Renew(ctx context.Context, options coordination.LeaseOptions) error {
	if err := coordination.ValidateAcquire(ctx, coordination.Key(l.key), options); err != nil {
		return err
	}
	if options.TTL < minimumLeaseTTL {
		return fmt.Errorf("%w: ttl must be at least %s", coordination.ErrInvalidTTL, minimumLeaseTTL)
	}
	updated, err := renewLeaseScript.Run(ctx, l.client, []string{l.key}, l.token, options.TTL.Milliseconds()).Int64()
	if err != nil {
		return backendError("renew", err)
	}
	if updated != 1 {
		return coordination.ErrLeaseLost
	}
	return nil
}

func (l *redisLease) Release(ctx context.Context) error {
	if ctx == nil {
		return coordination.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	released, err := releaseLeaseScript.Run(ctx, l.client, []string{l.key}, l.token).Int64()
	if err != nil {
		return backendError("release", err)
	}
	if released != 1 {
		return coordination.ErrLeaseLost
	}
	return nil
}

func backendError(operation string, err error) error {
	return fmt.Errorf("%w: redis lease %s: %w", coordination.ErrUnavailable, operation, err)
}

var _ coordination.Manager = (*coordinationAccess)(nil)
var _ coordination.Lease = (*coordinationLeaseAccess)(nil)
var _ coordination.Manager = (*redisCoordination)(nil)
var _ coordination.Lease = (*redisLease)(nil)
