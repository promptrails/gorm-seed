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
