package composition

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	cacheapp "github.com/rin721/go-scaffold-template/internal/kernel/app/cache"
	databaseapp "github.com/rin721/go-scaffold-template/internal/kernel/app/database"
	storageapp "github.com/rin721/go-scaffold-template/internal/kernel/app/storage"
	kernelcomposition "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	kernellogging "github.com/rin721/go-scaffold-template/internal/kernel/logging"
	"github.com/rin721/go-scaffold-template/internal/module"
	"github.com/rin721/go-scaffold-template/internal/module/auth"
	authconfig "github.com/rin721/go-scaffold-template/internal/module/auth/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/todo"
	configbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
	migrationbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/migration"
	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/httpx"
	"github.com/rin721/go-scaffold-template/pkg/i18n"
	"github.com/rin721/go-scaffold-template/pkg/idgen"
	"github.com/rin721/go-scaffold-template/pkg/logger"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
	"github.com/rin721/go-scaffold-template/pkg/validation"
)

type applicationGenerationFactory struct {
	logging *kernellogging.Manager
	hub     *httpx.ListenerHub

	loggerPool   *resourcePool[logger.Logger]
	databasePool *resourcePool[databaseapp.Access]
	cachePool    *resourcePool[cacheapp.Access]
	i18nPool     *resourcePool[i18n.Translator]
	storagePool  *resourcePool[storageapp.Access]

	nextID   atomic.Uint64
	failures chan error
}

type applicationGeneration struct {
	factory  *applicationGenerationFactory
	id       uint64
	snapshot config.Snapshot

	logger   *resourceHandle[logger.Logger]
	database *resourceHandle[databaseapp.Access]
	cache    *resourceHandle[cacheapp.Access]
	i18n     *resourceHandle[i18n.Translator]
	storage  *resourceHandle[storageapp.Access]

	module       todo.Module
	authModule   auth.Module
	participants []supervisor.Participant
	route        *httpx.PreparedRoute
	server       *httpx.Server
	runDone      chan struct{}
	runErr       error

	activeRequests      atomic.Int64
	resourceStats       kernel.GenerationResourceStats
	stopping            atomic.Bool
	settleMu            sync.Mutex
	participantStopDone bool
	participantStopErr  error
	mu                  sync.Mutex
	committed           bool
	current             bool
	settled             bool
	terminalErr         error
}

func newApplicationGenerationFactory(logging *kernellogging.Manager) (*applicationGenerationFactory, error) {
	if logging == nil {
		return nil, fmt.Errorf("application generation logging manager is nil")
	}
	hub, err := httpx.NewListenerHub(0)
	if err != nil {
		return nil, err
	}
	return &applicationGenerationFactory{
		logging: logging, hub: hub,
		loggerPool:   newResourcePool[logger.Logger]("logger"),
		databasePool: newResourcePool[databaseapp.Access]("database"),
		cachePool:    newResourcePool[cacheapp.Access]("cache"),
		i18nPool:     newResourcePool[i18n.Translator]("i18n"),
		storagePool:  newResourcePool[storageapp.Access]("storage"),
		failures:     make(chan error, 8),
	}, nil
}

