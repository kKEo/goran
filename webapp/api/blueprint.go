package api

import (
	"net/http"

	"gorm.io/gorm"

	"errors"
	"github.com/gin-gonic/gin"
	"github.com/kkEo/g-mk8s/webapp/model"
)

type BlueprintHandlers struct {
	DB *gorm.DB
}

func (h *BlueprintHandlers) GetBlueprint(c *gin.Context) {

	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The `name` parameter must be none-empty"})
		return
	}

	var blueprint model.Blueprint
	result := h.DB.First(&blueprint, "name = ?", name)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Blueprint not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, blueprint)
}

func (h *BlueprintHandlers) PostBlueprint(c *gin.Context) {
	var blueprint model.Blueprint

	if err := c.BindJSON(&blueprint); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	var blueprintDb model.Blueprint
	result := h.DB.First(&blueprintDb, "name = ?", blueprint.Name)
	if result.RowsAffected > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Blueprint already exists. Use PUT for update"})
		return
	}

	h.DB.Create(&blueprint)
	c.JSON(http.StatusCreated, blueprint)
}

func (h *BlueprintHandlers) PutBlueprint(c *gin.Context) {
	var blueprint model.Blueprint

	if err := c.BindJSON(&blueprint); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	if blueprint.ID == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Bad request. ID is not set"})
		return
	}

	h.DB.Save(&blueprint)
	c.JSON(http.StatusOK, blueprint)
}
