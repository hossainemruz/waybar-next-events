package commands

import (
	"context"
	"io"
	"net/http"

	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	"github.com/spf13/cobra"
)

func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd
}

func newTestRegistry() *calendar.Registry {
	registry := calendar.NewRegistry()
	_ = registry.Register(&stubService{})
	return registry
}

type fakeBaseManager struct {
	listAccounts []calendar.Account
	listErr      error
}

func (f *fakeBaseManager) ListAccounts() ([]calendar.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listAccounts, nil
}

type stubService struct{}

func (s *stubService) Type() calendar.ServiceType { return calendar.ServiceTypeGoogle }
func (s *stubService) DisplayName() string        { return "Google" }
func (s *stubService) AccountFields() []calendar.AccountField {
	return []calendar.AccountField{
		{Key: "client_id", Label: "OAuth Client ID", Required: true},
		{Key: "client_secret", Label: "OAuth Client Secret", Required: true, Secret: true},
	}
}
func (s *stubService) DiscoverCalendars(context.Context, calendar.Account, *http.Client) ([]calendar.Calendar, error) {
	return nil, nil
}
func (s *stubService) FetchEvents(context.Context, calendar.Account, calendar.EventQuery, *http.Client) ([]calendar.Event, error) {
	return nil, nil
}
