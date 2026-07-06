package gormseed

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID    uint `gorm:"primaryKey"`
	Name  string
	Email string `gorm:"uniqueIndex"`
}

type Post struct {
	ID     uint `gorm:"primaryKey"`
	Title  string
	UserID uint
	User   User
}

type UserProfile struct {
	ID  uint `gorm:"primaryKey"`
	Bio string
}

type Comment struct {
	ID      uint `gorm:"primaryKey"`
	Content string
	PostID  uint
	Post    Post
}

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Post{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func files(m map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range m {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

func TestSeedAndIdempotency(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"},{"ID":2,"Name":"Bob","Email":"b@x.io"}]`,
	})
	s := New(db).Add("users.json", &[]User{})

	res, err := s.Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Inserted() != 2 {
		t.Fatalf("first run inserted %d, want 2", res.Inserted())
	}

	// Second run is a no-op under the default Skip strategy.
	res2, err := s.Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if res2.Inserted() != 0 {
		t.Errorf("second run inserted %d, want 0 (idempotent)", res2.Inserted())
	}
	var count int64
	db.Model(&User{}).Count(&count)
	if count != 2 {
		t.Errorf("user count = %d, want 2", count)
	}
}

func TestConflictUpdate(t *testing.T) {
	db := newDB(t)
	db.Create(&User{ID: 1, Name: "Old", Email: "a@x.io"})

	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"New","Email":"a@x.io"}]`,
	})
	_, err := New(db).
		Add("users.json", &[]User{}, OnConflict(Update("name"))).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var u User
	db.First(&u, 1)
	if u.Name != "New" {
		t.Errorf("name = %q, want New (updated on conflict)", u.Name)
	}
}

func TestConflictUpdateAll(t *testing.T) {
	db := newDB(t)
	db.Create(&User{ID: 1, Name: "Old", Email: "old@x.io"})

	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"New","Email":"new@x.io"}]`,
	})
	_, err := New(db).
		Add("users.json", &[]User{}, OnConflict(UpdateAll())).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var u User
	db.First(&u, 1)
	if u.Name != "New" {
		t.Errorf("name = %q, want New", u.Name)
	}
	if u.Email != "new@x.io" {
		t.Errorf("email = %q, want new@x.io", u.Email)
	}
}

func TestConflictError(t *testing.T) {
	db := newDB(t)
	db.Create(&User{ID: 1, Name: "Old", Email: "a@x.io"})

	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Dup","Email":"a@x.io"}]`,
	})
	_, err := New(db).
		Add("users.json", &[]User{}, OnConflict(Error())).
		Run(context.Background(), fsys)
	if err == nil {
		t.Fatal("want error on duplicate with Error() strategy, got nil")
	}
}

func TestConflictTarget(t *testing.T) {
	db := newDB(t)

	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})
	_, err := New(db).
		Add("users.json", &[]User{}, ConflictTarget("id")).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestDefaultConflict(t *testing.T) {
	db := newDB(t)
	db.Create(&User{ID: 1, Name: "Old", Email: "a@x.io"})

	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"New","Email":"new@x.io"}]`,
	})
	_, err := New(db, WithDefaultConflict(UpdateAll())).
		Add("users.json", &[]User{}).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	var u User
	db.First(&u, 1)
	if u.Name != "New" {
		t.Errorf("name = %q, want New (default UpdateAll)", u.Name)
	}
}

func TestProfiles(t *testing.T) {
	body := map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
		`demo.json`:  `[{"ID":9,"Name":"Demo","Email":"d@x.io"}]`,
	}

	// Without the demo profile active, the tagged spec is skipped.
	db := newDB(t)
	res, _ := New(db).
		Add("users.json", &[]User{}).
		Add("demo.json", &[]User{}, Profile("demo")).
		Run(context.Background(), files(body))
	if res.Inserted() != 1 {
		t.Errorf("without profile inserted %d, want 1", res.Inserted())
	}

	// With it active, both load.
	db2 := newDB(t)
	res2, _ := New(db2, WithProfiles("demo")).
		Add("users.json", &[]User{}).
		Add("demo.json", &[]User{}, Profile("demo")).
		Run(context.Background(), files(body))
	if res2.Inserted() != 2 {
		t.Errorf("with profile inserted %d, want 2", res2.Inserted())
	}
}

func TestDryRun(t *testing.T) {
	db := newDB(t)
	res, err := New(db, WithDryRun()).
		Add("users.json", &[]User{}).
		Run(context.Background(), files(map[string]string{
			`users.json`: `[{"ID":1,"Name":"A","Email":"a@x.io"},{"ID":2,"Name":"B","Email":"b@x.io"}]`,
		}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Specs[0].Planned != 2 || res.Inserted() != 0 {
		t.Errorf("dry run planned=%d inserted=%d, want 2/0", res.Specs[0].Planned, res.Inserted())
	}
	var count int64
	db.Model(&User{}).Count(&count)
	if count != 0 {
		t.Errorf("dry run wrote %d rows, want 0", count)
	}
}

func TestSkipMissing(t *testing.T) {
	db := newDB(t)
	empty := files(map[string]string{})

	if _, err := New(db).Add("nope.json", &[]User{}).Run(context.Background(), empty); err == nil {
		t.Error("want error for missing file without WithSkipMissing")
	}
	res, err := New(db, WithSkipMissing()).Add("nope.json", &[]User{}).Run(context.Background(), empty)
	if err != nil {
		t.Fatalf("want skip, got error: %v", err)
	}
	if !res.Specs[0].Skipped || res.Specs[0].Reason != "missing" {
		t.Errorf("spec = %+v, want skipped/missing", res.Specs[0])
	}
}

func TestContinueOnError(t *testing.T) {
	db := newDB(t)
	res, err := New(db, WithContinueOnError()).
		Add("bad.json", &[]User{}).
		Add("users.json", &[]User{}).
		Run(context.Background(), files(map[string]string{
			`bad.json`:   `{ not valid`,
			`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
		}))
	if err != nil {
		t.Fatalf("continue-on-error should not return: %v", err)
	}
	if res.Specs[0].Err == nil {
		t.Error("first spec should record a decode error")
	}
	if res.Specs[1].Inserted != 1 {
		t.Errorf("second spec inserted %d, want 1", res.Specs[1].Inserted)
	}
}

func TestDirSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.json"),
		[]byte(`[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	db := newDB(t)
	res, err := New(db).Add("users.json", &[]User{}).Run(context.Background(), Dir(dir))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Inserted() != 1 {
		t.Errorf("inserted %d, want 1", res.Inserted())
	}
}

func TestWithTransaction(t *testing.T) {
	db := newDB(t)

	// Valid run within a transaction.
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})
	res, err := New(db, WithTransaction()).
		Add("users.json", &[]User{}).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Inserted() != 1 {
		t.Errorf("inserted %d, want 1", res.Inserted())
	}

	// Transaction with error: nothing should be written.
	db2 := newDB(t)
	_, err = New(db2, WithTransaction()).
		Add("bad.json", &[]User{}).
		Add("users.json", &[]User{}).
		Run(context.Background(), files(map[string]string{
			`bad.json`:   `{invalid`,
			`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
		}))
	if err == nil {
		t.Error("expected transaction error due to bad spec")
	}
	var count int64
	db2.Model(&User{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 users after failed transaction, got %d", count)
	}
}

func TestWithTransactionDryRun(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})
	res, err := New(db, WithTransaction(), WithDryRun()).
		Add("users.json", &[]User{}).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Inserted() != 0 {
		t.Errorf("dry run should not insert, got %d", res.Inserted())
	}
}

func TestEmptyFixture(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[]`,
	})
	res, err := New(db).
		Add("users.json", &[]User{}).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !res.Specs[0].Skipped || res.Specs[0].Reason != "empty" {
		t.Errorf("spec = %+v, want skipped/empty", res.Specs[0])
	}
}

func TestNoDecoder(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.xml`: `<users></users>`,
	})
	_, err := New(db).
		Add("users.xml", &[]User{}).
		Run(context.Background(), fsys)
	if err == nil || !strings.Contains(err.Error(), "no decoder") {
		t.Fatalf("want decoder error, got %v", err)
	}
}

func TestCustomDecoder(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.yaml`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})
	_, err := New(db, WithDecoder(".yaml", json.Unmarshal)).Add("users.yaml", &[]User{}).Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run with custom decoder: %v", err)
	}
}

func TestContextDecoder(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})
	var capturedCtx context.Context
	_, err := New(db, WithDecoderContext(".json", func(ctx context.Context, data []byte, dest any) error {
		capturedCtx = ctx
		return json.Unmarshal(data, dest)
	})).Add("users.json", &[]User{}).Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if capturedCtx == nil {
		t.Error("expected context to be passed to decoder")
	}
}

func TestBatchSize(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"},{"ID":2,"Name":"Bob","Email":"b@x.io"}]`,
	})
	res, err := New(db, WithBatchSize(1)).
		Add("users.json", &[]User{}).
		Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Inserted() != 2 {
		t.Errorf("inserted %d, want 2", res.Inserted())
	}
}