func (f *applicationGenerationFactory) Prepare(
	ctx context.Context,
	snapshot config.Snapshot,
	previous kernel.ActiveGeneration,
) (kernel.PreparedGeneration, error) {
	if ctx == nil {
		return nil, fmt.Errorf("application generation prepare context is nil")
	}
	generation := &applicationGeneration{
		factory: f, id: f.nextID.Add(1), snapshot: snapshot, runDone: make(chan struct{}),
	}
	abort := func(cause error) (kernel.PreparedGeneration, error) {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kernel.DefaultReloadTimeout)
		defer cancel()
		cleanupErr := generation.abort(cleanupCtx)
		return nil, errors.Join(cause, cleanupErr)
	}

	loggerDigest, err := snapshot.SectionDigest("logger")
	if err != nil {
		return abort(err)
	}
	var reused bool
	generation.logger, reused, err = f.loggerPool.acquire(ctx, loggerDigest, func(ctx context.Context) (logger.Logger, func(context.Context) error, error) {
		component, buildErr := startImmutableLogger(ctx, snapshot, f.logging)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return component.output, component.close, nil
	})
	if err != nil {
		return abort(err)
	}
	generation.recordResource("logger", reused)

	databaseDigest, err := snapshot.SectionDigest("database")
	if err != nil {
		return abort(err)
	}
	generation.database, reused, err = f.databasePool.acquire(ctx, databaseDigest, func(ctx context.Context) (databaseapp.Access, func(context.Context) error, error) {
		definition, buildErr := databaseDefinition()
		if buildErr != nil {
			return nil, nil, buildErr
		}
		component, buildErr := startImmutableComponent(ctx, snapshot, f.logging, definition)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return component.output, component.close, nil
	})
	if err != nil {
		return abort(err)
	}
	generation.recordResource("database", reused)

	cacheDigest, err := snapshot.SectionDigest("cache")
	if err != nil {
		return abort(err)
	}
	generation.cache, reused, err = f.cachePool.acquire(ctx, cacheDigest, func(ctx context.Context) (cacheapp.Access, func(context.Context) error, error) {
		definition, buildErr := cacheDefinition()
		if buildErr != nil {
			return nil, nil, buildErr
		}
		component, buildErr := startImmutableComponent(ctx, snapshot, f.logging, definition)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return component.output, component.close, nil
	})
	if err != nil {
		return abort(err)
	}
	generation.recordResource("cache", reused)

	i18nDigest, err := snapshot.SectionDigest("i18n")
	if err != nil {
		return abort(err)
	}
	generation.i18n, reused, err = f.i18nPool.acquire(ctx, i18nDigest, func(ctx context.Context) (i18n.Translator, func(context.Context) error, error) {
		definition, buildErr := i18nDefinition()
		if buildErr != nil {
			return nil, nil, buildErr
		}
		component, buildErr := startImmutableComponent(ctx, snapshot, f.logging, definition)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return component.output, component.close, nil
	})
	if err != nil {
		return abort(err)
	}
	generation.recordResource("i18n", reused)

	storageDigest, err := snapshot.SectionDigest("storage")
	if err != nil {
		return abort(err)
	}
	generation.storage, reused, err = f.storagePool.acquire(ctx, storageDigest, func(ctx context.Context) (storageapp.Access, func(context.Context) error, error) {
		definition, buildErr := storageDefinition()
		if buildErr != nil {
			return nil, nil, buildErr
		}
		component, buildErr := startImmutableComponent(ctx, snapshot, f.logging, definition)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		return component.output, component.close, nil
	})
	if err != nil {
		return abort(err)
	}
	generation.recordResource("storage", reused)

	authConfig, err := authconfig.Decode(snapshot)
	if err != nil {
		return abort(err)
	}
	policies, err := operationPolicies()
	if err != nil {
		return abort(err)
	}
	generation.authModule, err = auth.NewHTTP(auth.Dependencies{
		Clock: clock.System(), Logger: generation.logger.value(), Config: authConfig, Policies: policies,
	})
	if err != nil {
		return abort(err)
	}
	authorizer, err := newTodoAuthorizerAdapter(generation.authModule.Service)
	if err != nil {
		return abort(err)
	}

	todoConfig, err := configbinding.Decode(snapshot)
	if err != nil {
		return abort(err)
	}
	databaseAccess, err := adaptDatabaseAccess(generation.database.value())
	if err != nil {
		return abort(err)
	}
	compatibility, err := migrationbinding.NewCompatibility(databaseAccess)
	if err != nil {
		return abort(err)
	}
	if err := compatibility.Check(ctx); err != nil {
		return abort(fmt.Errorf("verify todo migration compatibility: %w", err))
	}
	databaseConfig, err := databaseapp.Decode(snapshot)
	if err != nil {
		return abort(err)
	}
	migrationCompletion, err := migrationbinding.NewCompletion(databaseConfig.PackageConfig())
	if err != nil {
		return abort(err)
	}
	if err := migrationCompletion.Verify(ctx); err != nil {
		return abort(fmt.Errorf("verify todo migration completion: %w", err))
	}
	generation.module, err = todo.New(todo.Dependencies{
		Database: databaseAccess, Clock: clock.System(), IDGenerator: idgen.UUID(),
		Config: todoConfig, Authorizer: authorizer,
	})
	if err != nil {
		return abort(err)
	}
	generation.resourceStats.Built = append(generation.resourceStats.Built, "todo")

	if err := module.ValidateContributions(generation.authModule.Contribution, generation.module.Contribution); err != nil {
		return abort(fmt.Errorf("validate application module contributions: %w", err))
	}
	generation.resourceStats.Built = append(generation.resourceStats.Built, "auth")
	participants := append(append([]supervisor.Participant(nil), generation.module.Contribution.Participants...), generation.authModule.Contribution.Participants...)
	for _, participant := range participants {
		if err := participant.Start(ctx); err != nil {
			return abort(fmt.Errorf("start generation participant %s: %w", participant.Name(), err))
		}
		generation.participants = append(generation.participants, participant)
	}

	httpConfig, err := kernelcomposition.HTTPServerConfig(snapshot)
	if err != nil {
		return abort(err)
	}
	router, err := applicationRouter(kernelcomposition.Capabilities{
		Logger: generation.logger.value(), Clock: clock.System(), IDGenerator: idgen.UUID(), Validator: validation.New(),
		Database: generation.database.value(), Cache: generation.cache.value(),
		I18n: generation.i18n.value(), Storage: generation.storage.value(),
	}, httpConfig, generation.module.Service, generation.authModule)
	if err != nil {
		return abort(err)
	}
	generation.route, err = f.hub.Prepare(ctx, httpConfig.Addr)
	if err != nil {
		return abort(err)
	}
	generation.server, err = httpx.NewServer(&httpConfig, generation.trackRequests(router))
	if err != nil {
		return abort(err)
	}
	generation.resourceStats.Built = append(generation.resourceStats.Built, "http")
	if err := generation.server.StartWithListener(ctx, generation.route.Listener()); err != nil {
		return abort(err)
	}
	go generation.runServer()
	select {
	case <-ctx.Done():
		return abort(ctx.Err())
	case <-generation.server.Running():
		return generation, nil
	case <-generation.runDone:
		return abort(fmt.Errorf("candidate HTTP server exited before ready: %w", generation.serverRunError()))
	}
}

