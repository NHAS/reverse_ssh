package data

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

func LoadDatabase(path string) (err error) {
	// Connect to the SQLite database (you can replace it with other supported databases)
	db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}

	// AutoMigrate will create the table if it does not exist, or update it if it has changed
	err = db.AutoMigrate(&Webhook{}, &Download{})
	if err != nil {
		return err
	}

	return nil
}
