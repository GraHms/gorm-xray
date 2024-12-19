package main

import (
	"github.com/grahms/gormxray"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

func main() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}

	// Register the plugin
	if err := db.Use(gormxray.NewPlugin(gormxray.WithExcludeQueryVars(true))); err != nil {
		log.Fatal("failed to register plugin")
	}

	// Perform a sample query to see gormxray in action
	var result int
	db.Raw("SELECT 1").Scan(&result)
}
