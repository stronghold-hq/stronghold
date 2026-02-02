// Package kms provides AWS KMS encryption for wallet private keys
package kms

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// Config holds KMS client configuration
type Config struct {
	Region string
	KeyID  string
}

// Client wraps AWS KMS for encrypting wallet private keys
type Client struct {
	kms   *kms.Client
	keyID string
}

// New creates a new KMS client
// It uses AWS SDK's default credential chain (env vars, IAM role, etc.)
func New(ctx context.Context, cfg *Config) (*Client, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("KMS region is required")
	}
	if cfg.KeyID == "" {
		return nil, fmt.Errorf("KMS key ID is required")
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		kms:   kms.NewFromConfig(awsCfg),
		keyID: cfg.KeyID,
	}, nil
}

// Encrypt encrypts a private key hex string using KMS
// Returns base64-encoded ciphertext suitable for database storage
func (c *Client) Encrypt(ctx context.Context, privateKeyHex string) (string, error) {
	if privateKeyHex == "" {
		return "", fmt.Errorf("private key cannot be empty")
	}

	input := &kms.EncryptInput{
		KeyId:     aws.String(c.keyID),
		Plaintext: []byte(privateKeyHex),
	}

	result, err := c.kms.Encrypt(ctx, input)
	if err != nil {
		return "", fmt.Errorf("KMS encrypt failed: %w", err)
	}

	// Return base64-encoded ciphertext for safe storage in TEXT column
	return base64.StdEncoding.EncodeToString(result.CiphertextBlob), nil
}

// Decrypt decrypts a base64-encoded ciphertext back to the private key hex string
func (c *Client) Decrypt(ctx context.Context, encryptedKey string) (string, error) {
	if encryptedKey == "" {
		return "", fmt.Errorf("encrypted key cannot be empty")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return "", fmt.Errorf("invalid base64 encoding: %w", err)
	}

	input := &kms.DecryptInput{
		CiphertextBlob: ciphertext,
	}

	result, err := c.kms.Decrypt(ctx, input)
	if err != nil {
		return "", fmt.Errorf("KMS decrypt failed: %w", err)
	}

	return string(result.Plaintext), nil
}

// KeyID returns the KMS key ID/ARN being used
func (c *Client) KeyID() string {
	return c.keyID
}
