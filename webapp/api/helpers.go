package api

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kkEo/g-mk8s/webapp/middleware"
)

var (
	nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	envRe  = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	allNum = regexp.MustCompile(`^[0-9]+$`)
)

func fail(c *gin.Context, status int, msg string) { middleware.Fail(c, status, msg) }

func bind(c *gin.Context, v interface{}) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		fail(c, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func paramID(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		fail(c, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return uint(id), true
}

// validName accepts short identifiers for users, workspaces, secrets and agents.
// Purely numeric names are rejected so :ws can be either a name or an id.
func validName(s string) bool { return nameRe.MatchString(s) && !allNum.MatchString(s) }
