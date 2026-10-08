package api

import (
	"errors"

	"gorm.io/gorm"

	"github.com/kkEo/g-mk8s/webapp/model"
)

// BootstrapResult is what the `bootstrap` command prints.
type BootstrapResult struct {
	User      model.User
	Workspace model.Workspace
	Token     string
}

// Bootstrap creates (or reuses) an admin user and a workspace and issues a
// fresh token. It is the only way to obtain the first credential: there is no
// built-in master token.
func Bootstrap(db *gorm.DB, userName, email, workspaceName string) (*BootstrapResult, error) {
	if !validName(userName) || !validName(workspaceName) {
		return nil, errors.New("user and workspace names must be 1-64 letters, digits, '_', '.' or '-' and not purely numeric")
	}
	var u model.User
	err := db.Where("name = ?", userName).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		u = model.User{Name: userName, Email: email, Admin: true}
		err = db.Create(&u).Error
	} else if err == nil && !u.Admin {
		err = db.Model(&u).Update("admin", true).Error
	}
	if err != nil {
		return nil, err
	}
	var ws model.Workspace
	err = db.Where("name = ?", workspaceName).First(&ws).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ws = model.Workspace{Name: workspaceName}
		err = db.Create(&ws).Error
	}
	if err != nil {
		return nil, err
	}
	var m model.Membership
	err = db.Where("workspace_id = ? AND user_id = ?", ws.ID, u.ID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = db.Create(&model.Membership{WorkspaceID: ws.ID, UserID: u.ID, Role: model.RoleAdmin}).Error
	}
	if err != nil {
		return nil, err
	}
	raw, _, err := issueUserToken(db, u.ID, "bootstrap")
	if err != nil {
		return nil, err
	}
	return &BootstrapResult{User: u, Workspace: ws, Token: raw}, nil
}
