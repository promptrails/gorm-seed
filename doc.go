// Package gormseed loads JSON (or custom-format) fixtures into a GORM database
// with idempotent, conflict-aware inserts — Rails-style db:seed for GORM.
//
// Unlike test-fixture loaders that truncate and reload, gormseed is built for
// repeatable seeding of reference, demo, and initial data: by default it uses
// INSERT ... ON CONFLICT DO NOTHING, so running it twice is a no-op rather than
// a duplicate-key error.
//
// Register fixtures, then run them against any fs.FS (a directory or an
// embed.FS):
//
//	seeder := gormseed.New(db, gormseed.WithAutoOrder()).
//		Add("users.json", &[]User{}).
//		Add("posts.json", &[]Post{}, gormseed.OnConflict(gormseed.UpdateAll()))
//
//	res, err := seeder.Run(ctx, gormseed.Dir("fixtures"))
//	// or: res, err := seeder.Run(ctx, embeddedFS)
//
// Features:
//   - Idempotent inserts with selectable conflict strategy (Skip, Update,
//     UpdateAll, Error), per spec or as a seeder-wide default.
//   - Foreign-key-safe ordering: WithAutoOrder loads parents before children by
//     inspecting belongs-to and has-many relationships; After adds explicit
//     edges.
//   - Profiles (dev/demo/test), dry runs, transactional runs, and a per-spec
//     Result report.
//   - Pluggable decoders (JSON built in; add YAML or others without the core
//     taking on the dependency).
//   - Batch inserts for large fixture files via WithBatchSize.
//   - Table cleanup via Clean, for resetting data between runs.
//   - Before/after seed hooks for monitoring and instrumentation.
//   - Optional logger for progress output.
package gormseed
