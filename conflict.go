package gormseed

import (
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// Conflict describes what to do when a seeded row collides with an existing one
// on its conflict target (the model's primary key by default).
type Conflict struct {
	kind    conflictKind
	columns []string // columns to overwrite (Update only)
	target  []string // conflict-target columns; empty = primary key
}

type conflictKind int

const (
	conflictSkip conflictKind = iota
	conflictUpdate
	conflictUpdateAll
	conflictError
)

// Skip keeps the existing row untouched on conflict (INSERT ... DO NOTHING).
// This is the default and makes seeding idempotent.
func Skip() Conflict { return Conflict{kind: conflictSkip} }

// Update overwrites the named columns of the existing row on conflict.
func Update(columns ...string) Conflict {
	return Conflict{kind: conflictUpdate, columns: columns}
}

// UpdateAll overwrites every column except the conflict target on conflict.
func UpdateAll() Conflict { return Conflict{kind: conflictUpdateAll} }

// Error performs a plain insert with no conflict handling, so a duplicate row
// surfaces as an error. Use it when fixtures are expected to apply to a fresh
// database.
func Error() Conflict { return Conflict{kind: conflictError} }

// withTarget returns a copy of c with an explicit conflict target.
func (c Conflict) withTarget(columns []string) Conflict {
	c.target = columns
	return c
}

// clause builds the gorm ON CONFLICT expression for c against the given schema.
// The second return value is false when no clause should be attached (Error).
func (c Conflict) clause(sch *schema.Schema) (clause.Expression, bool) {
	switch c.kind {
	case conflictError:
		return nil, false
	case conflictSkip:
		return clause.OnConflict{Columns: c.targetColumns(sch), DoNothing: true}, true
	case conflictUpdateAll:
		return clause.OnConflict{Columns: c.targetColumns(sch), UpdateAll: true}, true
	case conflictUpdate:
		return clause.OnConflict{
			Columns:   c.targetColumns(sch),
			DoUpdates: clause.AssignmentColumns(c.columns),
		}, true
	default:
		return clause.OnConflict{DoNothing: true}, true
	}
}

// targetColumns resolves the conflict-target columns: the explicit target if
// set, otherwise the model's primary key columns.
func (c Conflict) targetColumns(sch *schema.Schema) []clause.Column {
	names := c.target
	if len(names) == 0 && sch != nil {
		names = sch.PrimaryFieldDBNames
	}
	cols := make([]clause.Column, len(names))
	for i, n := range names {
		cols[i] = clause.Column{Name: n}
	}
	return cols
}
