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

// todoRequestAccessAdapter 在 Auth request context 与 Todo HTTP port 之间映射。
type todoRequestAccessAdapter struct{ auth *authservice.Service }

func newTodoRequestAccessAdapter(auth *authservice.Service) (httpbinding.RequestAccess, error) {
	if auth == nil {
		return nil, fmt.Errorf("auth service for Todo HTTP is nil")
	}
	return todoRequestAccessAdapter{auth: auth}, nil
}

func (a todoRequestAccessAdapter) Actor(ctx context.Context) (todoservice.Actor, bool) {
	principal, ok := authmodel.PrincipalFromContext(ctx)
	if !ok {
		return todoservice.Actor{}, false
	}
	return todoActor(principal), true
}

func (a todoRequestAccessAdapter) EnforceOperation(ctx context.Context, actor todoservice.Actor, operation string) error {
	principal, ok := authmodel.PrincipalFromContext(ctx)
	if !ok || principal.Subject != actor.Subject {
		return todoservice.ErrPermissionDenied
	}
	if err := a.auth.EnforceOperation(ctx, principal, operation); err != nil {
		if errors.Is(err, authmodel.ErrPermissionDenied) || errors.Is(err, authmodel.ErrUnauthenticated) {
			return todoservice.ErrPermissionDenied
		}
		return err
	}
	return nil
}

var _ todoservice.Authorizer = todoAuthorizerAdapter{}
var _ httpbinding.RequestAccess = todoRequestAccessAdapter{}
