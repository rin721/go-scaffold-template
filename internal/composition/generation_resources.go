package composition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	cacheapp "github.com/rin721/go-scaffold-template/internal/kernel/app/cache"
	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
	i18napp "github.com/rin721/go-scaffold-template/internal/kernel/app/i18n"
	loggerapp "github.com/rin721/go-scaffold-template/internal/kernel/app/logger"
	storageapp "github.com/rin721/go-scaffold-template/internal/kernel/app/storage"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	pkgi18n "github.com/rin721/go-scaffold-template/pkg/i18n"
	pkglogger "github.com/rin721/go-scaffold-template/pkg/logger"
)

type resourceBuilder[T any] func(context.Context) (T, func(context.Context) error, error)

type resourcePool[T any] struct {
	name string
	mu   sync.Mutex
	byID map[string]*resourceEntry[T]
}

type resourceEntry[T any] struct {
	value T
	close func(context.Context) error
	refs  int
}

type resourceHandle[T any] struct {
	pool   *resourcePool[T]
	digest string
	entry  *resourceEntry[T]

	mu       sync.Mutex
	released bool
	closeErr error
}

func newResourcePool[T any](name string) *resourcePool[T] {
	return &resourcePool[T]{name: name, byID: make(map[string]*resourceEntry[T])}
}

func (p *resourcePool[T]) acquire(ctx context.Context, digest string, build resourceBuilder[T]) (*resourceHandle[T], bool, error) {
	if p == nil || p.name == "" {
		return nil, false, fmt.Errorf("resource pool is invalid")
	}
	if ctx == nil {
		return nil, false, fmt.Errorf("resource pool %s context is nil", p.name)
	}
	if digest == "" {
		return nil, false, fmt.Errorf("resource pool %s digest is empty", p.name)
	}
	if build == nil {
		return nil, false, fmt.Errorf("resource pool %s builder is nil", p.name)
	}
	p.mu.Lock()
	if current := p.byID[digest]; current != nil {
		current.refs++
		p.mu.Unlock()
		return &resourceHandle[T]{pool: p, digest: digest, entry: current}, true, nil
	}
	p.mu.Unlock()

	value, closeResource, err := build(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("build %s resource: %w", p.name, err)
	}
	if closeResource == nil {
		return nil, false, fmt.Errorf("build %s resource returned nil finalizer", p.name)
	}
	created := &resourceEntry[T]{value: value, close: closeResource, refs: 1}
	p.mu.Lock()
	if current := p.byID[digest]; current != nil {
		current.refs++
		p.mu.Unlock()
		closeErr := closeResource(context.WithoutCancel(ctx))
		if closeErr != nil {
			return nil, false, fmt.Errorf("close duplicate %s resource: %w", p.name, closeErr)
		}
		return &resourceHandle[T]{pool: p, digest: digest, entry: current}, true, nil
	}
	p.byID[digest] = created
	p.mu.Unlock()
	return &resourceHandle[T]{pool: p, digest: digest, entry: created}, false, nil
}

func (h *resourceHandle[T]) value() T {
	return h.entry.value
}

func (h *resourceHandle[T]) release(ctx context.Context) error {
	if h == nil || h.pool == nil || h.entry == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.released {
		return h.closeErr
	}
	h.released = true
	p := h.pool
	p.mu.Lock()
	current := p.byID[h.digest]
	if current != h.entry || current.refs <= 0 {
		p.mu.Unlock()
		return fmt.Errorf("resource pool %s reference invariant failed", p.name)
	}
	current.refs--
	if current.refs > 0 {
		p.mu.Unlock()
		return nil
	}
	delete(p.byID, h.digest)
	closeResource := current.close
	p.mu.Unlock()
	if err := closeResource(ctx); err != nil {
		h.closeErr = fmt.Errorf("close %s resource: %w", p.name, err)
		return h.closeErr
	}
	return nil
}

func (p *resourcePool[T]) remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.byID)
}

type immutableComponent[T any] struct {
	coordinator *kernel.Coordinator
	output      T
}

