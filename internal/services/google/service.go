// Package google contains the Google Calendar service adapter.
package google

import (
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

const (
	clientIDKey     = "client_id"
	clientSecretKey = "client_secret"
)

// Service implements the Google calendar service adapter.
type Service struct{}

// NewService creates a Google service adapter.
func NewService() *Service {
	return &Service{}
}

func (s *Service) Type() calendar.ServiceType {
	return calendar.ServiceTypeGoogle
}

func (s *Service) DisplayName() string {
	return "Google"
}

func (s *Service) AccountFields() []calendar.AccountField {
	return []calendar.AccountField{
		{Key: clientIDKey, Label: "OAuth Client ID", Required: true},
		{Key: clientSecretKey, Label: "OAuth Client Secret", Required: true, Secret: true},
	}
}
