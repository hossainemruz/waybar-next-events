package secrets

import (
	"context"
	"errors"
	"fmt"
)

// ErrSecretNotFound indicates that a requested secret does not exist.
var ErrSecretNotFound = errors.New("secret not found")

// Store persists named secret values for an account.
type Store interface {
	Get(ctx context.Context, accountID, key string) (string, error)
	Set(ctx context.Context, accountID, key, value string) error
	Delete(ctx context.Context, accountID, key string) error
}

func validateSecretRef(accountID, key string) error {
	if accountID == "" {
		return fmt.Errorf("account ID cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("secret key cannot be empty")
	}

	return nil
}

func storageKey(accountID, key string) string {
	return accountID + "/" + key
}