func startImmutableComponent[T any](
	ctx context.Context,
	snapshot config.Snapshot,
	logging *kernellogging.Manager,
	definition app.Definition[T],
) (*immutableComponent[T], error) {
	binding, configured := app.Configuration(definition)
	if !configured {
		return nil, fmt.Errorf("immutable component has no configuration binding")
	}
	isolated, err := isolateComponentSnapshot(ctx, snapshot, binding.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("isolate component %s configuration: %w", binding.CapabilityID, err)
	}
	loader := config.New(config.MapSource("application-generation", isolated.Data()))
	runtime, err := kernel.New(loader, kernel.Options{Logging: logging})
	if err != nil {
		return nil, err
	}
	plan := app.NewPlan()
	added, err := app.Add(plan, definition)
	if err != nil {
		return nil, err
	}
	frozen, err := plan.Freeze()
	if err != nil {
		return nil, err
	}
	if err := runtime.Install(frozen); err != nil {
		return nil, err
	}
	coordinator, err := kernel.NewCoordinator(runtime)
	if err != nil {
		return nil, err
	}
	if err := coordinator.Start(ctx); err != nil {
		return nil, err
	}
	return &immutableComponent[T]{coordinator: coordinator, output: added.Output}, nil
}

func (c *immutableComponent[T]) close(ctx context.Context) error {
	if c == nil || c.coordinator == nil {
		return nil
	}
	return c.coordinator.Stop(ctx)
}

func startImmutableLogger(
	ctx context.Context,
	snapshot config.Snapshot,
	baseline *kernellogging.Manager,
) (*immutableComponent[pkglogger.Logger], error) {
	local, err := kernellogging.New(baseline.Logger())
	if err != nil {
		return nil, err
	}
	replacement, err := loggerapp.Replacement()
	if err != nil {
		return nil, err
	}
	binding, configured := app.ReplacementConfiguration(replacement)
	if !configured {
		return nil, fmt.Errorf("configured logger replacement has no configuration binding")
	}
	isolated, err := isolateComponentSnapshot(ctx, snapshot, binding.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("isolate configured logger replacement: %w", err)
	}
	loader := config.New(config.MapSource("application-generation", isolated.Data()))
	runtime, err := kernel.New(loader, kernel.Options{Logging: local})
	if err != nil {
		return nil, err
	}
	plan := app.NewPlan()
	builtin, err := app.Value[kernellogging.Target](kernel.BuiltinLoggerID, local)
	if err != nil {
		return nil, err
	}
	added, err := app.Add(plan, builtin)
	if err != nil {
		return nil, err
	}
	if err := app.Replace(plan, added.Binding, replacement); err != nil {
		return nil, err
	}
	frozen, err := plan.Freeze()
	if err != nil {
		return nil, err
	}
	if err := runtime.Install(frozen); err != nil {
		return nil, err
	}
	coordinator, err := kernel.NewCoordinator(runtime)
	if err != nil {
		return nil, err
	}
	if err := coordinator.Start(ctx); err != nil {
		return nil, err
	}
	return &immutableComponent[pkglogger.Logger]{coordinator: coordinator, output: local.Logger()}, nil
}

// isolateComponentSnapshot 只保留目标 binding 所属的顶层配置对象。
// 独立 immutable component 仍执行自己的严格未知字段校验，但不会把同一
// Application Generation 中由其他 owner 管理的配置节误判为未知配置。
func isolateComponentSnapshot(ctx context.Context, snapshot config.Snapshot, path string) (config.Snapshot, error) {
	if ctx == nil {
		return config.Snapshot{}, fmt.Errorf("component snapshot context is nil")
	}
	trimmed := strings.Trim(path, ".")
	if trimmed == "" {
		return config.Snapshot{}, fmt.Errorf("component config path is empty")
	}
	root := strings.SplitN(trimmed, ".", 2)[0]
	values := make(map[string]any, 1)
	if value, exists := snapshot.Value(root); exists {
		values[root] = value
	}
	return config.New(config.MapSource("application-generation-section", values)).Load(ctx)
}

func databaseDefinition() (app.Definition[databaseapp.Access], error) {
	return databaseapp.Definition()
}
func cacheDefinition() (app.Definition[cacheapp.Access], error)     { return cacheapp.Definition() }
func i18nDefinition() (app.Definition[pkgi18n.Translator], error)   { return i18napp.Definition() }
func storageDefinition() (app.Definition[storageapp.Access], error) { return storageapp.Definition() }

func releaseGenerationResources(
	ctx context.Context,
	storage *resourceHandle[storageapp.Access],
	i18n *resourceHandle[pkgi18n.Translator],
	cache *resourceHandle[cacheapp.Access],
	database *resourceHandle[databaseapp.Access],
	logging *resourceHandle[pkglogger.Logger],
) error {
	var joined error
	if storage != nil {
		joined = errors.Join(joined, storage.release(ctx))
	}
	if i18n != nil {
		joined = errors.Join(joined, i18n.release(ctx))
	}
	if cache != nil {
		joined = errors.Join(joined, cache.release(ctx))
	}
	if database != nil {
		joined = errors.Join(joined, database.release(ctx))
	}
	if logging != nil {
		joined = errors.Join(joined, logging.release(ctx))
	}
	return joined
}
