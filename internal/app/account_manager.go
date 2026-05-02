package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/hossainemruz/waybar-next-events/internal/auth"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

// AccountManager owns account workflows.
type AccountManager struct {
	loader           ConfigLoader
	services         *Registry
	secretStore      secrets.Store
	tokenStore       tokenstore.TokenStore
	newAccountID     func() (string, error)
	newAuthenticator func(store tokenstore.TokenStore) Authenticator
}

// ListAccounts returns all configured accounts.
func (m *AccountManager) ListAccounts() ([]calendar.Account, error) {
	cfg, err := m.loader.LoadOrEmpty()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg == nil {
		return []calendar.Account{}, nil
	}
	return cfg.Accounts, nil
}

// NewAccountManager creates an AccountManager.
func NewAccountManager(loader ConfigLoader, services *Registry, secretStore secrets.Store, tokenStore tokenstore.TokenStore) *AccountManager {
	return &AccountManager{
		loader:       loader,
		services:     services,
		secretStore:  secretStore,
		tokenStore:   tokenStore,
		newAccountID: config.NewAccountID,
		newAuthenticator: func(store tokenstore.TokenStore) Authenticator {
			return auth.NewAuthenticator(store)
		},
	}
}

// AddAccountInput creates, authenticates, and persists a new account.
type AddAccountInput struct {
	Service          calendar.ServiceType
	Name             string
	Settings         map[string]string
	Secrets          map[string]string
	CalendarSelector CalendarSelector
}

func (m *AccountManager) AddAccount(ctx context.Context, input AddAccountInput) (calendar.Account, error) {
	service, err := m.services.Service(input.Service)
	if err != nil {
		return calendar.Account{}, err
	}

	cfg, err := m.loader.LoadOrEmpty()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("load config: %w", err)
	}

	snapshot, err := m.loader.Snapshot()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("snapshot config: %w", err)
	}

	accountID, err := m.newAccountID()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("generate account id: %w", err)
	}

	account := calendar.Account{
		ID:        accountID,
		Service:   input.Service,
		Name:      strings.TrimSpace(input.Name),
		Settings:  cloneStringMap(input.Settings),
		Calendars: []calendar.CalendarRef{},
	}

	stagedSecrets := secrets.NewStagedStore()
	if err := stageSecrets(ctx, stagedSecrets, account.ID, service.AccountFields(), input.Secrets); err != nil {
		return calendar.Account{}, err
	}

	stagedTokens := tokenstore.NewStagedTokenStore()
	authenticator := m.newAuthenticator(stagedTokens)

	selectedCalendars, err := m.discoverAndSelectCalendars(ctx, service, account, stagedSecrets, authenticator, input.CalendarSelector)
	if err != nil {
		return calendar.Account{}, err
	}
	account.Calendars = selectedCalendars

	cfg.Accounts = append(cfg.Accounts, account)
	if err := m.loader.Save(cfg); err != nil {
		return calendar.Account{}, fmt.Errorf("save config: %w", err)
	}

	if err := m.commitAccountState(ctx, snapshot, account, service.AccountFields(), stagedSecrets, stagedTokens); err != nil {
		return calendar.Account{}, err
	}

	return account, nil
}

// UpdateAccountInput updates, re-authenticates if needed, and persists an account.
type UpdateAccountInput struct {
	AccountID        string
	Name             string
	Settings         map[string]string
	Secrets          map[string]string
	CalendarSelector CalendarSelector
}

