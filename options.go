package gormseed

import "context"

// Option configures a Seeder.
type Option func(*Seeder)

// WithAutoOrder enables automatic, foreign-key-safe load ordering. The seeder
// inspects each model's belongs-to and has-many relationships and loads parents
// before children. Explicit After dependencies are always honored on top of
// this. A dependency cycle makes Run fail; break it with explicit ordering.
func WithAutoOrder() Option {
	return func(s *Seeder) { s.autoOrder = true }
}

// WithProfiles activates the named profiles. Specs tagged with Profile load
// only when at least one of their profiles is active; untagged specs always
// load. With no active profiles, tagged specs are skipped.
func WithProfiles(profiles ...string) Option {
	return func(s *Seeder) {
		for _, p := range profiles {
			s.profiles[p] = true
		}
	}
}

// WithDryRun decodes and plans every spec but writes nothing. SpecResult.Planned
// reports how many rows would have been inserted.
func WithDryRun() Option {
	return func(s *Seeder) { s.dryRun = true }
}

// WithSkipMissing treats a registered spec whose file is absent as a skip
// rather than an error. Useful for optional, directory-based seed sets.
func WithSkipMissing() Option {
	return func(s *Seeder) { s.skipMissing = true }
}

// WithContinueOnError records a failing spec's error in its SpecResult and
// proceeds with the remaining specs instead of aborting the run.
func WithContinueOnError() Option {
	return func(s *Seeder) { s.continueOnError = true }
}

// WithTransaction wraps the entire run in a single database transaction, so a
// failure rolls back every spec. Note some databases (e.g. MySQL) do not
// support transactional DDL; this only affects the data writes seeding does.
func WithTransaction() Option {
	return func(s *Seeder) { s.transaction = true }
}

// WithDefaultConflict sets the conflict strategy used by specs that do not
// specify their own. The built-in default is Skip (idempotent seeding).
func WithDefaultConflict(c Conflict) Option {
	return func(s *Seeder) { s.defaultConflict = c }
}

// WithDecoder registers a decoder for files with the given extension (including
// the leading dot, e.g. ".yaml"). The function unmarshals the file bytes into a
// pointer to a slice of models. JSON (".json") is registered by default; use
// this to add formats such as YAML without the core taking on the dependency.
func WithDecoder(ext string, fn func(data []byte, dest any) error) Option {
	return func(s *Seeder) { s.decoders[ext] = decoderFunc(fn) }
}

// WithDecoderContext registers a context-aware decoder for files with the given
// extension. This is like WithDecoder but the function also receives the
// context from Run, allowing cancellation and tracing.
func WithDecoderContext(ext string, fn func(ctx context.Context, data []byte, dest any) error) Option {
	return func(s *Seeder) { s.decoders[ext] = fn }
}

// WithBatchSize sets the number of rows inserted in each batch when creating
// records. Use this when seeding large fixture files to avoid excessive memory
// usage or statement-size limits. A value of 0 (default) inserts all rows in a
// single Create call.
func WithBatchSize(n int) Option {
	return func(s *Seeder) { s.batchSize = n }
}

// WithLogger sets a logger on the seeder. The logger receives informational
// messages about seeding progress (spec start, rows inserted, etc.).
func WithLogger(l Logger) Option {
	return func(s *Seeder) { s.logger = l }
}

// WithBeforeSeedHook registers a hook that is called before each spec is
// seeded (after decoding, before insert). Multiple hooks are called in
// registration order.
func WithBeforeSeedHook(hook SeedHook) Option {
	return func(s *Seeder) { s.beforeHooks = append(s.beforeHooks, hook) }
}

// WithAfterSeedHook registers a hook that is called after each spec is seeded
// (after insert). Multiple hooks are called in registration order.
func WithAfterSeedHook(hook SeedHook) Option {
	return func(s *Seeder) { s.afterHooks = append(s.afterHooks, hook) }
}

// SpecOption configures a single registered fixture.
type SpecOption func(*spec)

// OnConflict sets the conflict strategy for this spec, overriding the seeder
// default.
func OnConflict(c Conflict) SpecOption {
	return func(sp *spec) { sp.conflict = c }
}

// ConflictTarget overrides the conflict-target columns for this spec. By
// default the model's primary key is used.
func ConflictTarget(columns ...string) SpecOption {
	return func(sp *spec) { sp.conflict = sp.conflict.withTarget(columns) }
}

// Profile tags this spec with one or more profiles. It then loads only when one
// of those profiles is active (see WithProfiles).
func Profile(profiles ...string) SpecOption {
	return func(sp *spec) { sp.profiles = append(sp.profiles, profiles...) }
}

// After declares that this spec must load after the named specs. It is honored
// regardless of WithAutoOrder.
func After(names ...string) SpecOption {
	return func(sp *spec) { sp.after = append(sp.after, names...) }
}