func TestBeforeAfterHooks(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})

	var mu sync.Mutex
	var beforeNames, afterNames []string

	_, err := New(db,
		WithBeforeSeedHook(func(ctx context.Context, name string, planned int) {
			mu.Lock()
			beforeNames = append(beforeNames, name)
			mu.Unlock()
		}),
		WithAfterSeedHook(func(ctx context.Context, name string, planned int) {
			mu.Lock()
			afterNames = append(afterNames, name)
			mu.Unlock()
		}),
	).Add("users.json", &[]User{}).Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(beforeNames) != 1 || beforeNames[0] != "users.json" {
		t.Errorf("before hooks = %v, want [users.json]", beforeNames)
	}
	if len(afterNames) != 1 || afterNames[0] != "users.json" {
		t.Errorf("after hooks = %v, want [users.json]", afterNames)
	}
}

func TestInvalidDestType(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1}]`,
	})

	// Passing a struct instead of pointer-to-slice should fail.
	_, err := New(db).
		Add("users.json", User{}).
		Run(context.Background(), fsys)
	if err == nil {
		t.Fatal("want error for non-pointer-to-slice dest, got nil")
	}
}

func TestNonExistentAfterDep(t *testing.T) {
	db := newDB(t)
	_, err := New(db).
		Add("a.json", &[]User{}, After("nonexistent.json")).
		ordered()
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("want 'not registered' error for After dep, got %v", err)
	}
}

func TestLogger(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})

	var mu sync.Mutex
	var logged []string
	_, err := New(db, WithLogger(LoggerFunc(func(format string, v ...any) {
		mu.Lock()
		logged = append(logged, format)
		mu.Unlock()
	}))).Add("users.json", &[]User{}).Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// No specific assertion; just ensure logger doesn't cause panics.
}

type LoggerFunc func(format string, v ...any)

func (f LoggerFunc) Printf(format string, v ...any) {
	f(format, v...)
}

func TestConcurrentAdd(t *testing.T) {
	db := newDB(t)
	s := New(db)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Add("users.json", &[]User{})
		}(i)
	}
	wg.Wait()
}

func TestClean(t *testing.T) {
	db := newDB(t)
	fsys := files(map[string]string{
		`users.json`: `[{"ID":1,"Name":"Alice","Email":"a@x.io"}]`,
	})
	s := New(db).
		Add("users.json", &[]User{})

	res, err := s.Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Inserted() != 1 {
		t.Fatalf("inserted %d, want 1", res.Inserted())
	}

	// Clean the table
	cleanRes, err := s.Clean(context.Background())
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if cleanRes == nil {
		t.Fatal("expected clean result, got nil")
	}

	// Verify data is gone
	var count int64
	db.Model(&User{}).Count(&count)
	if count != 0 {
		t.Errorf("after clean user count = %d, want 0", count)
	}

	// Should be able to re-seed
	res2, err := s.Run(context.Background(), fsys)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if res2.Inserted() != 1 {
		t.Errorf("re-seed inserted %d, want 1", res2.Inserted())
	}
}

func TestDuplicateDependency(t *testing.T) {
	db := newDB(t)
	s := New(db, WithAutoOrder()).
		Add("a.json", &[]User{}, After("b.json")).
		Add("b.json", &[]User{})

	ordered, err := s.ordered()
	if err != nil {
		t.Fatalf("ordered: %v", err)
	}
	if ordered[0].name != "b.json" || ordered[1].name != "a.json" {
		t.Errorf("order = %v, want [b.json a.json]", names(ordered))
	}
}

func TestRowCountInvalidType(t *testing.T) {
	if n := rowCount(42); n != 0 {
		t.Errorf("rowCount(42) = %d, want 0", n)
	}
}

func TestElemModelError(t *testing.T) {
	_, err := elemModel(42)
	if err == nil || !strings.Contains(err.Error(), "must be a pointer to a slice") {
		t.Errorf("want error for non-pointer, got %v", err)
	}
}

func TestSpecResultInserted(t *testing.T) {
	r := &Result{
		Specs: []SpecResult{
			{Inserted: 1},
			{Inserted: 2},
			{Inserted: 3},
		},
	}
	if r.Inserted() != 6 {
		t.Errorf("Inserted() = %d, want 6", r.Inserted())
	}
}

func TestConflictUpdateAllClause(t *testing.T) {
	c := UpdateAll()
	expr, ok := c.clause(nil)
	if !ok {
		t.Error("UpdateAll clause should return ok=true")
	}
	if expr == nil {
		t.Error("UpdateAll clause should return a non-nil expression")
	}
}

func TestErrorWithNilCheck(t *testing.T) {
	err1 := errors.New("test error")
	err2 := errors.New("test error")
	if err1.Error() != err2.Error() {
		t.Error("error message mismatch")
	}
}
