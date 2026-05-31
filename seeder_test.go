package gormseed

import (
	"context"
	"os"
	"path/filepath"
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
