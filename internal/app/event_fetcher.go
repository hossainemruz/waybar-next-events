package app

import (
	"context"
	"fmt"
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

	// Fetch is fail-fast: one errored account stops processing for all accounts.
	for _, account := range cfg.Accounts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		service, err := f.services.Service(account.Service)
		if err != nil {
			return nil, err
		}

		provider, err := service.Provider(ctx, account, f.secretStore)
		if err != nil {
			return nil, err
		}

		client, err := authenticator.HTTPClient(ctx, provider)
		if err != nil {
			return nil, err
		}

		accountEvents, err := service.FetchEvents(ctx, account, query, client)
		if err != nil {
			return nil, err
		}

		events = append(events, accountEvents...)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Start.Before(events[j].Start)
	})

	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}
