package concurrency

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

func group(ctx context.Context) (*errgroup.Group, context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	return errgroup.WithContext(ctx)
}

// SingleFlight 合并相同 key 的并发调用。
type SingleFlight struct {
	group singleflight.Group
}

func (s *SingleFlight) Do(key string, fn func() (any, error)) (any, error, bool) {
	return s.group.Do(key, fn)
}

// Pool 是固定并发度 worker pool。
type Pool struct {
	workers int
}

// NewPool 创建 worker pool。
func NewPool(workers int) *Pool {
	if workers <= 0 {
		workers = 1
	}
	return &Pool{workers: workers}
}

// Run 执行任务集合，任一任务失败后返回错误。
func (p *Pool) Run(ctx context.Context, tasks []func(context.Context) error) error {
	group, groupCtx := group(ctx)
	queue := make(chan func(context.Context) error)
	group.Go(func() error {
		defer close(queue)
		for _, task := range tasks {
			select {
			case <-groupCtx.Done():
				return groupCtx.Err()
			case queue <- task:
			}
		}
		return nil
	})
	for i := 0; i < p.workers; i++ {
		group.Go(func() error {
			for task := range queue {
				if task == nil {
					continue
				}
				if err := task(groupCtx); err != nil {
					return fmt.Errorf("pool task failed: %w", err)
				}
			}
			return nil
		})
	}
	return group.Wait()
}
