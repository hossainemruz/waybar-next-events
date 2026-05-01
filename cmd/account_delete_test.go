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

func TestRunAccountDeleteDelegatesToAppService(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}))
	prompter := &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmed: true}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountDelete(cmd, accountDeleteDependencies{
		newLoader:   func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter { return prompter },
		deleteAccount: func(context.Context, app.DeleteAccountInput) (calendar.Account, error) {
			called = true
			return calendar.Account{Name: "Work"}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v", err)
	}
	if !called {
		t.Fatal("expected deleteAccount to be called")
	}
	if !strings.Contains(stdout.String(), `Deleted account "Work".`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunAccountDeleteReturnsNilOnAbort(t *testing.T) {
	loader := appconfig.NewLoaderWithPath(writeGenericConfig(t, []appconfig.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}))
	err := runAccountDelete(newTestCommand(), accountDeleteDependencies{
		newLoader: func() *appconfig.Loader { return loader },
		newPrompter: func(*cobra.Command) accountDeletePrompter {
			return &stubAccountDeletePrompter{selectionErr: huh.ErrUserAborted}
		},
		deleteAccount: func(context.Context, app.DeleteAccountInput) (calendar.Account, error) {
			t.Fatal("deleteAccount should not be called")
			return calendar.Account{}, nil
		},
	})
	if err != nil {
		t.Fatalf("runAccountDelete() error = %v, want nil", err)
	}
}

type stubAccountDeletePrompter struct {
	selectedAccountID    string
	selectionErr         error
	confirmed            bool
	confirmErr           error
	confirmedAccountName string
}

func (s *stubAccountDeletePrompter) PromptAccountSelection(context.Context, *appconfig.Config) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}

func (s *stubAccountDeletePrompter) PromptDeleteConfirmation(_ context.Context, accountName string) (bool, error) {
	s.confirmedAccountName = accountName
	if s.confirmErr != nil {
		return false, s.confirmErr
	}
	return s.confirmed, nil
}
