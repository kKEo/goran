package main

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/db"
	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
)

func main() {

	router := gin.Default()

	log.Println("Init database")

	database := db.Init()

	log.Println("Database initialized")

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, world!",
		})
	})

	protected := router.Group("/protected")
	protected.Use(middleware.AuthMiddleware())

	protected.GET("/greet/:name", func(c *gin.Context) {
		name := c.Param("name")
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Hello, %s", name),
		})
	})

	protected.POST("/users", func(c *gin.Context) {
		var newUser model.User
		if err := c.BindJSON(&newUser); err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "Bad request"})
			return
		}

		database.Create(&newUser)
		c.JSON(200, newUser)
	})

	router.GET("/users/:name", func(c *gin.Context) {
		var user model.User
		name := c.Param("name")

		result := database.First(&user, "name = ?", name)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	})

	log.Println("Starting server on :8080")

	router.Run(":8080")

}
