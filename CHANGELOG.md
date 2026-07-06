# Changelog

## v0.1.0 (2026-07-06)

### Added
- Initial release with idempotent seeding support
- Conflict strategies: Skip, Update, UpdateAll, Error
- Foreign-key-safe auto-ordering via WithAutoOrder (BelongsTo)
- Explicit After dependency ordering
- Profiles for environment-specific seeding
- Dry runs, transactional runs, continue-on-error
- Pluggable decoders (JSON built-in)
- Any fs.FS source (directory or embed.FS)

### Features added after initial release
- HasMany/HasOne relationship auto-ordering support
- WithBatchSize for large fixture file chunked inserts
- Clean method for table truncation/reset
- WithBeforeSeedHook and WithAfterSeedHook callbacks
- WithLogger for progress output
- WithDecoderContext for context-aware decoders
- After dependency validation (error on non-existent spec names)
- Concurrent-safe Add via mutex protection
