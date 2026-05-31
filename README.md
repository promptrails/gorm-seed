# gorm-seed

Rails-style `db:seed` for [GORM](https://gorm.io): load JSON fixtures into your
database with **idempotent, conflict-aware** inserts.

[![Go Reference](https://pkg.go.dev/badge/github.com/promptrails/gorm-seed.svg)](https://pkg.go.dev/github.com/promptrails/gorm-seed)
[![CI](https://github.com/promptrails/gorm-seed/actions/workflows/ci.yml/badge.svg)](https://github.com/promptrails/gorm-seed/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/promptrails/gorm-seed)](https://goreportcard.com/report/github.com/promptrails/gorm-seed)

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
- **Foreign-key-safe ordering** — `WithAutoOrder()` inspects belongs-to
  relationships and loads parents before children; `After(...)` adds explicit
  edges. Cycles are reported, not silently mis-ordered.
- **Any `fs.FS`** — a directory (`Dir("...")`) or an `embed.FS` work the same way.
- **Profiles** — tag specs (`Profile("demo")`) and activate them per run
  (`WithProfiles("demo")`); untagged specs always load.
- **Dry run, transactions, reporting** — `WithDryRun()`, `WithTransaction()`, and
  a per-spec `Result`.
- **Pluggable decoders** — JSON built in; add YAML or others with `WithDecoder`,
  no extra dependency in the core.

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

## Ordering

Without options, specs load in registration order — you control the sequence.
Turn on `WithAutoOrder()` to let the seeder derive a foreign-key-safe order from
your models' belongs-to relationships, or pin specific edges with `After`:

```go
gormseed.New(db, gormseed.WithAutoOrder()).
    Add("posts.json", &[]Post{}).   // registered first…
    Add("users.json", &[]User{})    // …but loaded first (Post belongs to User)
```

## Profiles

```go
seeder := gormseed.New(db, gormseed.WithProfiles("demo")).
    Add("users.json", &[]User{}).                       // always loads
    Add("demo_data.json", &[]Order{}, gormseed.Profile("demo")) // only with "demo"
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

## License

MIT — see [LICENSE](LICENSE).
