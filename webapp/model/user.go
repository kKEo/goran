package model

import "time"

// Base replaces gorm.Model with snake_case JSON and no soft deletes.
type Base struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User is a person who talks to the API with an ApiToken.
// Admin users may create other users and tokens for them and see every workspace.
type User struct {
	Base
	Name  string `gorm:"uniqueIndex;size:64" json:"name"`
	Email string `gorm:"size:255" json:"email"`
	Admin bool   `json:"admin"`
}

// Workspace is the tenancy boundary: one per client or environment.
// Tasks, agents, secrets and memberships all belong to exactly one workspace.
type Workspace struct {
	Base
	Name string `gorm:"uniqueIndex;size:64" json:"name"`
}

// Membership roles.
const (
	RoleAdmin  = "admin"  // manage members, secrets, agents; approve plans
	RoleMember = "member" // create and watch tasks
)

// Membership links a user to a workspace with a role.
type Membership struct {
	Base
	WorkspaceID uint   `gorm:"uniqueIndex:idx_membership" json:"workspace_id"`
	UserID      uint   `gorm:"uniqueIndex:idx_membership" json:"user_id"`
	Role        string `gorm:"size:16" json:"role"`
}