func (m *AccountManager) UpdateAccount(ctx context.Context, input UpdateAccountInput) (calendar.Account, error) {
	cfg, err := m.loader.LoadOrEmpty()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("load config: %w", err)
	}

	original, err := findAccount(cfg, input.AccountID)
	if err != nil {
		return calendar.Account{}, err
	}

	service, err := m.services.Service(original.Service)
	if err != nil {
		return calendar.Account{}, err
	}

	snapshot, err := m.loader.Snapshot()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("snapshot config: %w", err)
	}

	secretKeys := secretFieldKeys(service.AccountFields())
	originalSecrets, err := snapshotAccountSecrets(ctx, m.secretStore, original.ID, secretKeys)
	if err != nil {
		return calendar.Account{}, err
	}

	updated := calendar.Account{
		ID:        original.ID,
		Service:   original.Service,
		Name:      strings.TrimSpace(input.Name),
		Settings:  cloneStringMap(input.Settings),
		Calendars: cloneCalendarRefs(original.Calendars),
	}
	mergedSecrets := mergeSecrets(originalSecrets, input.Secrets, service.AccountFields())

	stagedSecrets := secrets.NewStagedStore()
	if err := stageSecrets(ctx, stagedSecrets, updated.ID, service.AccountFields(), mergedSecrets); err != nil {
		return calendar.Account{}, err
	}

	stagedTokens := tokenstore.NewStagedTokenStore()
	if !credentialsChanged(original, originalSecrets, updated, mergedSecrets, secretKeys) {
		if err := seedStagedTokenStore(ctx, stagedTokens, m.tokenStore, original); err != nil {
			return calendar.Account{}, err
		}
	}

	authenticator := m.newAuthenticator(stagedTokens)
	selectedCalendars, err := m.discoverAndSelectCalendars(ctx, service, updated, stagedSecrets, authenticator, input.CalendarSelector)
	if err != nil {
		return calendar.Account{}, err
	}
	updated.Calendars = selectedCalendars

	for i := range cfg.Accounts {
		if cfg.Accounts[i].ID == updated.ID {
			cfg.Accounts[i] = updated
			break
		}
	}

	if err := m.loader.Save(cfg); err != nil {
		return calendar.Account{}, fmt.Errorf("save config: %w", err)
	}

	if err := m.commitAccountState(ctx, snapshot, updated, service.AccountFields(), stagedSecrets, stagedTokens); err != nil {
		return calendar.Account{}, err
	}

	return updated, nil
}

// DeleteAccountInput removes an account, its secrets, and its token.
type DeleteAccountInput struct {
	AccountID string
}

func (m *AccountManager) DeleteAccount(ctx context.Context, input DeleteAccountInput) (calendar.Account, error) {
	cfg, err := m.loader.LoadOrEmpty()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("load config: %w", err)
	}

	account, err := findAccount(cfg, input.AccountID)
	if err != nil {
		return calendar.Account{}, err
	}

	service, err := m.services.Service(account.Service)
	if err != nil {
		return calendar.Account{}, err
	}

	snapshot, err := m.loader.Snapshot()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("snapshot config: %w", err)
	}

	secretKeys := secretFieldKeys(service.AccountFields())
	secretSnapshot, err := snapshotAccountSecrets(ctx, m.secretStore, account.ID, secretKeys)
	if err != nil {
		return calendar.Account{}, err
	}

	stagedSecrets := secrets.NewStagedStore()
	if err := stageSecretDeletion(ctx, stagedSecrets, account.ID, secretKeys); err != nil {
		return calendar.Account{}, err
	}

	stagedTokens := tokenstore.NewStagedTokenStore()
	if err := stagedTokens.Clear(ctx, tokenKey(*account)); err != nil {
		return calendar.Account{}, fmt.Errorf("stage token deletion: %w", err)
	}

	cfg.Accounts = deleteAccountByID(cfg.Accounts, account.ID)
	if err := m.loader.Save(cfg); err != nil {
		return calendar.Account{}, fmt.Errorf("save config: %w", err)
	}

	if err := stagedSecrets.Commit(ctx, m.secretStore); err != nil {
		if restoreErr := m.loader.RestoreSnapshot(snapshot); restoreErr != nil {
			return calendar.Account{}, errors.Join(fmt.Errorf("persist secrets: %w", err), fmt.Errorf("restore config: %w", restoreErr))
		}
		return calendar.Account{}, fmt.Errorf("persist secrets: %w", err)
	}

	if err := stagedTokens.Commit(ctx, m.tokenStore); err != nil {
		secretRollbackErr := restoreAccountSecrets(context.Background(), m.secretStore, account.ID, secretSnapshot)
		configRollbackErr := m.loader.RestoreSnapshot(snapshot)
		return calendar.Account{}, joinCommitRollbackError("persist token removal", err, configRollbackErr, secretRollbackErr)
	}

	return *account, nil
}

// LoginAccountInput performs a forced re-authentication and only commits the new token on success.
type LoginAccountInput struct {
	AccountID string
}

func (m *AccountManager) LoginAccount(ctx context.Context, input LoginAccountInput) (calendar.Account, error) {
	cfg, err := m.loader.LoadOrEmpty()
	if err != nil {
		return calendar.Account{}, fmt.Errorf("load config: %w", err)
	}

	account, err := findAccount(cfg, input.AccountID)
	if err != nil {
		return calendar.Account{}, err
	}

	service, err := m.services.Service(account.Service)
	if err != nil {
		return calendar.Account{}, err
	}

	provider, err := service.Provider(ctx, *account, m.secretStore)
	if err != nil {
		return calendar.Account{}, err
	}

	stagedTokens := tokenstore.NewStagedTokenStore()
	authenticator := m.newAuthenticator(stagedTokens)
	if _, err := authenticator.ForceAuthenticate(ctx, provider); err != nil {
		return calendar.Account{}, err
	}

	// LoginAccount only replaces the persisted OAuth token. It does not mutate
	// config, so there is no config state to snapshot and roll back here.
	if err := stagedTokens.Commit(ctx, m.tokenStore); err != nil {
		return calendar.Account{}, fmt.Errorf("persist OAuth token: %w", err)
	}

	return *account, nil
}

