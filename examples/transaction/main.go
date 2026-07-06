// Command transaction demonstrates transactional seeding. If any spec fails,
// all prior inserts are rolled back.
package main

import (
	"context"
	"fmt"

	"github.com/glebarez/sqlite"
	gormseed "github.com/promptrails/gorm-seed"
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

func main() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(&User{}, &Post{}); err != nil {
		panic(err)
	}

	// WithTransaction wraps all inserts in a single transaction. If any spec
	// fails, everything is rolled back.
	seeder := gormseed.New(db,
		gormseed.WithTransaction(),
		gormseed.WithAutoOrder(),
	).
		Add("users.json", &[]User{}).
		Add("posts.json", &[]Post{})

	res, err := seeder.Run(context.Background(), gormseed.Dir("fixtures"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("transactional run: %d rows inserted\n", res.Inserted())

	var users, posts int64
	db.Model(&User{}).Count(&users)
	db.Model(&Post{}).Count(&posts)
	fmt.Printf("users=%d posts=%d\n", users, posts)
}
