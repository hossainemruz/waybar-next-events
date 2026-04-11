package auth

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/http"
	"time"
)

const (
	// callbackHost is the fixed callback server host and port.
	callbackHost = "127.0.0.1:18751"
	// callbackPath is the fixed callback endpoint path.
	callbackPath = "/callback"
	// serverShutdownTimeout is the timeout for graceful server shutdown.
	serverShutdownTimeout = 5 * time.Second
)

// CallbackResult holds the result of the OAuth2 callback.
type CallbackResult struct {
	Code      string
	State     string
	Error     string
	ErrorDesc string
}

// CallbackServer handles the OAuth2 redirect callback.
// It runs a local HTTP server on 127.0.0.1:18751 and waits for the callback.
type CallbackServer struct {
	expectedState string
	resultChan    chan CallbackResult
	server        *http.Server
	listener      net.Listener
}

// NewCallbackServer creates a new callback server expecting the given state.
func NewCallbackServer(expectedState string) (*CallbackServer, error) {
	// Create a dedicated mux to avoid global handler pollution
	mux := http.NewServeMux()

	srv := &CallbackServer{
		expectedState: expectedState,
		resultChan:    make(chan CallbackResult, 1),
	}

	mux.HandleFunc(callbackPath, srv.handleCallback)

	// Listen on the fixed port
	listener, err := net.Listen("tcp", callbackHost)
	if err != nil {
		return nil, fmt.Errorf("cannot start callback server on %s (is another instance running?): %w", callbackHost, err)
	}

	srv.listener = listener
	srv.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return srv, nil
}

// Start starts the callback server in a goroutine.
// Returns a channel that will be closed when the server is ready to accept connections.
func (s *CallbackServer) Start() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready) // Signal that the goroutine has started
		// Serve always returns a non-nil error, but we only care about unexpected errors
		if err := s.server.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("callback server error", "error", err)
		}
	}()
	return ready
}

// Wait waits for the callback result or context cancellation.
// Returns the callback result or an error if context is cancelled.
func (s *CallbackServer) Wait(ctx context.Context) (CallbackResult, error) {
	select {
	case result := <-s.resultChan:
		return result, nil
	case <-ctx.Done():
		return CallbackResult{}, ctx.Err()
	}
}

// Shutdown gracefully shuts down the server.
func (s *CallbackServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	_ = s.server.Shutdown(ctx)
	_ = s.listener.Close()
}

// URL returns the callback URL that should be registered with the OAuth provider.
func (s *CallbackServer) URL() string {
	return "http://" + callbackHost + callbackPath
}

// handleCallback processes the OAuth2 callback request.
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := r.URL.Query()

	// Check for provider error response first
	if errCode := query.Get("error"); errCode != "" {
		result := CallbackResult{
			Error:     errCode,
			ErrorDesc: query.Get("error_description"),
		}
		s.resultChan <- result

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>Error: %s</p><p>You can close this tab.</p></body></html>", html.EscapeString(errCode))
		return
	}

	// Validate state parameter (CSRF protection)
	state := query.Get("state")
	if state != s.expectedState {
		result := CallbackResult{
			Error: "state_mismatch",
		}
		s.resultChan <- result

		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Get authorization code
	code := query.Get("code")
	if code == "" {
		result := CallbackResult{
			Error: "missing_code",
		}
		s.resultChan <- result

		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	// Success - send result and return success page
	result := CallbackResult{
		Code:  code,
		State: state,
	}
	s.resultChan <- result

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<html>
<body>
<h1>Authentication Successful</h1>
<p>You have successfully authenticated. You can close this tab now.</p>
</body>
</html>`)
}
