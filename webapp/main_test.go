package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateUser(t *testing.T) {
	router := SetupApp()

	var jsonStr = []byte(`{"name": "John", "email": "john@example.com"}`)

	req, _ := http.NewRequest("POST", "/protected/users", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, err)

	assert.Equal(t, "John", resp["Name"])
	assert.Equal(t, "john@example.com", resp["Email"])
}

func TestCreateNewApiKey(t *testing.T) {
	router := SetupApp()

	var jsonStr = []byte(`{"name": "t1", "user_name": "John"}`)
	req, _ := http.NewRequest("POST", "/protected/tokens", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	log.Println(w.Body.String())

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, err)

	assert.Equal(t, "t1", resp["name"])
	assert.Equal(t, "John", resp["user_name"])
	assert.NotEmpty(t, resp["api_key"])
}

func TestCreateCustomApiKey(t *testing.T) {
	router := SetupApp()

	var jsonStr = []byte(`{"name": "t1", "user_name": "John", "api_key": "johnsapikey"}`)
	req, _ := http.NewRequest("POST", "/protected/tokens", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	log.Println(w.Body.String())

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(t, err)

	assert.Equal(t, "t1", resp["name"])
	assert.Equal(t, "John", resp["user_name"])
	assert.Equal(t, "johnsapikey", resp["api_key"])
}
