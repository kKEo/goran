package model

import (
	"gorm.io/gorm"
)

type Blueprint struct {
	gorm.Model

	Name        string `json:"name" binding:"required"`
	Machine     string `json:"machine"`
	RemoteToken string `json:"api_token" binding:"required"`
}
