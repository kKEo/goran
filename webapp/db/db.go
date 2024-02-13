package db

import (
	"github.com/kkEo/g-mk8s/webapp/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
)

func Init() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("local.sqlite"))
	if err != nil {
		log.Fatal("Failed to connect database", err)
	}
	db.AutoMigrate(&model.User{})
	return db
}
