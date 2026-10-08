package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoxRoundTrip(t *testing.T) {
	key, err := NewKey()
	require.NoError(t, err)
	parsed, err := ParseKey(EncodeKey(key))
	require.NoError(t, err)
	box, err := NewBox(parsed)
	require.NoError(t, err)

	ct, err := box.Seal([]byte("linode-token"))
	require.NoError(t, err)
	assert.NotContains(t, string(ct), "linode-token")

	pt, err := box.Open(ct)
	require.NoError(t, err)
	assert.Equal(t, "linode-token", string(pt))

	other, _ := NewKey()
	otherBox, _ := NewBox(other)
	_, err = otherBox.Open(ct)
	assert.Error(t, err)
}

func TestParseKeyRejectsBadLength(t *testing.T) {
	_, err := ParseKey("abcd")
	assert.Error(t, err)
}

func TestTokens(t *testing.T) {
	tok, err := NewToken("gu_")
	require.NoError(t, err)
	assert.Len(t, tok, 3+48)
	assert.Equal(t, "gu_", tok[:3])
	assert.Equal(t, tok[:10], Prefix(tok))
	assert.NotEqual(t, tok, Hash(tok))
	assert.Len(t, Hash(tok), 64)
}
