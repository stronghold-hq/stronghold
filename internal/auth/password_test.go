package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("mysecretpassword")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "$2a$"), "expected bcrypt hash prefix $2a$, got: %s", hash)
}

func TestHashPassword_DifferentHashes(t *testing.T) {
	password := "samepassword"

	hash1, err := HashPassword(password)
	require.NoError(t, err)

	hash2, err := HashPassword(password)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "hashing the same password twice should produce different hashes due to random salt")
}

func TestCheckPassword_Correct(t *testing.T) {
	password := "correctpassword"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	err = CheckPassword(hash, password)
	assert.NoError(t, err, "checking the correct password against its hash should succeed")
}

func TestCheckPassword_Wrong(t *testing.T) {
	password := "correctpassword"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	err = CheckPassword(hash, "wrongpassword")
	assert.Error(t, err, "checking a wrong password against the hash should fail")
}

func TestCheckPassword_Empty(t *testing.T) {
	password := "nonemptypassword"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	err = CheckPassword(hash, "")
	assert.Error(t, err, "checking an empty password against a non-empty password hash should fail")
}
