package api

import (
	"log"
	"net/http"

	"gorm.io/gorm"

	"errors"
	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/util"
)

type TokenHandlers struct {
	DB *gorm.DB
}

func (h *TokenHandlers) GetTokens(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The `name` parameter is required"})
		return
	}

	var tokens []model.ApiToken
	result := h.DB.Where("user_name = ?", name).Find(&tokens)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

func (h *TokenHandlers) PostToken(c *gin.Context) {
	var newToken model.ApiToken
	if err := c.BindJSON(&newToken); err != nil {
		c.AbortWithStatusJSON(400, gin.H{"error": "Bad request"})
		return
	}

	if newToken.Token == "" {
		log.Println("Token not provided - let's generate one")
		// todo: make encoding type read from the environment variable
		var encodingType = util.Hex
		token, err := util.GenerateToken(8, encodingType)
		if err != nil {
			log.Println("Error: ", err)
			c.AbortWithStatusJSON(500, gin.H{"error": "Internal server error"})
			return
		}
		newToken.Token = token
	}

	result := h.DB.Create(&newToken)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		c.AbortWithStatusJSON(500, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, newToken)
}
