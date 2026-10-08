package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCustomColumnTypesRoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Task{}))

	in := Task{
		Name:    "t1",
		Kind:    "shell",
		Params:  JSON(`{"script":"echo hi"}`),
		Secrets: StringMap{"TOKEN": "linode"},
		Labels:  NormalizeLabels([]string{"eu ", "b", "eu", ""}),
		Status:  StatusNew,
	}
	require.NoError(t, db.Create(&in).Error)

	var out Task
	require.NoError(t, db.First(&out, in.ID).Error)
	assert.Equal(t, StatusNew, out.Status)
	assert.JSONEq(t, `{"script":"echo hi"}`, string(out.Params))
	assert.Equal(t, StringMap{"TOKEN": "linode"}, out.Secrets)
	assert.Equal(t, Labels{"b", "eu"}, out.Labels)
}

func TestLabelsContainsAll(t *testing.T) {
	have := NormalizeLabels([]string{"eu", "prod"})
	assert.True(t, have.ContainsAll(NormalizeLabels(nil)))
	assert.True(t, have.ContainsAll(Labels{"eu"}))
	assert.False(t, have.ContainsAll(Labels{"eu", "us"}))
}

func TestStatusTerminal(t *testing.T) {
	assert.True(t, StatusDone.Terminal())
	assert.True(t, StatusRejected.Terminal())
	assert.False(t, StatusAwaitingApproval.Terminal())
	assert.False(t, StatusNew.Terminal())
}
