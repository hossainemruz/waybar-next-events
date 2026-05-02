package commands

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestRunAccountLoginDelegatesToAppService(t *testing.T) {
	prompter := &stubAccountLoginPrompter{selectedAccountID: "work-id"}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountLogin(cmd, accountLoginDeps{
		manager: &fakeAccountLoginManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}},
			loginAccountFunc: func(_ context.Context, input app.LoginAccountInput) (calendar.Account, error) {
				called = true
				if input.AccountID != "work-id" {
					t.Fatalf("input.AccountID = %q, want work-id", input.AccountID)
				}
				return calendar.Account{Name: "Work"}, nil
			},
		},
		prompter: prompter,
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
	err := runAccountLogin(newTestCommand(), accountLoginDeps{
		manager: &fakeAccountLoginManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}},
		},
		prompter: &stubAccountLoginPrompter{selectionErr: huh.ErrUserAborted},
	})
	if err != nil {
		t.Fatalf("runAccountLogin() error = %v, want nil", err)
	}
}

type stubAccountLoginPrompter struct {
	selectedAccountID string
	selectionErr      error
}

func (s *stubAccountLoginPrompter) SelectAccount(context.Context, []calendar.Account, string) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}

type fakeAccountLoginManager struct {
	fakeBaseManager
	loginAccountFunc func(context.Context, app.LoginAccountInput) (calendar.Account, error)
}

func (f *fakeAccountLoginManager) LoginAccount(ctx context.Context, input app.LoginAccountInput) (calendar.Account, error) {
	if f.loginAccountFunc != nil {
		return f.loginAccountFunc(ctx, input)
	}
	return calendar.Account{}, nil
}
