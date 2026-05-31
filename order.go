package gormseed

import (
	"fmt"
	"strings"

	"gorm.io/gorm/schema"
)

// ordered returns the specs in the order they should load. Without WithAutoOrder
// and without any After dependencies, that is registration order. Otherwise a
// stable topological sort is applied: dependencies first, ties broken by
// registration order. A cycle is an error.
func (s *Seeder) ordered() ([]*spec, error) {
	if !s.autoOrder && !s.hasAfter() {
		return s.specs, nil
	}

	indeg := make(map[*spec]int, len(s.specs))
	edges := make(map[*spec][]*spec, len(s.specs))
	seen := make(map[[2]*spec]bool) // dedupe so a doubled edge can't inflate indegree
	for _, sp := range s.specs {
		indeg[sp] = 0
	}
	addEdge := func(from, to *spec) {
		if from == to || from == nil || to == nil {
			return
		}
		key := [2]*spec{from, to}
		if seen[key] {
			return
		}
		seen[key] = true
		edges[from] = append(edges[from], to)
		indeg[to]++
	}

	// Explicit After dependencies (always honored).
	for _, sp := range s.specs {
		for _, dep := range sp.after {
			addEdge(s.byName[dep], sp)
		}
	}

	// Foreign-key edges from belongs-to relationships: parent loads before child.
	if s.autoOrder {
		if err := s.addForeignKeyEdges(addEdge); err != nil {
			return nil, err
		}
	}

	return s.topoSort(indeg, edges)
}

// addForeignKeyEdges inspects each registered model's belongs-to relationships
// and adds an edge from the referenced (parent) spec to the owning (child) spec.
func (s *Seeder) addForeignKeyEdges(addEdge func(from, to *spec)) error {
	schemas := make(map[*spec]*schema.Schema, len(s.specs))
	byTable := make(map[string]*spec, len(s.specs))
	for _, sp := range s.specs {
		sch, err := s.schemaOf(sp.dest)
		if err != nil {
			return err
		}
		schemas[sp] = sch
		byTable[sch.Table] = sp
	}
	for _, sp := range s.specs {
		for _, rel := range schemas[sp].Relationships.Relations {
			if rel.Type != schema.BelongsTo || rel.FieldSchema == nil {
				continue
			}
			addEdge(byTable[rel.FieldSchema.Table], sp)
		}
	}
	return nil
}

// topoSort returns a stable topological order (ties broken by registration
// order) or an error naming the specs caught in a cycle.
func (s *Seeder) topoSort(indeg map[*spec]int, edges map[*spec][]*spec) ([]*spec, error) {
	emitted := make(map[*spec]bool, len(s.specs))
	order := make([]*spec, 0, len(s.specs))
	for len(order) < len(s.specs) {
		var pick *spec
		for _, sp := range s.specs { // s.specs is in registration order → stable
			if !emitted[sp] && indeg[sp] == 0 {
				pick = sp
				break
			}
		}
		if pick == nil {
			return nil, fmt.Errorf("gormseed: dependency cycle among %s", strings.Join(remaining(s.specs, emitted), ", "))
		}
		emitted[pick] = true
		order = append(order, pick)
		for _, to := range edges[pick] {
			indeg[to]--
		}
	}
	return order, nil
}

func (s *Seeder) hasAfter() bool {
	for _, sp := range s.specs {
		if len(sp.after) > 0 {
			return true
		}
	}
	return false
}

func remaining(specs []*spec, emitted map[*spec]bool) []string {
	var names []string
	for _, sp := range specs {
		if !emitted[sp] {
			names = append(names, sp.name)
		}
	}
	return names
}
