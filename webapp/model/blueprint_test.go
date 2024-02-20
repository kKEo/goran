package model

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestStatusSerialization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("Cannot connect database")
	}

	db.AutoMigrate(&Blueprint{})

	model := Blueprint{Name: "b1", RemoteToken: "aaa", Status: New}
	db.Create(&model)

	var queriedModel Blueprint
	db.First(&queriedModel, "name = ?", "b1")
	fmt.Printf("Queried Model: %+v\n", queriedModel)

	assert.Equal(t, New, queriedModel.Status)
}
