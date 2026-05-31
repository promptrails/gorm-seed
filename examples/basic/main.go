// Command basic seeds an in-memory SQLite database from embedded JSON fixtures,
// demonstrating foreign-key-safe ordering and idempotent re-runs.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/glebarez/sqlite"
	gormseed "github.com/promptrails/gorm-seed"
	"gorm.io/gorm"
)

//go:embed fixtures/*.json
var fixtures embed.FS

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

func main() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(&User{}, &Post{}); err != nil {
		panic(err)
	}

	sub, err := fs.Sub(fixtures, "fixtures")
	if err != nil {
		panic(err)
	}

	// Posts are registered first, but WithAutoOrder loads users first because
	// Post belongs to User.
	seeder := gormseed.New(db, gormseed.WithAutoOrder()).
		Add("posts.json", &[]Post{}).
		Add("users.json", &[]User{})

	res, err := seeder.Run(context.Background(), sub)
	if err != nil {
		panic(err)
	}
	fmt.Printf("first run: %d rows inserted\n", res.Inserted())

	// Running again is a no-op under the default Skip strategy.
	res, _ = seeder.Run(context.Background(), sub)
	fmt.Printf("second run: %d rows inserted (idempotent)\n", res.Inserted())

	var users, posts int64
	db.Model(&User{}).Count(&users)
	db.Model(&Post{}).Count(&posts)
	fmt.Printf("database now holds %d users, %d posts\n", users, posts)
}
