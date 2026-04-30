package cmd

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

func TestRunAccountLoginDelegatesToAppService(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}))
	prompter := &stubAccountLoginPrompter{selectedAccountID: "work-id"}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountLogin(cmd, accountLoginDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountLoginPrompter { return prompter },
		loginAccount: func(_ context.Context, input app.LoginAccountInput) (calendar.Account, error) {
			called = true
			if input.AccountID != "work-id" {
				t.Fatalf("input.AccountID = %q, want work-id", input.AccountID)
			}
			return calendar.Account{Name: "Work"}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v", err)
	}
	if !called {
		t.Fatal("expected loginAccount to be called")
	}
	if !strings.Contains(stdout.String(), `Logged in to account "Work".`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunAccountLoginReturnsNilOnAbort(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}))
	err := runAccountLogin(newTestCommand(), accountLoginDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountLoginPrompter {
			return &stubAccountLoginPrompter{selectionErr: huh.ErrUserAborted}
		},
		loginAccount: func(context.Context, app.LoginAccountInput) (calendar.Account, error) {
			t.Fatal("loginAccount should not be called")
			return calendar.Account{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v, want nil", err)
	}
}

type stubAccountLoginPrompter struct {
	selectedAccountID string
	selectionErr      error
}

func (s *stubAccountLoginPrompter) PromptAccountSelection(context.Context, *appconfig.Config) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}
