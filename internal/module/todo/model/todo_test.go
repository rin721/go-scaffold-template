package model

import (
	"errors"
	"testing"
	"time"
)

func TestTodoNormalizesAndCompletesIdempotently(t *testing.T) {
	title, err := NormalizeTitle("  学习 Go  ", 20)
	if err != nil || title != "学习 Go" {
		t.Fatalf("NormalizeTitle() = %q, %v", title, err)
	}
	createdAt := time.Date(2026, 8, 15, 1, 2, 3, 0, time.FixedZone("test", 8*60*60))
	todo, err := New("id", title, "actor-a", createdAt)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	completedAt := createdAt.Add(time.Hour)
	changed, err := todo.Complete(completedAt)
	if err != nil || !changed || todo.Status != StatusCompleted || todo.CompletedAt == nil {
		t.Fatalf("Complete() = %v, %v, todo=%#v", changed, err, todo)
	}
	firstCompleted := *todo.CompletedAt
	changed, err = todo.Complete(completedAt.Add(time.Hour))
	if err != nil || changed || !todo.CompletedAt.Equal(firstCompleted) {
		t.Fatalf("Complete(repeat) = %v, %v, completed=%v", changed, err, todo.CompletedAt)
	}
}

func TestTodoRejectsInvalidValues(t *testing.T) {
	if _, err := NormalizeTitle("   ", 10); !errors.Is(err, ErrInvalidTitle) {
		t.Fatalf("empty title error = %v", err)
	}
	if _, err := NormalizeTitle("abcd", 3); !errors.Is(err, ErrInvalidTitle) {
		t.Fatalf("long title error = %v", err)
	}
	if _, err := ParseStatus("unknown"); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("status error = %v", err)
	}
	if _, err := Restore(Todo{ID: "id", Title: "title", OwnerSubject: "actor-a", Status: StatusCompleted, Version: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}); !errors.Is(err, ErrInvalidTime) {
		t.Fatalf("restore error = %v", err)
	}
}