func (f *applicationGenerationFactory) Failures() <-chan error { return f.failures }

func (f *applicationGenerationFactory) Stop(ctx context.Context) error {
	var joined error
	joined = errors.Join(joined, f.hub.Stop(ctx))
	remaining := map[string]int{
		"logger": f.loggerPool.remaining(), "database": f.databasePool.remaining(),
		"cache": f.cachePool.remaining(), "i18n": f.i18nPool.remaining(), "storage": f.storagePool.remaining(),
	}
	for owner, count := range remaining {
		if count != 0 {
			joined = errors.Join(joined, fmt.Errorf("resource pool %s retains %d entries", owner, count))
		}
	}
	return joined
}

func (g *applicationGeneration) ID() uint64 { return g.id }

func (g *applicationGeneration) Snapshot() config.Snapshot { return g.snapshot }

func (g *applicationGeneration) ConfiguredAddress() string {
	if g.route == nil {
		return ""
	}
	return g.route.ConfiguredAddress()
}

func (g *applicationGeneration) BoundAddress() string {
	if g.route == nil || g.route.BoundAddress() == nil {
		return ""
	}
	return g.route.BoundAddress().String()
}

func (g *applicationGeneration) ActiveRequests() int64 { return g.activeRequests.Load() }

func (g *applicationGeneration) ActiveConnections() int64 {
	if g.route == nil {
		return 0
	}
	return g.route.ActiveConnections()
}

