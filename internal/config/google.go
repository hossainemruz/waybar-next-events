package config

// GoogleCalendar holds Google Calendar provider configuration,
// including display metadata and a list of accounts.
type GoogleCalendar struct {
	Name     string          `json:"name"`
	Accounts []GoogleAccount `json:"accounts"`
}

// GoogleAccount represents a single Google account with its own OAuth2 credentials
// and an optional list of calendars to fetch events from.
type GoogleAccount struct {
	Name         string     `json:"name"`
	ClientID     string     `json:"clientId"`
	ClientSecret string     `json:"clientSecret"`
	Calendars    []Calendar `json:"calendars"`
}

// Calendar represents a single calendar within a Google account.
type Calendar struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// CalendarIDs returns the calendar IDs for this account.
// If no calendars are configured, it defaults to ["primary"].
func (a *GoogleAccount) CalendarIDs() []string {
	if len(a.Calendars) == 0 {
		return []string{"primary"}
	}
	ids := make([]string, len(a.Calendars))
	for i, cal := range a.Calendars {
		ids[i] = cal.ID
	}
	return ids
}

// FindAccountByName returns a pointer to the account with the given name,
// or nil if no account with that name exists.
func (g *GoogleCalendar) FindAccountByName(name string) *GoogleAccount {
	for i := range g.Accounts {
		if g.Accounts[i].Name == name {
			return &g.Accounts[i]
		}
	}
	return nil
}

// AccountNames returns a slice of all account names in the configuration.
func (g *GoogleCalendar) AccountNames() []string {
	names := make([]string, len(g.Accounts))
	for i, acc := range g.Accounts {
		names[i] = acc.Name
	}
	return names
}

// EnsureGoogleInitialized ensures the Config's Google field is initialized
// with a valid GoogleCalendar, creating one if nil. This is useful for
// first-run account creation when no config file exists yet.
func (c *Config) EnsureGoogleInitialized() {
	if c.Google == nil {
		c.Google = &GoogleCalendar{
			Name:     "Google Calendar",
			Accounts: []GoogleAccount{},
		}
	}
}
