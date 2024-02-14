package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/api"
	"github.com/kkEo/g-mk8s/webapp/db"
	"github.com/kkEo/g-mk8s/webapp/middleware"
)

func SetupApp() *gin.Engine {
	app := gin.Default()

	log.Println("Init database")
	database := db.Init()

	log.Println("Database initialized")
	app.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, world!",
		})
	})

	protected := app.Group("/protected")
	authMidleware := middleware.AuthMiddleware{DB: database}
	protected.Use(authMidleware.Handle())

	userApiHandlers := &api.UserHandlers{DB: database}
	app.GET("/users/:name", userApiHandlers.GetUser)
	protected.POST("/users", userApiHandlers.PostUser)

	tokenApiHandlers := &api.TokenHandlers{DB: database}
	protected.POST("/tokens", tokenApiHandlers.PostToken)
	protected.GET("/users/:name/tokens", tokenApiHandlers.GetTokens)

	return app
}

func main() {
	app := SetupApp()
	log.Println("Starting server on :8080")
	app.Run(":8080")
}
