package model

import (
	"gorm.io/gorm"
)

type ApiToken struct {
	gorm.Model
	Name     string `json:"name" binding:"required"`
	UserName string `json:"user_name" binding:"required"`
	Token    string `json:"api_key"`
}
