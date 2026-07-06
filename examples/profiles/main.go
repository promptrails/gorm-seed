// Command profiles demonstrates profile-based seeding. Tag specs with Profile
// to load them only when the corresponding profile is active.
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

func main() {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		panic(err)
	}

	// Specify fixtures for "development" and "demo" with Profile tag.
	// Users tagged with "demo" only load when WithProfiles("demo") is active.
	seeder := gormseed.New(db, gormseed.WithProfiles("demo")).
		Add("users.json", &[]User{}).
		Add("demo_users.json", &[]User{}, gormseed.Profile("demo"))

	res, err := seeder.Run(context.Background(), gormseed.Dir("fixtures"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("seeded %d rows\n", res.Inserted())

	var count int64
	db.Model(&User{}).Count(&count)
	fmt.Printf("total users: %d\n", count)
}
