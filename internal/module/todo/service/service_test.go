package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/module/todo/model"
	"github.com/rin721/go-scaffold-template/pkg/clock"
	"github.com/rin721/go-scaffold-template/pkg/fault"
)

const testID = "11111111-1111-4111-8111-111111111111"

var testActor = Actor{Subject: "actor-a", Kind: "service", Scopes: []string{"todos:read", "todos:write"}}

func TestServiceCreateListAndComplete(t *testing.T) {
	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	repository := newMemoryRepository()
	service, err := New(repository, clock.Fixed(now), fixedIDs{id: testID}, Policy{
		TitleMaxRunes: 20, DefaultListLimit: 10, MaxListLimit: 50,
	}, allowAuthorizer{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	created, err := service.Create(t.Context(), CreateCommand{Actor: testActor, Title: "  学习 Go  "})
	if err != nil || created.Title != "学习 Go" || created.Status != model.StatusPending || created.Version != 1 {
		t.Fatalf("Create() = %#v, %v", created, err)
	}
	listed, err := service.List(t.Context(), ListQuery{Actor: testActor})
	if err != nil || listed.Total != 1 || listed.Limit != 10 || len(listed.Items) != 1 {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	completed, err := service.Complete(t.Context(), CompleteCommand{Actor: testActor, ID: testID})
	if err != nil || completed.Status != model.StatusCompleted || completed.Version != 2 {
		t.Fatalf("Complete() = %#v, %v", completed, err)
	}
	repeated, err := service.Complete(t.Context(), CompleteCommand{Actor: testActor, ID: testID})
	if err != nil || repeated.Version != 2 || !repeated.CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatalf("Complete(repeat) = %#v, %v", repeated, err)
	}
}

func TestServiceClassifiesInvalidInputAndRepositoryErrors(t *testing.T) {
	service, err := New(newMemoryRepository(), clock.Fixed(time.Now()), fixedIDs{id: testID}, Policy{
		TitleMaxRunes: 3, DefaultListLimit: 1, MaxListLimit: 2,
	}, allowAuthorizer{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, call := range []func() error{
		func() error {
			_, err := service.Create(t.Context(), CreateCommand{Actor: testActor, Title: "long"})
			return err
		},
		func() error {
			_, err := service.Get(t.Context(), GetQuery{Actor: testActor, ID: "invalid"})
			return err
		},
		func() error { _, err := service.List(t.Context(), ListQuery{Actor: testActor, Limit: 3}); return err },
		func() error {
			_, err := service.List(t.Context(), ListQuery{Actor: testActor, Status: "unknown"})
			return err
		},
	} {
		if err := call(); fault.CodeOf(err) != fault.CodeInvalidArgument {
			t.Fatalf("error = %v, code = %s", err, fault.CodeOf(err))
		}
	}
	_, err = service.Get(t.Context(), GetQuery{Actor: testActor, ID: testID})
	if fault.CodeOf(err) != fault.CodeNotFound {
		t.Fatalf("Get(missing) error = %v, code = %s", err, fault.CodeOf(err))
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.List(cancelled, ListQuery{Actor: testActor}); !errors.Is(err, context.Canceled) {
		t.Fatalf("List(cancelled) error = %v", err)
	}
}

func TestServicePreservesDependencyFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	policy := Policy{TitleMaxRunes: 20, DefaultListLimit: 10, MaxListLimit: 50}
	idFailure := errors.New("id source failed")
	serviceWithIDFailure, err := New(newMemoryRepository(), clock.Fixed(now), fixedIDs{err: idFailure}, policy, allowAuthorizer{})
	if err != nil {
		t.Fatalf("New(id failure) error = %v", err)
	}
	if _, err := serviceWithIDFailure.Create(t.Context(), CreateCommand{Actor: testActor, Title: "学习 Go"}); fault.CodeOf(err) != fault.CodeInternal || !errors.Is(err, idFailure) {
		t.Fatalf("Create(id failure) error = %v", err)
	}

	serviceWithInvalidID, err := New(newMemoryRepository(), clock.Fixed(now), fixedIDs{id: "invalid"}, policy, allowAuthorizer{})
	if err != nil {
		t.Fatalf("New(invalid ID) error = %v", err)
	}
	if _, err := serviceWithInvalidID.Create(t.Context(), CreateCommand{Actor: testActor, Title: "学习 Go"}); fault.CodeOf(err) != fault.CodeInternal {
		t.Fatalf("Create(invalid generated ID) error = %v", err)
	}

	serviceWithZeroClock, err := New(newMemoryRepository(), clock.Fixed(time.Time{}), fixedIDs{id: testID}, policy, allowAuthorizer{})
	if err != nil {
		t.Fatalf("New(zero clock) error = %v", err)
	}
	if _, err := serviceWithZeroClock.Create(t.Context(), CreateCommand{Actor: testActor, Title: "学习 Go"}); fault.CodeOf(err) != fault.CodeInternal {
		t.Fatalf("Create(zero clock) error = %v", err)
	}

	repositoryFailure := fault.Wrap(errors.New("database unavailable"), fault.CodeUnavailable, "test", true)
	serviceWithRepositoryFailure, err := New(failingRepository{err: repositoryFailure}, clock.Fixed(now), fixedIDs{id: testID}, policy, allowAuthorizer{})
	if err != nil {
		t.Fatalf("New(repository failure) error = %v", err)
	}
	if _, err := serviceWithRepositoryFailure.Get(t.Context(), GetQuery{Actor: testActor, ID: testID}); fault.CodeOf(err) != fault.CodeUnavailable || !fault.Retryable(err) || !errors.Is(err, repositoryFailure) {
		t.Fatalf("Get(repository failure) error = %v", err)
	}
}

func TestServiceUsesPersistedOwnerAndHidesCrossActorExistence(t *testing.T) {
	now := time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)
	authorizer := ownerAuthorizer{}
	service, err := New(newMemoryRepository(), clock.Fixed(now), fixedIDs{id: testID}, Policy{
		TitleMaxRunes: 20, DefaultListLimit: 10, MaxListLimit: 50,
	}, authorizer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	actorA := testActor
	actorB := Actor{Subject: "actor-b", Kind: "service", Scopes: append([]string(nil), testActor.Scopes...)}
	created, err := service.Create(t.Context(), CreateCommand{Actor: actorA, Title: "owned"})
	if err != nil || created.OwnerSubject != actorA.Subject {
		t.Fatalf("Create() = %#v, %v", created, err)
	}
	if _, err := service.Get(t.Context(), GetQuery{Actor: actorB, ID: created.ID}); fault.CodeOf(err) != fault.CodeNotFound || !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Get(cross actor) error = %v, code = %s", err, fault.CodeOf(err))
	}
	listed, err := service.List(t.Context(), ListQuery{Actor: actorB})
	if err != nil || listed.Total != 0 || len(listed.Items) != 0 {
		t.Fatalf("List(cross actor) = %#v, %v", listed, err)
	}
	if _, err := service.Complete(t.Context(), CompleteCommand{Actor: actorB, ID: created.ID}); fault.CodeOf(err) != fault.CodeNotFound || !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Complete(cross actor) error = %v, code = %s", err, fault.CodeOf(err))
	}
}

type fixedIDs struct {
	id  string
	err error
}

type allowAuthorizer struct{}

func (allowAuthorizer) Enforce(context.Context, Actor, Action, ResourceFacts) error { return nil }

type ownerAuthorizer struct{}

func (ownerAuthorizer) Enforce(_ context.Context, actor Actor, _ Action, resource ResourceFacts) error {
	if resource.OwnerSubject != "" && resource.OwnerSubject != actor.Subject {
		return ErrPermissionDenied
	}
	return nil
}

func (f fixedIDs) New() (string, error) { return f.id, f.err }

type memoryRepository struct{ values map[string]model.Todo }

type failingRepository struct{ err error }

func (r failingRepository) Create(context.Context, model.Todo) (model.Todo, error) {
	return model.Todo{}, r.err
}
func (r failingRepository) Get(context.Context, string) (model.Todo, error) {
	return model.Todo{}, r.err
}
func (r failingRepository) List(context.Context, ListFilter) ([]model.Todo, int64, error) {
	return nil, 0, r.err
}
func (r failingRepository) Save(context.Context, model.Todo) (model.Todo, error) {
	return model.Todo{}, r.err
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{values: make(map[string]model.Todo)}
}

func (r *memoryRepository) Create(_ context.Context, todo model.Todo) (model.Todo, error) {
	todo.Version = 1
	r.values[todo.ID] = todo
	return todo, nil
}

func (r *memoryRepository) Get(_ context.Context, id string) (model.Todo, error) {
	todo, exists := r.values[id]
	if !exists {
		return model.Todo{}, fault.New(fault.CodeNotFound, "missing")
	}
	return todo, nil
}

func (r *memoryRepository) List(_ context.Context, filter ListFilter) ([]model.Todo, int64, error) {
	items := make([]model.Todo, 0, len(r.values))
	for _, todo := range r.values {
		if todo.OwnerSubject == filter.OwnerSubject && (filter.Status == nil || todo.Status == *filter.Status) {
			items = append(items, todo)
		}
	}
	return items, int64(len(items)), nil
}

func (r *memoryRepository) Save(_ context.Context, todo model.Todo) (model.Todo, error) {
	current, exists := r.values[todo.ID]
	if !exists {
		return model.Todo{}, fault.New(fault.CodeNotFound, "missing")
	}
	if current.Version != todo.Version {
		return model.Todo{}, fault.New(fault.CodeConflict, "version")
	}
	todo.Version++
	r.values[todo.ID] = todo
	return todo, nil
}
