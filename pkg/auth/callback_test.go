package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCallbackServer(t *testing.T) {
	t.Run("ValidCallback", func(t *testing.T) {
		expectedState := "test-state-123"
		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()

		// Give server time to start
		time.Sleep(10 * time.Millisecond)

		// Simulate callback
		resp, err := http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Check result
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := server.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}

		if result.Code != "auth-code" {
			t.Errorf("Code = %s, want auth-code", result.Code)
		}
		if result.State != expectedState {
			t.Errorf("State = %s, want %s", result.State, expectedState)
		}
		if result.Error != "" {
			t.Errorf("Error = %s, want empty", result.Error)
		}
	})

	t.Run("StateMismatch", func(t *testing.T) {
		expectedState := "correct-state"
		wrongState := "wrong-state"

		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()
		time.Sleep(10 * time.Millisecond)

		resp, err := http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + wrongState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := server.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}

		if result.Error != "state_mismatch" {
			t.Errorf("Error = %s, want state_mismatch", result.Error)
		}
	})

	t.Run("MissingCode", func(t *testing.T) {
		expectedState := "test-state"

		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()
		time.Sleep(10 * time.Millisecond)

		resp, err := http.Get("http://127.0.0.1:18751/callback?state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := server.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}

		if result.Error != "missing_code" {
			t.Errorf("Error = %s, want missing_code", result.Error)
		}
	})

	t.Run("ProviderError", func(t *testing.T) {
		expectedState := "test-state"

		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()
		time.Sleep(10 * time.Millisecond)

		resp, err := http.Get("http://127.0.0.1:18751/callback?error=access_denied&error_description=user+denied")
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := server.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}

		if result.Error != "access_denied" {
			t.Errorf("Error = %s, want access_denied", result.Error)
		}
		if result.ErrorDesc != "user denied" {
			t.Errorf("ErrorDesc = %s, want 'user denied'", result.ErrorDesc)
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		server, err := NewCallbackServer("state")
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()
		time.Sleep(10 * time.Millisecond)

		resp, err := http.Post("http://127.0.0.1:18751/callback", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("HTTP POST error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
		}
	})

	t.Run("WrongPath", func(t *testing.T) {
		server, err := NewCallbackServer("state")
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()
		time.Sleep(10 * time.Millisecond)

		resp, err := http.Get("http://127.0.0.1:18751/wrong-path")
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		server, err := NewCallbackServer("state")
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		server.Start()

		// Don't send callback - just wait for timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = server.Wait(ctx)
		if err == nil {
			t.Error("Wait() expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("Wait() error = %v, want context deadline exceeded", err)
		}
	})
}
