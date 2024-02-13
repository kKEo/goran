package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/api"
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

	userApiHandlers := &api.UserHandlers{DB: database}
	router.GET("/users/:name", userApiHandlers.GetUser)
	protected.POST("/users", userApiHandlers.PostUser)

	log.Println("Starting server on :8080")
	router.Run(":8080")

}
