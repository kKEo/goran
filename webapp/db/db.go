package db

import (
	"github.com/kkEo/g-mk8s/webapp/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

func Init() *gorm.DB {

	newLogger := logger.New(
		log.New(os.Stdout, "\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second, // Slow SQL threshold
			LogLevel:      logger.Info, // Log level
			Colorful:      true,        // Disable color
		},
	)

	db, err := gorm.Open(sqlite.Open("local.sqlite"), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		log.Fatal("Failed to connect database", err)
	}
	db.AutoMigrate(&model.User{})
	db.AutoMigrate(&model.ApiToken{})
	db.AutoMigrate(&model.Blueprint{})
	return db
}