func (g *applicationGeneration) ResourceStats() kernel.GenerationResourceStats {
	return kernel.GenerationResourceStats{
		Reused: append([]string(nil), g.resourceStats.Reused...),
		Built:  append([]string(nil), g.resourceStats.Built...),
	}
}

func (g *applicationGeneration) recordResource(owner string, reused bool) {
	if reused {
		g.resourceStats.Reused = append(g.resourceStats.Reused, owner)
		return
	}
	g.resourceStats.Built = append(g.resourceStats.Built, owner)
}

func (g *applicationGeneration) Commit(previous kernel.ActiveGeneration) (kernel.ActiveGeneration, error) {
	g.mu.Lock()
	if g.committed || g.settled {
		g.mu.Unlock()
		return nil, fmt.Errorf("application generation %d is already settled", g.id)
	}
	var previousGeneration *applicationGeneration
	if previous != nil {
		var ok bool
		previousGeneration, ok = previous.(*applicationGeneration)
		if !ok || previousGeneration.factory != g.factory {
			g.mu.Unlock()
			return nil, fmt.Errorf("previous application generation is incompatible")
		}
	}
	var previousRoute *httpx.PreparedRoute
	if previousGeneration != nil {
		previousRoute = previousGeneration.route
	}
	if _, err := g.factory.hub.Commit(g.route, previousRoute); err != nil {
		g.mu.Unlock()
		return nil, err
	}
	g.factory.logging.Replace(g.logger.value())
	g.committed = true
	g.current = true
	if previousGeneration != nil {
		previousGeneration.mu.Lock()
		previousGeneration.current = false
		previousGeneration.mu.Unlock()
	}
	g.mu.Unlock()
	return g, nil
}

func (g *applicationGeneration) Abort(ctx context.Context) error { return g.abort(ctx) }

func (g *applicationGeneration) abort(ctx context.Context) error {
	if g == nil {
		return nil
	}
	g.settleMu.Lock()
	defer g.settleMu.Unlock()
	g.mu.Lock()
	if g.committed {
		g.mu.Unlock()
		return fmt.Errorf("cannot abort committed application generation %d", g.id)
	}
	if g.settled {
		terminalErr := g.terminalErr
		g.mu.Unlock()
		return terminalErr
	}
	g.stopping.Store(true)
	g.mu.Unlock()
	var joined error
	if g.server != nil {
		joined = errors.Join(joined, g.server.Stop(ctx))
		joined = errors.Join(joined, g.waitRun(ctx))
	}
	if g.route != nil {
		joined = errors.Join(joined, g.factory.hub.Abort(g.route))
	}
	joined = errors.Join(joined, g.stopParticipants(ctx))
	joined = errors.Join(joined, releaseGenerationResources(
		ctx, g.storage, g.i18n, g.cache, g.database, g.logger,
	))
	g.mu.Lock()
	g.settled = true
	g.terminalErr = joined
	g.mu.Unlock()
	return joined
}

func (g *applicationGeneration) Retire(ctx context.Context) error {
	return g.stop(ctx, false)
}

func (g *applicationGeneration) Stop(ctx context.Context) error {
	return g.stop(ctx, true)
}

