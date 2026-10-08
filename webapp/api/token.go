package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/middleware"
	"github.com/kkEo/g-mk8s/webapp/model"
	"github.com/kkEo/g-mk8s/webapp/util"
)

// issueUserToken creates a token for a user and returns the plaintext once.
func issueUserToken(db *gorm.DB, userID uint, name string) (string, *model.ApiToken, error) {
	raw, err := util.NewToken("gu_")
	if err != nil {
		return "", nil, err
	}
	t := model.ApiToken{Name: name, UserID: userID, Prefix: util.Prefix(raw), Hash: util.Hash(raw)}
	if err := db.Create(&t).Error; err != nil {
		return "", nil, err
	}
	return raw, &t, nil
}

type tokenInput struct {
	Name string `json:"name"`
}

func (s *Server) respondNewToken(c *gin.Context, raw string, t *model.ApiToken) {
	c.JSON(http.StatusCreated, gin.H{
		"id": t.ID, "name": t.Name, "prefix": t.Prefix, "user_id": t.UserID,
		"token": raw,
		"note":  "store this token now; it is not shown again",
	})
}

// createToken issues a token for the caller.
func (s *Server) createToken(c *gin.Context) {
	var in tokenInput
	if c.Request.ContentLength > 0 && !bind(c, &in) {
		return
	}
	if in.Name == "" {
		in.Name = "token"
	}
	raw, t, err := issueUserToken(s.db, middleware.CurrentUser(c).ID, in.Name)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondNewToken(c, raw, t)
}

// createTokenForUser lets an admin issue a token for someone else.
func (s *Server) createTokenForUser(c *gin.Context) {
	var in tokenInput
	if c.Request.ContentLength > 0 && !bind(c, &in) {
		return
	}
	var u model.User
	if err := s.db.Where("name = ?", c.Param("name")).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, "user not found")
			return
		}
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if in.Name == "" {
		in.Name = "token"
	}
	raw, t, err := issueUserToken(s.db, u.ID, in.Name)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondNewToken(c, raw, t)
}

// listTokens shows the caller's tokens without their secret part.
func (s *Server) listTokens(c *gin.Context) {
	var ts []model.ApiToken
	s.db.Where("user_id = ?", middleware.CurrentUser(c).ID).Order("id").Find(&ts)
	if ts == nil {
		ts = []model.ApiToken{}
	}
	c.JSON(http.StatusOK, ts)
}

func (s *Server) deleteToken(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	u := middleware.CurrentUser(c)
	q := s.db.Where("id = ?", id)
	if !u.Admin {
		q = q.Where("user_id = ?", u.ID)
	}
	res := q.Delete(&model.ApiToken{})
	if res.RowsAffected == 0 {
		fail(c, http.StatusNotFound, "token not found")
		return
	}
	c.Status(http.StatusNoContent)
}
