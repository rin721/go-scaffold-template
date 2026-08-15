package database

import (
	"context"
	"sync"
)

// Borrow 创建仅在 use 回调期间有效的非所有者 Client。
//
// 回调返回后，借用 Client、由其创建的 Repository 以及 Tx 都会返回
// ErrClientUnavailable，避免资源租约结束后继续访问已经换代或关闭的连接池。
func Borrow(ctx context.Context, client Client, use func(Client) error) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if use == nil {
		return ErrNilClientFunc
	}
	if isNilValue(client) {
		return ErrClientUnavailable
	}
	provider, _ := client.(sessionProvider)
	lifetime, cancel := context.WithCancel(ctx)
	state := &borrowState{ctx: lifetime, cancel: cancel}
	borrowed := &borrowedClient{delegate: client, provider: provider, state: state}
	defer state.close()
	return use(borrowed)
}

type borrowState struct {
	mu     sync.RWMutex
	closed bool
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *borrowState) close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
}

func (s *borrowState) operationContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if !s.available() {
		return nil, nil, ErrClientUnavailable
	}
	return contextBoundTo(ctx, s.ctx)
}

func (s *borrowState) available() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.closed
}

type borrowedClient struct {
	delegate Client
	provider sessionProvider
	state    *borrowState
}

func (c *borrowedClient) WithinTx(ctx context.Context, use func(context.Context, Tx) error) error {
	if !c.state.available() {
		return ErrClientUnavailable
	}
	if use == nil {
		return ErrNilTransactionFunc
	}
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	return c.delegate.WithinTx(operationCtx, func(txCtx context.Context, tx Tx) error {
		provider, ok := tx.(sessionProvider)
		if !ok || isNilProvider(provider) {
			return ErrClientUnavailable
		}
		txLifetime, txCancel := context.WithCancel(c.state.ctx)
		txState := &borrowState{ctx: txLifetime, cancel: txCancel}
		defer txState.close()
		return use(txCtx, &borrowedTx{provider: provider, state: txState})
	})
}

func (c *borrowedClient) Migrate(ctx context.Context, schemas ...Schema) error {
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	return c.delegate.Migrate(operationCtx, schemas...)
}

func (c *borrowedClient) CheckSchemas(ctx context.Context, schemas ...Schema) error {
	operationCtx, cancel, err := c.state.operationContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	return c.delegate.CheckSchemas(operationCtx, schemas...)
}

func (c *borrowedClient) databaseSession(ctx context.Context) (any, error) {
	if isNilProvider(c.provider) {
		return nil, ErrClientUnavailable
	}
	operationCtx, _, err := c.state.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	return c.provider.databaseSession(operationCtx)
}

type borrowedTx struct {
	provider sessionProvider
	state    *borrowState
}

func (*borrowedTx) databaseTransaction() {}

func (t *borrowedTx) databaseSession(ctx context.Context) (any, error) {
	operationCtx, _, err := t.state.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	return t.provider.databaseSession(operationCtx)
}

var _ Client = (*borrowedClient)(nil)
var _ Tx = (*borrowedTx)(nil)
