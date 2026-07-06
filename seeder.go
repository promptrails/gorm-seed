package gormseed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Seeder loads JSON (or custom-format) fixtures into a GORM database with
// idempotent, conflict-aware inserts. Build one with New, register fixtures
// with Add, then call Run. A Seeder is not safe for concurrent Add calls, but
// Run may be called repeatedly (seeding is idempotent under the default Skip
// strategy).
type Seeder struct {
	mu       sync.Mutex
	db       *gorm.DB
	specs    []*spec
	byName   map[string]*spec
	decoders map[string]func(context.Context, []byte, any) error
	cache    sync.Map

	autoOrder       bool
	dryRun          bool
	skipMissing     bool
	continueOnError bool
	transaction     bool
	profiles        map[string]bool
	defaultConflict Conflict
	batchSize       int
	logger          Logger
	beforeHooks     []SeedHook
	afterHooks      []SeedHook
}

// New creates a Seeder for the given database with the given options.
func New(db *gorm.DB, opts ...Option) *Seeder {
	s := &Seeder{
		db:              db,
		byName:          map[string]*spec{},
		decoders:        map[string]func(context.Context, []byte, any) error{".json": decoderFunc(json.Unmarshal)},
		profiles:        map[string]bool{},
		defaultConflict: Skip(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// decoderFunc wraps a plain func([]byte, any) error as a context-aware decoder.
func decoderFunc(fn func([]byte, any) error) func(context.Context, []byte, any) error {
	return func(_ context.Context, data []byte, dest any) error {
		return fn(data, dest)
	}
}

// Add registers a fixture: file name (read from the fs.FS passed to Run) and a
// pointer to a slice of models it decodes into, e.g. &[]User{}. Specs load in
// registration order unless WithAutoOrder or After reorders them. Add returns
// the Seeder for chaining.
func (s *Seeder) Add(name string, dest any, opts ...SpecOption) *Seeder {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := &spec{name: name, dest: dest, conflict: s.defaultConflict, index: len(s.specs)}
	for _, opt := range opts {
		opt(sp)
	}
	s.specs = append(s.specs, sp)
	s.byName[name] = sp
	return s
}

// Dir returns an fs.FS rooted at the given directory, for passing to Run.
func Dir(path string) fs.FS { return os.DirFS(path) }

// Run loads every registered fixture from fsys and returns a per-spec Result.
// Reads use fs.FS so a directory (Dir) and an embed.FS work the same way.
func (s *Seeder) Run(ctx context.Context, fsys fs.FS) (*Result, error) {
	ordered, err := s.ordered()
	if err != nil {
		return nil, err
	}

	res := &Result{}
	exec := func(tx *gorm.DB) error {
		for _, sp := range ordered {
			sr, err := s.load(ctx, tx, fsys, sp)
			res.Specs = append(res.Specs, sr)
			if err != nil {
				if s.continueOnError {
					continue
				}
				return err
			}
		}
		return nil
	}

	if s.transaction && !s.dryRun {
		if err := s.db.WithContext(ctx).Transaction(exec); err != nil {
			return res, err
		}
		return res, nil
	}
	if err := exec(s.db.WithContext(ctx)); err != nil {
		return res, err
	}
	return res, nil
}

// Clean truncates all tables associated with registered specs. Specs are
// processed in reverse load order so that child tables are emptied before
// parent tables, avoiding foreign-key violations.
func (s *Seeder) Clean(ctx context.Context) (*Result, error) {
	ordered, err := s.ordered()
	if err != nil {
		return nil, err
	}

	res := &Result{}
	exec := func(tx *gorm.DB) error {
		for i := len(ordered) - 1; i >= 0; i-- {
			sp := ordered[i]
			sr := SpecResult{Name: sp.name}

			sch, err := s.schemaOf(sp.dest)
			if err != nil {
				sr.Err = fmt.Errorf("gormseed: schema for %s: %w", sp.name, err)
				res.Specs = append(res.Specs, sr)
				if !s.continueOnError {
					return sr.Err
				}
				continue
			}

			if s.logger != nil {
				s.logger.Printf("gormseed: truncating table %s for %s", sch.Table, sp.name)
			}

			if err := tx.WithContext(ctx).Migrator().DropTable(sch.Table); err != nil {
				sr.Err = fmt.Errorf("gormseed: drop table %s for %s: %w", sch.Table, sp.name, err)
				res.Specs = append(res.Specs, sr)
				if !s.continueOnError {
					return sr.Err
				}
				continue
			}
			if err := tx.WithContext(ctx).AutoMigrate(sp.dest); err != nil {
				sr.Err = fmt.Errorf("gormseed: recreate table for %s: %w", sp.name, err)
				res.Specs = append(res.Specs, sr)
				if !s.continueOnError {
					return sr.Err
				}
				continue
			}

			res.Specs = append(res.Specs, sr)
		}
		return nil
	}

	if s.transaction {
		if err := s.db.WithContext(ctx).Transaction(exec); err != nil {
			return res, err
		}
		return res, nil
	}
	if err := exec(s.db.WithContext(ctx)); err != nil {
		return res, err
	}
	return res, nil
}

// load processes a single spec: profile filter, read, decode, insert.
func (s *Seeder) load(ctx context.Context, tx *gorm.DB, fsys fs.FS, sp *spec) (SpecResult, error) {
	sr := SpecResult{Name: sp.name}

	if !s.profileActive(sp) {
		sr.Skipped, sr.Reason = true, "profile"
		return sr, nil
	}

	decode, ok := s.decoders[filepath.Ext(sp.name)]
	if !ok {
		sr.Err = fmt.Errorf("gormseed: no decoder registered for %q", sp.name)
		return sr, sr.Err
	}

	data, err := fs.ReadFile(fsys, sp.name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && s.skipMissing {
			sr.Skipped, sr.Reason = true, "missing"
			return sr, nil
		}
		sr.Err = fmt.Errorf("gormseed: read %s: %w", sp.name, err)
		return sr, sr.Err
	}

	if err := decode(ctx, data, sp.dest); err != nil {
		sr.Err = fmt.Errorf("gormseed: decode %s: %w", sp.name, err)
		return sr, sr.Err
	}

	n := rowCount(sp.dest)
	sr.Planned = n
	if n == 0 {
		sr.Skipped, sr.Reason = true, "empty"
		return sr, nil
	}
	if s.dryRun {
		return sr, nil
	}

	for _, h := range s.beforeHooks {
		h(ctx, sp.name, n)
	}

	sch, err := s.schemaOf(sp.dest)
	if err != nil {
		sr.Err = fmt.Errorf("gormseed: schema for %s: %w", sp.name, err)
		return sr, sr.Err
	}

	db := tx.WithContext(ctx)
	if expr, ok := sp.conflict.clause(sch); ok {
		db = db.Clauses(expr)
	}

	if s.batchSize > 0 {
		if err := db.CreateInBatches(sp.dest, s.batchSize).Error; err != nil {
			sr.Err = fmt.Errorf("gormseed: insert %s: %w", sp.name, err)
			return sr, sr.Err
		}
	} else {
		if err := db.Create(sp.dest).Error; err != nil {
			sr.Err = fmt.Errorf("gormseed: insert %s: %w", sp.name, err)
			return sr, sr.Err
		}
	}

	sr.Inserted = db.RowsAffected

	for _, h := range s.afterHooks {
		h(ctx, sp.name, n)
	}

	return sr, nil
}

// profileActive reports whether sp should load given the active profiles.
func (s *Seeder) profileActive(sp *spec) bool {
	if len(sp.profiles) == 0 {
		return true
	}
	for _, p := range sp.profiles {
		if s.profiles[p] {
			return true
		}
	}
	return false
}

// schemaOf parses (and caches) the GORM schema for a spec's element model.
func (s *Seeder) schemaOf(dest any) (*schema.Schema, error) {
	model, err := elemModel(dest)
	if err != nil {
		return nil, err
	}
	sch, err := schema.Parse(model, &s.cache, s.db.NamingStrategy)
	if err != nil {
		return nil, fmt.Errorf("gormseed: parse schema for %T: %w", model, err)
	}
	return sch, nil
}

// elemModel returns a new pointer to the element type of a *[]Model dest, for
// schema parsing.
func elemModel(dest any) (any, error) {
	t := reflect.TypeOf(dest)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Slice {
		return nil, fmt.Errorf("gormseed: dest must be a pointer to a slice, got %T", dest)
	}
	elem := t.Elem().Elem()
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	return reflect.New(elem).Interface(), nil
}

// rowCount returns the length of a *[]Model dest, or 0 if it is not a slice
// pointer or is empty.
func rowCount(dest any) int {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Slice {
		return 0
	}
	return v.Elem().Len()
}
