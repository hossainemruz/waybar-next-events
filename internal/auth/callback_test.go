package auth

import (
	"context"
	"net"
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

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		// Simulate callback
		resp, err := http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

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

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		// A request with wrong state should NOT consume the callback slot.
		// It should receive a 400 response but the flow should remain open.
		resp, err := http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + wrongState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Wrong-state Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		// The flow should still be open, so a valid callback should succeed.
		resp, err = http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Valid callback Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

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

	t.Run("MissingCode", func(t *testing.T) {
		expectedState := "test-state"

		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		resp, err := http.Get("http://127.0.0.1:18751/callback?state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

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

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		// Provider error callbacks should include state validation.
		// A valid state should be accepted.
		resp, err := http.Get("http://127.0.0.1:18751/callback?error=access_denied&error_description=user+denied&state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

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

	t.Run("ProviderErrorWithWrongState", func(t *testing.T) {
		expectedState := "correct-state"
		wrongState := "wrong-state"

		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		// An error callback with wrong state should NOT consume the flow.
		resp, err := http.Get("http://127.0.0.1:18751/callback?error=access_denied&error_description=spoofed&state=" + wrongState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Wrong-state error Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		// The valid callback should still succeed after the spoofed error.
		resp, err = http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Valid callback Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := server.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}

		if result.Code != "auth-code" {
			t.Errorf("Code = %s, want auth-code", result.Code)
		}
		if result.Error != "" {
			t.Errorf("Error = %s, want empty", result.Error)
		}
	})

	t.Run("ProviderErrorWithoutState", func(t *testing.T) {
		expectedState := "test-state"

		server, err := NewCallbackServer(expectedState)
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		// An error callback with NO state should NOT consume the flow.
		resp, err := http.Get("http://127.0.0.1:18751/callback?error=access_denied&error_description=no+state")
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("No-state error Status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		// The valid callback should still succeed.
		resp, err = http.Get("http://127.0.0.1:18751/callback?code=auth-code&state=" + expectedState)
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Valid callback Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		result, err := server.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait() error = %v", err)
		}

		if result.Code != "auth-code" {
			t.Errorf("Code = %s, want auth-code", result.Code)
		}
	})

	t.Run("WrongMethod", func(t *testing.T) {
		server, err := NewCallbackServer("state")
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		resp, err := http.Post("http://127.0.0.1:18751/callback", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("HTTP POST error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

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

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		resp, err := http.Get("http://127.0.0.1:18751/wrong-path")
		if err != nil {
			t.Fatalf("HTTP GET error = %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

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

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

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

	t.Run("ReadyBeforeReturn", func(t *testing.T) {
		// Start() should block until the server is accepting connections,
		// so immediately after Start() returns, a dial should succeed.
		server, err := NewCallbackServer("test-state")
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		if err := server.Start(); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		// The server should already be accepting connections — no sleep needed.
		conn, err := net.DialTimeout("tcp", server.listener.Addr().String(), 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Dial to callback server failed after Start() returned: %v", err)
		}
		_ = conn.Close()
	})

	t.Run("StartTwiceReturnsError", func(t *testing.T) {
		server, err := NewCallbackServer("test-state")
		if err != nil {
			t.Fatalf("NewCallbackServer() error = %v", err)
		}
		defer server.Shutdown()

		if err := server.Start(); err != nil {
			t.Fatalf("first Start() error = %v", err)
		}

		// Second call should return an error.
		if err := server.Start(); err == nil {
			t.Error("second Start() expected error, got nil")
		}
	})
}
