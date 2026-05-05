package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/hossainemruz/waybar-next-events/internal/auth"
	"github.com/hossainemruz/waybar-next-events/internal/auth/tokenstore"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/hossainemruz/waybar-next-events/internal/secrets"
)

// EventFetcher fetches events across accounts.
type EventFetcher struct {
	loader           ConfigLoader
	services         *Registry
	secretStore      secrets.Store
	newAuthenticator func() Authenticator
}

// NewEventFetcher creates an EventFetcher.
func NewEventFetcher(loader ConfigLoader, services *Registry, secretStore secrets.Store, tokenStore tokenstore.TokenStore) *EventFetcher {
	return &EventFetcher{
		loader:      loader,
		services:    services,
		secretStore: secretStore,
		newAuthenticator: func() Authenticator {
			return auth.NewAuthenticator(tokenStore)
		},
	}
}

// Fetch loads configured accounts, fetches events across them, sorts them, and applies the limit.
func (f *EventFetcher) Fetch(ctx context.Context, query calendar.EventQuery, limit int) ([]calendar.Event, error) {
	cfg, err := f.loader.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Accounts) == 0 {
		return nil, config.ErrNoAccounts
	}

	authenticator := f.newAuthenticator()
	events := make([]calendar.Event, 0)
	var errs []error
	successCount := 0

	for _, account := range cfg.Accounts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		service, err := f.services.Service(account.Service)
		if err != nil {
			slog.Error("failed to resolve service for account", "account", account.Name, "error", err)
			errs = append(errs, fmt.Errorf("account %q: %w", account.Name, err))
			continue
		}

		provider, err := service.Provider(ctx, account, f.secretStore)
		if err != nil {
			slog.Error("failed to create provider for account", "account", account.Name, "error", err)
			errs = append(errs, fmt.Errorf("account %q: %w", account.Name, err))
			continue
		}

		client, err := authenticator.HTTPClient(ctx, provider)
		if err != nil {
			slog.Error("failed to create HTTP client for account", "account", account.Name, "error", err)
			errs = append(errs, fmt.Errorf("account %q: %w", account.Name, err))
			continue
		}

		accountEvents, err := service.FetchEvents(ctx, account, query, client)
		if err != nil {
			slog.Error("failed to fetch events for account", "account", account.Name, "error", err)
			errs = append(errs, fmt.Errorf("account %q: %w", account.Name, err))
			continue
		}

		successCount++
		events = append(events, accountEvents...)
	}

	if successCount == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}