func (g *applicationGeneration) stop(ctx context.Context, retireCurrentRoute bool) error {
	if g == nil {
		return nil
	}
	g.settleMu.Lock()
	defer g.settleMu.Unlock()
	g.mu.Lock()
	if !g.committed {
		g.mu.Unlock()
		return fmt.Errorf("application generation %d is not committed", g.id)
	}
	if g.settled {
		terminalErr := g.terminalErr
		g.mu.Unlock()
		return terminalErr
	}
	wasCurrent := g.current
	g.stopping.Store(true)
	g.mu.Unlock()
	var joined error
	if retireCurrentRoute && wasCurrent {
		joined = errors.Join(joined, g.factory.hub.Retire(g.route))
	}
	if g.route != nil {
		joined = errors.Join(joined, g.route.WaitDrained(ctx))
	}
	if joined == nil && g.server != nil {
		joined = errors.Join(joined, g.server.Stop(ctx), g.waitRun(ctx))
	}
	if joined == nil && g.route != nil {
		joined = errors.Join(joined, g.factory.hub.Release(g.route))
	}
	if joined == nil {
		joined = errors.Join(joined, g.stopParticipants(ctx))
	}
	terminal := false
	if joined == nil {
		terminal = true
		if wasCurrent {
			g.factory.logging.Restore()
		}
		joined = releaseGenerationResources(ctx, g.storage, g.i18n, g.cache, g.database, g.logger)
	}
	if terminal {
		g.mu.Lock()
		g.settled = true
		g.terminalErr = joined
		g.mu.Unlock()
	}
	return joined
}

func (g *applicationGeneration) ForceStop(ctx context.Context) error {
	if g == nil {
		return nil
	}
	g.settleMu.Lock()
	defer g.settleMu.Unlock()
	g.mu.Lock()
	if !g.committed {
		g.mu.Unlock()
		return fmt.Errorf("application generation %d is not committed", g.id)
	}
	if g.settled {
		terminalErr := g.terminalErr
		g.mu.Unlock()
		return terminalErr
	}
	wasCurrent := g.current
	g.stopping.Store(true)
	g.mu.Unlock()
	var joined error
	joined = errors.Join(joined, g.factory.hub.Retire(g.route))
	if g.server != nil {
		joined = errors.Join(joined, g.server.ForceStop(ctx), g.waitRun(ctx))
	}
	if joined == nil && g.route != nil {
		joined = errors.Join(joined, g.factory.hub.Release(g.route))
	}
	if joined == nil {
		joined = errors.Join(joined, g.stopParticipants(ctx))
	}
	terminal := false
	if joined == nil {
		terminal = true
		if wasCurrent {
			g.factory.logging.Restore()
		}
		joined = releaseGenerationResources(ctx, g.storage, g.i18n, g.cache, g.database, g.logger)
	}
	if terminal {
		g.mu.Lock()
		g.settled = true
		g.terminalErr = joined
		g.mu.Unlock()
	}
	return joined
}

func (g *applicationGeneration) runServer() {
	err := g.server.Run(context.Background())
	g.mu.Lock()
	g.runErr = err
	close(g.runDone)
	g.mu.Unlock()
	if err != nil && !g.stopping.Load() {
		select {
		case g.factory.failures <- fmt.Errorf("run HTTP generation %d: %w", g.id, err):
		default:
		}
	}
}

func (g *applicationGeneration) waitRun(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.runDone:
		return g.serverRunError()
	}
}

func (g *applicationGeneration) serverRunError() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.runErr
}

func (g *applicationGeneration) stopParticipants(ctx context.Context) error {
	if g.participantStopDone {
		return g.participantStopErr
	}
	g.participantStopDone = true
	var joined error
	for index := len(g.participants) - 1; index >= 0; index-- {
		participant := g.participants[index]
		if participant == nil {
			continue
		}
		if err := participant.Stop(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("stop generation participant %s: %w", participant.Name(), err))
		}
	}
	g.participantStopErr = joined
	return g.participantStopErr
}

func (g *applicationGeneration) trackRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		g.activeRequests.Add(1)
		defer g.activeRequests.Add(-1)
		next.ServeHTTP(response, request)
	})
}

var _ kernel.GenerationFactory = (*applicationGenerationFactory)(nil)
var _ kernel.PreparedGeneration = (*applicationGeneration)(nil)
var _ kernel.ActiveGeneration = (*applicationGeneration)(nil)
