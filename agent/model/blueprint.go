package model

type Blueprint struct {
	Name        string `json:"name" binding:"required"`
	Machine     string `json:"machine"`
	RemoteToken string `json:"api_token" binding:"required"`
}
