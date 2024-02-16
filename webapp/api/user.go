package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/model"
	"gorm.io/gorm"
	"net/http"
)

type UserHandlers struct {
	DB *gorm.DB
}

func (h *UserHandlers) GetUser(c *gin.Context) {
	var user model.User
	name := c.Param("name")

	result := h.DB.First(&user, "name = ?", name)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandlers) GetUsers(c *gin.Context) {
	var users []model.User

	result := h.DB.Find(&users)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Users not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *UserHandlers) PostUser(c *gin.Context) {
	var newUser model.User
	if err := c.BindJSON(&newUser); err != nil {
		c.AbortWithStatusJSON(400, gin.H{"error": "Bad request"})
		return
	}

	h.DB.Create(&newUser)
	c.JSON(200, newUser)
}
