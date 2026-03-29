package auth

import (
	"context"

	"s3-storage/model"
)

// AuthProvider defines the interface for authentication providers
type AuthProvider interface {
	// Name returns the provider name
	Name() string

	// Authenticate validates credentials and returns a user
	Authenticate(ctx context.Context, credentials map[string]string) (*model.User, error)

	// GetLoginURL returns the OAuth login URL (for OAuth providers)
	// Returns empty string for non-OAuth providers
	GetLoginURL(state string) string

	// HandleCallback processes OAuth callback
	// Returns nil user for non-OAuth providers
	HandleCallback(ctx context.Context, code, state string) (*model.User, error)

	// SupportsOAuth returns true if this provider uses OAuth flow
	SupportsOAuth() bool
}

// Credentials keys
const (
	CredentialUsername = "username"
	CredentialPassword = "password"
)

// Provider names
const (
	ProviderLocal   = "local"
	ProviderConfig  = "config"
	ProviderAzureAD = "azuread"
)
