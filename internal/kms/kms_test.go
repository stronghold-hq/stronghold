package kms

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew_RequiresRegion(t *testing.T) {
	_, err := New(context.Background(), &Config{
		Region: "",
		KeyID:  "alias/test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "region is required")
}

func TestNew_RequiresKeyID(t *testing.T) {
	_, err := New(context.Background(), &Config{
		Region: "us-east-1",
		KeyID:  "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key ID is required")
}

func TestClient_EncryptRejectsEmpty(t *testing.T) {
	// This test would require AWS credentials to run
	// Skip if no credentials available
	t.Skip("Requires AWS credentials")
}

func TestClient_DecryptRejectsEmpty(t *testing.T) {
	// This test would require AWS credentials to run
	// Skip if no credentials available
	t.Skip("Requires AWS credentials")
}

func TestClient_KeyID(t *testing.T) {
	// Can't test without credentials, but we can verify the method exists
	t.Skip("Requires AWS credentials")
}