func (m *AccountManager) discoverAndSelectCalendars(ctx context.Context, service Service, account calendar.Account, secretStore secrets.Store, authenticator Authenticator, selector CalendarSelector) ([]calendar.CalendarRef, error) {
	provider, err := service.Provider(ctx, account, secretStore)
	if err != nil {
		return nil, err
	}

	client, err := authenticator.HTTPClient(ctx, provider)
	if err != nil {
		return nil, err
	}

	discovered, err := service.DiscoverCalendars(ctx, account, client)
	if err != nil {
		return nil, err
	}

	if len(discovered) == 0 {
		if selector != nil {
			if err := selector.ConfirmEmptyCalendars(ctx, account); err != nil {
				return nil, err
			}
		}
		return []calendar.CalendarRef{}, nil
	}

	if selector == nil {
		return nil, ErrCalendarSelectionRequired
	}

	selected, err := selector.SelectCalendars(ctx, account, discovered)
	if err != nil {
		return nil, err
	}

	return cloneCalendarRefs(selected), nil
}

func (m *AccountManager) commitAccountState(ctx context.Context, configSnapshot config.Snapshot, account calendar.Account, fields []calendar.AccountField, stagedSecrets *secrets.StagedStore, stagedTokens *tokenstore.StagedTokenStore) error {
	secretKeys := secretFieldKeys(fields)
	secretSnapshot, err := snapshotAccountSecrets(ctx, m.secretStore, account.ID, secretKeys)
	if err != nil {
		if restoreErr := m.loader.RestoreSnapshot(configSnapshot); restoreErr != nil {
			return errors.Join(fmt.Errorf("snapshot account secrets: %w", err), fmt.Errorf("restore config: %w", restoreErr))
		}
		return fmt.Errorf("snapshot account secrets: %w", err)
	}

	if err := stagedSecrets.Commit(ctx, m.secretStore); err != nil {
		if restoreErr := m.loader.RestoreSnapshot(configSnapshot); restoreErr != nil {
			return errors.Join(fmt.Errorf("persist account secrets: %w", err), fmt.Errorf("restore config: %w", restoreErr))
		}
		return fmt.Errorf("persist account secrets: %w", err)
	}

	if err := stagedTokens.Commit(ctx, m.tokenStore); err != nil {
		secretRollbackErr := restoreAccountSecrets(context.Background(), m.secretStore, account.ID, secretSnapshot)
		configRollbackErr := m.loader.RestoreSnapshot(configSnapshot)
		return joinCommitRollbackError("persist OAuth token", err, configRollbackErr, secretRollbackErr)
	}

	return nil
}

func tokenKey(account calendar.Account) string {
	return tokenstore.TokenKey(string(account.Service), account.ID)
}

func findAccount(cfg *config.Config, accountID string) (*calendar.Account, error) {
	if cfg == nil || len(cfg.Accounts) == 0 {
		return nil, config.ErrNoAccounts
	}

	account := cfg.FindAccountByID(accountID)
	if account == nil {
		return nil, fmt.Errorf("%w: %q", config.ErrAccountNotFound, accountID)
	}

	return account, nil
}

func seedStagedTokenStore(ctx context.Context, stagedStore *tokenstore.StagedTokenStore, backingStore tokenstore.TokenStore, account *calendar.Account) error {
	token, found, err := backingStore.Get(ctx, tokenKey(*account))
	if err != nil {
		return fmt.Errorf("load existing OAuth token for account %q: %w", account.Name, err)
	}
	if !found {
		return nil
	}

	if err := stagedStore.Set(ctx, tokenKey(*account), token); err != nil {
		return fmt.Errorf("stage existing OAuth token for account %q: %w", account.Name, err)
	}

	return nil
}

