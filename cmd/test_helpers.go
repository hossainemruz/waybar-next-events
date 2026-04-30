package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	appconfig "github.com/hossainemruz/waybar-next-events/internal/config"
	"github.com/spf13/cobra"
)

func writeGenericConfig(t *testing.T, accounts []appconfig.Account) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &appconfig.Config{Accounts: accounts}
	cfg.Normalize()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, appconfig.ConfigFilePermission); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	return path
}

func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(ioDiscard{})
	cmd.SetErr(ioDiscard{})
	return cmd
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
