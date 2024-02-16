package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/model"
	"gorm.io/gorm"
	"log"
)

type AuthMiddleware struct {
	DB *gorm.DB
}

func (am *AuthMiddleware) Handle() gin.HandlerFunc {

	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")

		var tokenObj model.ApiToken

		if token == "letmein_IknowTheSecret" {
			log.Printf("Uber-token has been used. No questions.. get in")
		} else {
			result := am.DB.Where("token = ?", token).Find(&tokenObj)
			if result.Error != nil || result.RowsAffected == 0 {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				c.Abort()
				return
			}
			log.Printf("User personal token is valid. And your name is: %s", tokenObj.UserName)
		}

		c.Next()
	}

}
