package model

import "time"

// ApiToken authenticates a user. Only the SHA-256 hash is stored; the plaintext
// is returned once at creation and never again.
type ApiToken struct {
	Base
	Name       string     `gorm:"size:64" json:"name"`
	UserID     uint       `gorm:"index" json:"user_id"`
	Prefix     string     `gorm:"size:12" json:"prefix"`
	Hash       string     `gorm:"uniqueIndex;size:64" json:"-"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// RegistrationToken is a one-time, expiring credential that lets a new agent
// join a workspace. It is consumed by POST /agent/register.
type RegistrationToken struct {
	Base
	WorkspaceID uint       `gorm:"index" json:"workspace_id"`
	Prefix      string     `gorm:"size:12" json:"prefix"`
	Hash        string     `gorm:"uniqueIndex;size:64" json:"-"`
	ExpiresAt   time.Time  `json:"expires_at"`
	UsedAt      *time.Time `json:"used_at"`
	AgentID     *uint      `json:"agent_id"`
}

// Agent is a registered runner. Its key is stored hashed like a user token.
type Agent struct {
	Base
	WorkspaceID uint       `gorm:"index" json:"workspace_id"`
	Name        string     `gorm:"size:64" json:"name"`
	Labels      Labels     `gorm:"type:text" json:"labels"`
	KeyPrefix   string     `gorm:"size:12" json:"key_prefix"`
	KeyHash     string     `gorm:"uniqueIndex;size:64" json:"-"`
	LastSeenAt  *time.Time `json:"last_seen_at"`
}

// Secret is a workspace-scoped value encrypted at rest with the server master key.
// The API never returns the value; it is only decrypted into the environment of
// a task at the moment an agent claims it.
type Secret struct {
	Base
	WorkspaceID uint   `gorm:"uniqueIndex:idx_secret" json:"workspace_id"`
	Name        string `gorm:"uniqueIndex:idx_secret;size:64" json:"name"`
	Ciphertext  []byte `json:"-"`
}
