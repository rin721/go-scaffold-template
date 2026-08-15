package composition

import (
	"context"
	"errors"
	"fmt"

	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	authservice "github.com/rin721/go-scaffold-template/internal/module/auth/service"
	httpbinding "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	todoservice "github.com/rin721/go-scaffold-template/internal/module/todo/service"
)

// todoAuthorizerAdapter 只负责在两个项目自有模块契约之间映射。
type todoAuthorizerAdapter struct{ auth *authservice.Service }

func newTodoAuthorizerAdapter(auth *authservice.Service) (todoservice.Authorizer, error) {
	if auth == nil {
		return nil, fmt.Errorf("auth service for Todo is nil")
	}
	return todoAuthorizerAdapter{auth: auth}, nil
}

func (a todoAuthorizerAdapter) Enforce(
	ctx context.Context,
	actor todoservice.Actor,
	action todoservice.Action,
	resource todoservice.ResourceFacts,
) error {
	principal, ok := authmodel.PrincipalFromContext(ctx)
	if !ok || principal.Subject != actor.Subject {
		return todoservice.ErrPermissionDenied
	}
	if err := a.auth.EnforceAction(ctx, principal, authmodel.Action(action), authmodel.ResourceFacts{
		Type: "todo", ID: resource.ID, OwnerSubject: resource.OwnerSubject,
	}); err != nil {
		if errors.Is(err, authmodel.ErrPermissionDenied) || errors.Is(err, authmodel.ErrUnauthenticated) {
			return todoservice.ErrPermissionDenied
		}
		return err
	}
	return nil
}

func todoActor(principal authmodel.Principal) todoservice.Actor {
	scopes := make([]string, len(principal.Scopes))
	for index, scope := range principal.Scopes {
		scopes[index] = string(scope)
	}
	return todoservice.Actor{Subject: principal.Subject, Kind: string(principal.Kind), Scopes: scopes}
}

// todoActorAccessAdapter 只把 Auth request context 映射为 Todo Actor。
type todoActorAccessAdapter struct{}

func (todoActorAccessAdapter) Actor(ctx context.Context) (todoservice.Actor, bool) {
	principal, ok := authmodel.PrincipalFromContext(ctx)
	if !ok {
		return todoservice.Actor{}, false
	}
	return todoActor(principal), true
}

var _ todoservice.Authorizer = todoAuthorizerAdapter{}
var _ httpbinding.ActorAccess = todoActorAccessAdapter{}
