package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
)

func (s *Server) me(c *gin.Context) {
	u := middleware.CurrentUser(c)
	var ms []model.Membership
	s.db.Where("user_id = ?", u.ID).Find(&ms)
	if ms == nil {
		ms = []model.Membership{}
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "memberships": ms})
}

func (s *Server) listUsers(c *gin.Context) {
	var users []model.User
	if err := s.db.Order("id").Find(&users).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, users)
}

type userInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Admin bool   `json:"admin"`
}

func (s *Server) createUser(c *gin.Context) {
	var in userInput
	if !bind(c, &in) {
		return
	}
	if !validName(in.Name) {
		fail(c, http.StatusBadRequest, "name must be 1-64 letters, digits, '_', '.' or '-' and not purely numeric")
		return
	}
	var existing model.User
	if err := s.db.Where("name = ?", in.Name).First(&existing).Error; err == nil {
		fail(c, http.StatusConflict, "user already exists")
		return
	}
	u := model.User{Name: in.Name, Email: in.Email, Admin: in.Admin}
	if err := s.db.Create(&u).Error; err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, u)
}
