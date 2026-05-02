package commands

import (
	"context"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/hossainemruz/waybar-next-events/internal/app"
	"github.com/hossainemruz/waybar-next-events/internal/calendar"
)

func TestRunAccountDeleteDelegatesToAppService(t *testing.T) {
	prompter := &stubAccountDeletePrompter{selectedAccountID: "work-id", confirmed: true}
	stdout := &strings.Builder{}
	cmd := newTestCommand()
	cmd.SetOut(stdout)

	called := false
	err := runAccountDelete(cmd, accountDeleteDeps{
		manager: &fakeAccountDeleteManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}},
			deleteAccountFunc: func(context.Context, app.DeleteAccountInput) (calendar.Account, error) {
				called = true
				return calendar.Account{Name: "Work"}, nil
			},
		},
		prompter: prompter,
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
	err := runAccountDelete(newTestCommand(), accountDeleteDeps{
		manager: &fakeAccountDeleteManager{
			fakeBaseManager: fakeBaseManager{listAccounts: []calendar.Account{{ID: "work-id", Service: calendar.ServiceTypeGoogle, Name: "Work"}}},
		},
		prompter: &stubAccountDeletePrompter{selectionErr: huh.ErrUserAborted},
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

func (s *stubAccountDeletePrompter) SelectAccount(context.Context, []calendar.Account, string) (string, error) {
	if s.selectionErr != nil {
		return "", s.selectionErr
	}
	return s.selectedAccountID, nil
}

func (s *stubAccountDeletePrompter) ConfirmDelete(_ context.Context, accountName string) (bool, error) {
	s.confirmedAccountName = accountName
	if s.confirmErr != nil {
		return false, s.confirmErr
	}
	return s.confirmed, nil
}

type fakeAccountDeleteManager struct {
	fakeBaseManager
	deleteAccountFunc func(context.Context, app.DeleteAccountInput) (calendar.Account, error)
}

func (f *fakeAccountDeleteManager) DeleteAccount(ctx context.Context, input app.DeleteAccountInput) (calendar.Account, error) {
	if f.deleteAccountFunc != nil {
		return f.deleteAccountFunc(ctx, input)
	}
	return calendar.Account{}, nil
}
