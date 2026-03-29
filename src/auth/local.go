package auth

import (
	"context"
	"errors"
	"time"

	"s3-storage/model"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")
)

// UserLookupFunc is a function that looks up a user by username
type UserLookupFunc func(username string) (*model.User, error)

// LocalProvider implements username/password authentication
type LocalProvider struct {
	lookupUser    UserLookupFunc
	bootstrapUser *model.User
	bcryptCost    int
}

// NewLocalProvider creates a new local authentication provider
// If bootstrapUsername and bootstrapPassword are provided, a bootstrap admin user will be created
// The bootstrap user is intended for initial setup and should be removed or changed after first login
// Parameters:
// - lookupUser: function to look up users by username (required)
// - bootstrapUsername: optional username for a bootstrap admin user
// - bootstrapPassword: optional password for the bootstrap admin user (must be at least 8 characters if provided)
// Returns:
// - *LocalProvider: the initialized local provider instance
// Example usage:
//
//	lookupFunc := func(username string) (*model.User, error) {
//		// Implement user lookup logic (e.g., query database)
//		return nil, nil
//	}
//	localProvider := NewLocalProvider(lookupFunc, "admin", "SecurePassword123")
func NewLocalProvider(lookupUser UserLookupFunc, bootstrapUsername, bootstrapPassword string) *LocalProvider {
	provider := &LocalProvider{
		lookupUser: lookupUser,
		bcryptCost: 12,
	}

	// Create bootstrap admin if credentials provided
	if bootstrapUsername != "" && bootstrapPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), provider.bcryptCost)
		if err == nil {
			provider.bootstrapUser = &model.User{
				ID:           "bootstrap-admin",
				Username:     bootstrapUsername,
				PasswordHash: string(hash),
				DisplayName:  "Administrator",
				Roles:        []string{"admin", "user"},
				Provider:     ProviderConfig,
				IsBootstrap:  true,
				CreatedAt:    time.Now().Unix(),
				UpdatedAt:    time.Now().Unix(),
			}
		}
	}

	return provider
}

// Name returns the provider name
// This is used to identify the provider in the system
// For the local provider, it returns "local"
// Example usage:
//
//	provider := NewLocalProvider(nil, "", "")
//	fmt.Println(provider.Name()) // Output: "local"
func (p *LocalProvider) Name() string {
	return ProviderLocal
}

// Authenticate validates username and password
// It first checks the bootstrap user (if configured) and then looks up the user in storage
// Parameters:
// - ctx: the context for the authentication operation
// - credentials: a map containing "username" and "password" keys
// Returns:
// - *model.User: the authenticated user if successful
// - error: an error if authentication fails (e.g., invalid credentials)
// Example usage:
//
//	provider := NewLocalProvider(nil, "admin", "SecurePassword123")
//	user, err := provider.Authenticate(context.Background(), map[string]string{"username": "admin", "password": "SecurePassword123"})
//	if err != nil {
//		fmt.Println("Authentication failed:", err)
//	} else {
//		fmt.Println("Authenticated user:", user.Username)
//	}
func (p *LocalProvider) Authenticate(ctx context.Context, credentials map[string]string) (*model.User, error) {
	username := credentials[CredentialUsername]
	password := credentials[CredentialPassword]

	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// Check bootstrap user first
	if p.bootstrapUser != nil && p.bootstrapUser.Username == username {
		if err := bcrypt.CompareHashAndPassword([]byte(p.bootstrapUser.PasswordHash), []byte(password)); err == nil {
			return p.bootstrapUser, nil
		}
	}

	// Look up user in storage
	if p.lookupUser == nil {
		return nil, ErrInvalidCredentials
	}

	user, err := p.lookupUser(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// GetLoginURL returns empty string (not OAuth)
func (p *LocalProvider) GetLoginURL(state string) string {
	return ""
}

// HandleCallback returns nil (not OAuth)
func (p *LocalProvider) HandleCallback(ctx context.Context, code, state string) (*model.User, error) {
	return nil, nil
}

// SupportsOAuth returns false
func (p *LocalProvider) SupportsOAuth() bool {
	return false
}

// GetBootstrapUser returns the bootstrap admin user
func (p *LocalProvider) GetBootstrapUser() *model.User {
	return p.bootstrapUser
}

// HashPassword creates a bcrypt hash of the password
func (p *LocalProvider) HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), p.bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks if the password matches the hash
// Returns true if the password is correct, false otherwise
// Parameters:
// - hash: the bcrypt hash of the password
// - password: the plaintext password to verify
// Returns:
// - bool: true if the password is valid, false otherwise
// Example usage:
//
//	provider := NewLocalProvider(nil, "", "")
//	hash, _ := provider.HashPassword("SecurePassword123")
//	isValid := provider.VerifyPassword(hash, "SecurePassword123")
//	fmt.Println("Password valid:", isValid) // Output: "Password valid: true"
func (p *LocalProvider) VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
