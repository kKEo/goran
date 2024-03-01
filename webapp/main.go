package main

import (
	"log"
	"net/http"

	"gorm.io/gorm"
	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/api"
	"github.com/kkEo/g-mk8s/webapp/db"
	"github.com/kkEo/g-mk8s/webapp/middleware"
)

type Config struct {
	Database *gorm.DB
	SkipAuth bool
}


func SetupApp(c *Config) *gin.Engine {
	app := gin.Default()
	database := c.Database

	app.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello, world!",
		})
	})

	protected := app.Group("/protected")

	log.Println("SkipAuth: ", c.SkipAuth)
	if c.SkipAuth == false {
		authMidleware := middleware.AuthMiddleware{DB: database}
		protected.Use(authMidleware.Handle())
	}

	userApiHandlers := &api.UserHandlers{DB: database}
	app.GET("/users/:name", userApiHandlers.GetUser)
	protected.GET("/users", userApiHandlers.GetUsers)
	protected.POST("/users", userApiHandlers.PostUser)

	tokenApiHandlers := &api.TokenHandlers{DB: database}
	protected.POST("/tokens", tokenApiHandlers.PostToken)
	protected.GET("/users/:name/tokens", tokenApiHandlers.GetTokens)

	// TODO: Introduce common user ownership
	// protectedUserOwned := protected.Group("users/:user")

	blueprintHandlers := &api.BlueprintHandlers{DB: database}
	protected.POST("/blueprints", blueprintHandlers.PostBlueprint)
	protected.PUT("/blueprints", blueprintHandlers.PutBlueprint)
	protected.GET("/blueprints/:name", blueprintHandlers.GetBlueprint)

	protected.GET("/next", blueprintHandlers.GetNext)

	return app
}

func main() {
	config := Config{
		Database: db.Init(),
	}

	app := SetupApp(&config)

	log.Println("Starting server on :8080")
	app.Run(":8080")
}