func stageSecrets(ctx context.Context, staged *secrets.StagedStore, accountID string, fields []calendar.AccountField, values map[string]string) error {
	for _, field := range fields {
		if !field.Secret {
			continue
		}

		value, ok := values[field.Key]
		if !ok {
			if !field.Required {
				continue
			}
			return fmt.Errorf("missing account secret %q", field.Key)
		}
		if err := staged.Set(ctx, accountID, field.Key, strings.TrimSpace(value)); err != nil {
			return fmt.Errorf("stage account secret %q: %w", field.Key, err)
		}
	}

	return nil
}

func stageSecretDeletion(ctx context.Context, staged *secrets.StagedStore, accountID string, keys []string) error {
	for _, key := range keys {
		if err := staged.Delete(ctx, accountID, key); err != nil {
			return fmt.Errorf("stage account secret deletion %q: %w", key, err)
		}
	}

	return nil
}

type secretSnapshot struct {
	value string
	found bool
}

func snapshotAccountSecrets(ctx context.Context, store secrets.Store, accountID string, keys []string) (map[string]secretSnapshot, error) {
	snapshot := make(map[string]secretSnapshot, len(keys))
	for _, key := range keys {
		value, err := store.Get(ctx, accountID, key)
		if err != nil {
			if errors.Is(err, secrets.ErrSecretNotFound) {
				snapshot[key] = secretSnapshot{found: false}
				continue
			}
			return nil, fmt.Errorf("snapshot secret %q: %w", key, err)
		}

		snapshot[key] = secretSnapshot{value: value, found: true}
	}

	return snapshot, nil
}

func restoreAccountSecrets(ctx context.Context, store secrets.Store, accountID string, snapshot map[string]secretSnapshot) error {
	for key, stored := range snapshot {
		if stored.found {
			if err := store.Set(ctx, accountID, key, stored.value); err != nil {
				return fmt.Errorf("restore secret %q: %w", key, err)
			}
			continue
		}

		if err := store.Delete(ctx, accountID, key); err != nil {
			return fmt.Errorf("restore secret deletion for %q: %w", key, err)
		}
	}

	return nil
}

func credentialsChanged(original *calendar.Account, originalSecrets map[string]secretSnapshot, updated calendar.Account, newSecrets map[string]string, secretKeys []string) bool {
	if !maps.Equal(original.Settings, updated.Settings) {
		return true
	}

	for _, key := range secretKeys {
		trimmed := strings.TrimSpace(newSecrets[key])
		stored, ok := originalSecrets[key]
		if !ok {
			return true
		}
		if !stored.found {
			if trimmed != "" {
				return true
			}
			continue
		}
		if stored.value != trimmed {
			return true
		}
	}

	return false
}

func mergeSecrets(existing map[string]secretSnapshot, updated map[string]string, fields []calendar.AccountField) map[string]string {
	merged := make(map[string]string, len(fields))
	for _, field := range fields {
		if !field.Secret {
			continue
		}

		if value, ok := updated[field.Key]; ok {
			merged[field.Key] = strings.TrimSpace(value)
			continue
		}

		if stored, ok := existing[field.Key]; ok && stored.found {
			merged[field.Key] = stored.value
			continue
		}

		if !field.Required {
			merged[field.Key] = ""
		}
	}

	return merged
}

func secretFieldKeys(fields []calendar.AccountField) []string {
	keys := make([]string, 0)
	for _, field := range fields {
		if field.Secret {
			keys = append(keys, field.Key)
		}
	}
	return keys
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = strings.TrimSpace(value)
	}
	return cloned
}

func cloneCalendarRefs(values []calendar.CalendarRef) []calendar.CalendarRef {
	if len(values) == 0 {
		return []calendar.CalendarRef{}
	}

	cloned := make([]calendar.CalendarRef, len(values))
	copy(cloned, values)
	return cloned
}

func deleteAccountByID(accounts []calendar.Account, accountID string) []calendar.Account {
	updated := make([]calendar.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.ID == accountID {
			continue
		}
		updated = append(updated, account)
	}
	return updated
}

func joinCommitRollbackError(action string, commitErr, configRollbackErr, secretRollbackErr error) error {
	wrapped := fmt.Errorf("%s: %w", action, commitErr)
	if configRollbackErr != nil && secretRollbackErr != nil {
		return errors.Join(wrapped, fmt.Errorf("restore config: %w", configRollbackErr), fmt.Errorf("restore secret: %w", secretRollbackErr))
	}
	if configRollbackErr != nil {
		return errors.Join(wrapped, fmt.Errorf("restore config: %w", configRollbackErr))
	}
	if secretRollbackErr != nil {
		return errors.Join(wrapped, fmt.Errorf("restore secret: %w", secretRollbackErr))
	}
	return wrapped
}
