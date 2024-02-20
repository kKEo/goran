package model

import (
	"database/sql/driver"
	"gorm.io/gorm"
)

type Status string

const (
	New     Status = "new"
	Pending Status = "pending"
	Done    Status = "done"
	Error   Status = "error"
)

func (e *Status) Scan(value interface{}) error {
	*e = Status(value.(string))
	return nil
}

func (e Status) Value() (driver.Value, error) {
	return string(e), nil
}

type Blueprint struct {
	gorm.Model

	Name        string `json:"name" binding:"required"`
	Machine     string `json:"machine"`
	RemoteToken string `json:"api_token" binding:"required"`
	Status      Status `json:"status"`
}
