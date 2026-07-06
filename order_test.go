package gormseed

import (
	"strings"
	"testing"
)

func names(specs []*spec) []string {
	out := make([]string, len(specs))
	for i, sp := range specs {
		out[i] = sp.name
	}
	return out
}

func TestAutoOrderLoadsParentsFirst(t *testing.T) {
	db := newDB(t)
	// Register child before parent; auto-order must flip them.
	s := New(db, WithAutoOrder()).
		Add("posts.json", &[]Post{}).
		Add("users.json", &[]User{})

	ordered, err := s.ordered()
	if err != nil {
		t.Fatalf("ordered: %v", err)
	}
	got := names(ordered)
	if got[0] != "users.json" || got[1] != "posts.json" {
		t.Errorf("order = %v, want [users.json posts.json]", got)
	}
}

func TestAutoOrderHasMany(t *testing.T) {
	db := newDB(t)
	s := New(db, WithAutoOrder()).
		Add("comments.json", &[]Comment{}).
		Add("posts.json", &[]Post{}).
		Add("users.json", &[]User{})

	ordered, err := s.ordered()
	if err != nil {
		t.Fatalf("ordered: %v", err)
	}
	got := names(ordered)

	// Users must come before posts (Post has UserID FK), and posts before
	// comments (Comment has PostID FK).
	ui, pi, ci := -1, -1, -1
	for i, n := range got {
		switch n {
		case "users.json":
			ui = i
		case "posts.json":
			pi = i
		case "comments.json":
			ci = i
		}
	}
	if ui < 0 || pi < 0 || ci < 0 {
		t.Fatalf("not all specs found in order: %v", got)
	}
	if ui > pi {
		t.Errorf("users (%d) must load before posts (%d)", ui, pi)
	}
	if pi > ci {
		t.Errorf("posts (%d) must load before comments (%d)", pi, ci)
	}
}

func TestRegistrationOrderByDefault(t *testing.T) {
	db := newDB(t)
	s := New(db).
		Add("posts.json", &[]Post{}).
		Add("users.json", &[]User{})
	ordered, _ := s.ordered()
	got := names(ordered)
	if got[0] != "posts.json" || got[1] != "users.json" {
		t.Errorf("order = %v, want registration order [posts.json users.json]", got)
	}
}

func TestAfterDependency(t *testing.T) {
	db := newDB(t)
	s := New(db).
		Add("a.json", &[]User{}, After("b.json")).
		Add("b.json", &[]User{})
	ordered, err := s.ordered()
	if err != nil {
		t.Fatalf("ordered: %v", err)
	}
	got := names(ordered)
	if got[0] != "b.json" || got[1] != "a.json" {
		t.Errorf("order = %v, want [b.json a.json]", got)
	}
}

func TestAfterNonExistentDep(t *testing.T) {
	db := newDB(t)
	_, err := New(db).
		Add("a.json", &[]User{}, After("nonexistent.json")).
		ordered()
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("want 'not registered' error, got %v", err)
	}
}

func TestCycleIsError(t *testing.T) {
	db := newDB(t)
	s := New(db).
		Add("a.json", &[]User{}, After("b.json")).
		Add("b.json", &[]User{}, After("a.json"))
	_, err := s.ordered()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestComplexCycle(t *testing.T) {
	db := newDB(t)
	s := New(db).
		Add("a.json", &[]User{}, After("b.json")).
		Add("b.json", &[]User{}, After("c.json")).
		Add("c.json", &[]User{}, After("a.json"))
	_, err := s.ordered()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestNoAutoOrderNoDeps(t *testing.T) {
	db := newDB(t)
	s := New(db).
		Add("a.json", &[]User{}).
		Add("b.json", &[]User{}).
		Add("c.json", &[]User{})

	if s.hasAfter() {
		t.Error("hasAfter should be false when no After dependencies")
	}
}

func TestAutoOrderWithBelongsToAndHasMany(t *testing.T) {
	db := newDB(t)
	s := New(db, WithAutoOrder()).
		Add("a.json", &[]Comment{}).
		Add("b.json", &[]Post{}).
		Add("c.json", &[]User{})

	ordered, err := s.ordered()
	if err != nil {
		t.Fatalf("ordered: %v", err)
	}
	got := names(ordered)

	// User must be before Post, Post before Comment
	ui, pi, ci := -1, -1, -1
	for i, n := range got {
		switch n {
		case "c.json":
			ui = i
		case "b.json":
			pi = i
		case "a.json":
			ci = i
		}
	}
	if ui > pi || pi > ci {
		t.Errorf("expected c.json < b.json < a.json, got %v", got)
	}
}
