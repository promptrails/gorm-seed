package gormseed

// spec is a single registered fixture: a file name, the destination slice it
// decodes into, and per-spec behavior.
type spec struct {
	name     string
	dest     any // pointer to a slice of models, e.g. &[]User{}
	conflict Conflict
	profiles []string
	after    []string // names of specs that must load before this one
	index    int      // registration order, used as a stable tie-breaker
}

// Result reports what a Run did, one entry per registered spec in load order.
type Result struct {
	Specs []SpecResult
}

// Inserted is the total number of rows inserted across all specs.
func (r *Result) Inserted() int64 {
	var n int64
	for _, s := range r.Specs {
		n += s.Inserted
	}
	return n
}

// SpecResult is the outcome of loading one spec.
type SpecResult struct {
	// Name is the fixture file name.
	Name string
	// Inserted is the number of rows the database reported as affected.
	Inserted int64
	// Planned is the number of rows decoded from the file. With a dry run it is
	// what would have been written; otherwise it equals the input row count.
	Planned int
	// Skipped is true when the spec was not loaded (filtered by profile, file
	// missing with WithSkipMissing, or empty file).
	Skipped bool
	// Reason explains a skip ("profile", "missing", "empty"), when Skipped.
	Reason string
	// Err is set when the spec failed and WithContinueOnError was enabled.
	Err error
}
