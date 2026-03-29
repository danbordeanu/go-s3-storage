package auth

import (
	"fmt"
	"sync"
)

// Credential represents an S3 credential pair
type Credential struct {
	AccessKeyID     string
	SecretAccessKey string
	Active          bool
}

// CredentialStore is the interface for credential storage
type CredentialStore interface {
	// GetCredential retrieves a credential by access key ID
	GetCredential(accessKeyID string) (*Credential, error)
}

// MemoryStore is an in-memory implementation of CredentialStore
type MemoryStore struct {
	mu          sync.RWMutex
	credentials map[string]*Credential
	userLookup  map[string]string // maps accessKeyID → userID
}

// NewMemoryStore creates a new MemoryStore with the provided credentials
// If accessKeyID and secretAccessKey are non-empty, they will be added to the store
// This allows for a default credential to be set up at initialization
// If both accessKeyID and secretAccessKey are empty, the store will be initialized without any credentials
// This is useful for testing or when credentials will be added dynamically later
// Parameters:
//   - accessKeyID: The access key ID to add to the store (optional)
//   - secretAccessKey: The secret access key to add to the store (optional)
//
// Returns:
//   - A pointer to the initialized MemoryStore
//
// Example usage:
//
//	store := NewMemoryStore("myAccessKeyID", "mySecretAccessKey")
func NewMemoryStore(accessKeyID, secretAccessKey string) *MemoryStore {
	store := &MemoryStore{
		credentials: make(map[string]*Credential),
		userLookup:  make(map[string]string),
	}
	if accessKeyID != "" && secretAccessKey != "" {
		store.credentials[accessKeyID] = &Credential{
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
			Active:          true,
		}
	}
	return store
}

// GetCredential retrieves a credential by access key ID
// Returns an error if the access key is not found or is inactive
// Parameters:
//   - accessKeyID: The access key ID to look up
//
// Returns:
//   - A pointer to the Credential if found and active
//   - An error if the access key is not found or is inactive
//
// Example usage:
//
//	cred, err := store.GetCredential("myAccessKeyID")
//	if err != nil {
//	    // handle error
//	}
func (m *MemoryStore) GetCredential(accessKeyID string) (*Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cred, ok := m.credentials[accessKeyID]
	if !ok {
		return nil, fmt.Errorf("access key not found: %s", accessKeyID)
	}

	if !cred.Active {
		return nil, fmt.Errorf("access key is inactive: %s", accessKeyID)
	}

	return cred, nil
}

// AddCredential adds a credential to the store
// This can be used to add new credentials dynamically after the store has been initialized
// Parameters:
//   - cred: A pointer to the Credential to add
//
// Returns:
//   - None
//
// Example usage:
//
//	store.AddCredential(&Credential{
//	    AccessKeyID:     "newAccessKeyID",
func (m *MemoryStore) AddCredential(cred *Credential) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.credentials[cred.AccessKeyID] = cred
}

// RemoveCredential removes a credential from the store
// This can be used to deactivate or delete credentials dynamically after the store has been initialized
// Parameters:
//   - accessKeyID: The access key ID of the credential to remove
//
// Returns:
//   - None
//
// Example usage:
//
//	store.RemoveCredential("myAccessKeyID")
func (m *MemoryStore) RemoveCredential(accessKeyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.credentials, accessKeyID)
	delete(m.userLookup, accessKeyID)
}

// AddUserCredential adds a user's credential to the store
// This method associates a user's credential with their user ID, allowing for user-based credential management
// Parameters:
//   - userID: The ID of the user to associate with the credential
//   - accessKeyID: The access key ID of the credential to add
//   - secretAccessKey: The secret access key of the credential to add
//
// Returns:
//   - None
//
// Example usage:
//
//	store.AddUserCredential("user123", "userAccessKeyID", "userSecretAccessKey")
func (m *MemoryStore) AddUserCredential(userID, accessKeyID, secretAccessKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.credentials[accessKeyID] = &Credential{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Active:          true,
	}
	m.userLookup[accessKeyID] = userID
}

// RemoveUserCredential removes a user's credential from the store
// This method removes the association between a user's credential and their user ID, effectively deactivating the credential for that user
// Parameters:
//   - accessKeyID: The access key ID of the credential to remove
//
// Returns:
//   - None
//
// Example usage:
//
//	store.RemoveUserCredential("userAccessKeyID")
func (m *MemoryStore) RemoveUserCredential(accessKeyID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.credentials, accessKeyID)
	delete(m.userLookup, accessKeyID)
}

// GetUserIDByAccessKey retrieves the user ID associated with an access key
// This method allows for looking up which user is associated with a given access key, which can be useful for auditing and access control purposes
// Parameters:
//   - accessKeyID: The access key ID to look up
//
// Returns:
//   - The user ID associated with the access key if found
//   - An error if no user is associated with the access key
//
// Example usage:
//
//	userID, err := store.GetUserIDByAccessKey("userAccessKeyID")
//	if err != nil {
//	    // handle error
//	}
func (m *MemoryStore) GetUserIDByAccessKey(accessKeyID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userID, ok := m.userLookup[accessKeyID]
	if !ok {
		return "", fmt.Errorf("no user associated with access key: %s", accessKeyID)
	}

	return userID, nil
}
