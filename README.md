# gorm-seed

Rails-style `db:seed` for [GORM](https://gorm.io): load JSON fixtures into your
database with **idempotent, conflict-aware** inserts.

[![Go Reference](https://pkg.go.dev/badge/github.com/promptrails/gorm-seed.svg)](https://pkg.go.dev/github.com/promptrails/gorm-seed)
[![CI](https://github.com/promptrails/gorm-seed/actions/workflows/ci.yml/badge.svg)](https://github.com/promptrails/gorm-seed/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/promptrails/gorm-seed)](https://goreportcard.com/report/github.com/promptrails/gorm-seed)
[![codecov](https://codecov.io/gh/promptrails/gorm-seed/branch/main/graph/badge.svg)](https://codecov.io/gh/promptrails/gorm-seed)

Test-fixture loaders truncate and reload — wrong for reference, demo, and
initial data. `gorm-seed` is built for **repeatable seeding**: by default it
emits `INSERT ... ON CONFLICT DO NOTHING`, so running it twice is a no-op
instead of a duplicate-key error.

```go
import gormseed "github.com/promptrails/gorm-seed"

seeder := gormseed.New(db, gormseed.WithAutoOrder()).
    Add("users.json", &[]User{}).
    Add("posts.json", &[]Post{}, gormseed.OnConflict(gormseed.UpdateAll()))

res, err := seeder.Run(ctx, gormseed.Dir("fixtures"))
fmt.Printf("seeded %d rows\n", res.Inserted())
```

`fixtures/users.json`:

```json
[
  { "ID": 1, "Name": "Alice", "Email": "alice@example.com" },
  { "ID": 2, "Name": "Bob",   "Email": "bob@example.com" }
]
```

## Install

```bash
go get github.com/promptrails/gorm-seed
```

Requires Go 1.26+. The only runtime dependency is GORM itself.

## Features

- **Idempotent by default** — `ON CONFLICT DO NOTHING`; safe to run on every boot.
- **Conflict strategies** — `Skip` (default), `Update(cols…)`, `UpdateAll()`, or
  `Error()`, per spec or as a seeder-wide default; override the conflict target
  with `ConflictTarget`.
- **Foreign-key-safe ordering** — `WithAutoOrder()` inspects belongs-to and
  has-many relationships and loads parents before children; `After(...)` adds
  explicit edges. Cycles are reported, not silently mis-ordered.
- **Any `fs.FS`** — a directory (`Dir("...")`) or an `embed.FS` work the same way.
- **Profiles** — tag specs (`Profile("demo")`) and activate them per run
  (`WithProfiles("demo")`); untagged specs always load.
- **Dry run, transactions, reporting** — `WithDryRun()`, `WithTransaction()`, and
  a per-spec `Result`.
- **Pluggable decoders** — JSON built in; add YAML or others with `WithDecoder`,
  no extra dependency in the core.
- **Batch inserts** — `WithBatchSize(n)` for large fixture files.
- **Table cleanup** — `Clean(ctx)` to drop and recreate tables.
- **Hooks** — `WithBeforeSeedHook` / `WithAfterSeedHook` for instrumentation.
- **Logging** — `WithLogger(l)` for progress output.

## Conflict strategies

```go
gormseed.Skip()            // keep existing row (default, idempotent)
gormseed.Update("name")    // overwrite named columns on conflict
gormseed.UpdateAll()       // overwrite every non-key column
gormseed.Error()           // plain insert; duplicates surface as an error
```

```go
seeder.Add("plans.json", &[]Plan{}, gormseed.OnConflict(gormseed.UpdateAll()))
```

Override the conflict target:

```go
seeder.Add("plans.json", &[]Plan{},
    gormseed.OnConflict(gormseed.UpdateAll()),
    gormseed.ConflictTarget("code"),
)
```

Set a seeder-wide default:

```go
gormseed.New(db, gormseed.WithDefaultConflict(gormseed.UpdateAll()))
```

## Ordering

Without options, specs load in registration order — you control the sequence.
Turn on `WithAutoOrder()` to let the seeder derive a foreign-key-safe order from
your models' belongs-to and has-many relationships, or pin specific edges with
`After`:

```go
gormseed.New(db, gormseed.WithAutoOrder()).
    Add("posts.json", &[]Post{}).   // registered first…
    Add("users.json", &[]User{})    // …but loaded first (Post belongs to User)
```

Has-many relationships are also respected:

```go
gormseed.New(db, gormseed.WithAutoOrder()).
    Add("comments.json", &[]Comment{}).
    Add("posts.json", &[]Post{}).
    Add("users.json", &[]User{})
// Load order: users → posts → comments
```

Explicit edges with `After`:

```go
gormseed.New(db).
    Add("a.json", &[]User{}, gormseed.After("b.json")).
    Add("b.json", &[]User{})
// Load order: b → a
```

## Profiles

```go
seeder := gormseed.New(db, gormseed.WithProfiles("demo")).
    Add("users.json", &[]User{}).                                // always loads
    Add("demo_data.json", &[]Order{}, gormseed.Profile("demo"))  // only with "demo"
```

## Transactions

```go
seeder := gormseed.New(db, gormseed.WithTransaction(), gormseed.WithAutoOrder()).
    Add("users.json", &[]User{}).
    Add("posts.json", &[]Post{})

res, err := seeder.Run(ctx, gormseed.Dir("fixtures"))
// On error, all inserts are rolled back.
```

## Batch inserts

```go
seeder := gormseed.New(db, gormseed.WithBatchSize(100)).
    Add("large_fixture.json", &[]User{})
// Inserts happen in batches of 100 rows.
```

## Hooks

```go
seeder := gormseed.New(db,
    gormseed.WithBeforeSeedHook(func(ctx context.Context, name string, planned int) {
        fmt.Printf("seeding %s (%d rows)...\n", name, planned)
    }),
    gormseed.WithAfterSeedHook(func(ctx context.Context, name string, planned int) {
        fmt.Printf("done seeding %s\n", name)
    }),
)
```

## Logging

```go
seeder := gormseed.New(db, gormseed.WithLogger(log.Printf))
```

## Clean (truncate tables)

```go
seeder := gormseed.New(db).Add("users.json", &[]User{})
seeder.Run(ctx, fsys)
seeder.Clean(ctx) // drops and recreates the users table
```

## Embedding fixtures

```go
//go:embed fixtures/*.json
var fixtures embed.FS

sub, _ := fs.Sub(fixtures, "fixtures")
seeder.Run(ctx, sub)
```

## Custom formats (YAML, …)

```go
import "gopkg.in/yaml.v3"

seeder := gormseed.New(db, gormseed.WithDecoder(".yaml", yaml.Unmarshal))
seeder.Add("users.yaml", &[]User{})
```

For context-aware decoders:

```go
seeder := gormseed.New(db, gormseed.WithDecoderContext(".yaml",
    func(ctx context.Context, data []byte, dest any) error {
        return yaml.Unmarshal(data, dest)
    },
))
```

## API Reference

### Types

- `Seeder` — main struct, created with `New`
- `Conflict` — conflict strategy
- `Result` — run result with `Specs []SpecResult`
- `SpecResult` — per-spec outcome
- `Logger` — interface for logging
- `SeedHook` — callback type for hooks

### Functions

- `New(db, opts...)` — create a seeder
- `Dir(path)` — create an `fs.FS` from a directory path
- `Skip()`, `Update(cols...)`, `UpdateAll()`, `Error()` — conflict strategies

### Options

| Option | Type | Description |
|--------|------|-------------|
| `WithAutoOrder()` | `Option` | Enable FK-safe ordering |
| `WithProfiles(names...)` | `Option` | Activate profiles |
| `WithDryRun()` | `Option` | Plan only, no writes |
| `WithSkipMissing()` | `Option` | Skip missing files |
| `WithContinueOnError()` | `Option` | Continue on spec errors |
| `WithTransaction()` | `Option` | Wrap in a DB transaction |
| `WithDefaultConflict(c)` | `Option` | Set default conflict strategy |
| `WithDecoder(ext, fn)` | `Option` | Register a decoder |
| `WithDecoderContext(ext, fn)` | `Option` | Register a context-aware decoder |
| `WithBatchSize(n)` | `Option` | Set batch insert size |
| `WithLogger(l)` | `Option` | Set logger |
| `WithBeforeSeedHook(h)` | `Option` | Register before-seed hook |
| `WithAfterSeedHook(h)` | `Option` | Register after-seed hook |

### Spec Options

- `OnConflict(c)` — per-spec conflict strategy
- `ConflictTarget(cols...)` — override conflict target columns
- `Profile(names...)` — tag with profiles
- `After(names...)` — declare dependencies

### Methods

- `(*Seeder) Add(name, dest, opts...)` — register a fixture
- `(*Seeder) Run(ctx, fsys)` — execute seeding
- `(*Seeder) Clean(ctx)` — drop and recreate tables
- `(*Result) Inserted()` — total inserted rows

## License

MIT — see [LICENSE](LICENSE).
