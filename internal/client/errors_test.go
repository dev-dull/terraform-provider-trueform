package client

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         *APIError
		wantMsg     string
		isNotFound  bool
		isAuth      bool
		isValidation bool
	}{
		{
			name: "not found error",
			err: &APIError{
				Code:    ErrCodeNotFound,
				Message: "Resource not found",
			},
			wantMsg:    "TrueNAS API error 3: Resource not found",
			isNotFound: true,
			isAuth:     false,
			isValidation: false,
		},
		{
			name: "authentication error",
			err: &APIError{
				Code:    ErrCodeNotAuthenticated,
				Message: "Not authenticated",
			},
			wantMsg:    "TrueNAS API error 1: Not authenticated",
			isNotFound: false,
			isAuth:     true,
			isValidation: false,
		},
		{
			name: "authorization error",
			err: &APIError{
				Code:    ErrCodeNotAuthorized,
				Message: "Not authorized",
			},
			wantMsg:    "TrueNAS API error 2: Not authorized",
			isNotFound: false,
			isAuth:     true,
			isValidation: false,
		},
		{
			name: "validation error",
			err: &APIError{
				Code:    ErrCodeValidation,
				Message: "Invalid input",
			},
			wantMsg:    "TrueNAS API error 4: Invalid input",
			isNotFound: false,
			isAuth:     false,
			isValidation: true,
		},
		{
			name: "error with details",
			err: &APIError{
				Code:    ErrCodeInternalError,
				Message: "Internal error",
				Details: "Stack trace here",
			},
			wantMsg:    "TrueNAS API error -32603: Internal error (Stack trace here)",
			isNotFound: false,
			isAuth:     false,
			isValidation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("APIError.Error() = %v, want %v", got, tt.wantMsg)
			}
			if got := tt.err.IsNotFound(); got != tt.isNotFound {
				t.Errorf("APIError.IsNotFound() = %v, want %v", got, tt.isNotFound)
			}
			if got := tt.err.IsAuthError(); got != tt.isAuth {
				t.Errorf("APIError.IsAuthError() = %v, want %v", got, tt.isAuth)
			}
			if got := tt.err.IsValidationError(); got != tt.isValidation {
				t.Errorf("APIError.IsValidationError() = %v, want %v", got, tt.isValidation)
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	t.Run("with wrapped error", func(t *testing.T) {
		innerErr := errors.New("connection refused")
		err := NewConnectionError("192.168.1.100", innerErr)

		if err.Host != "192.168.1.100" {
			t.Errorf("ConnectionError.Host = %v, want '192.168.1.100'", err.Host)
		}
		if err.Err != innerErr {
			t.Errorf("ConnectionError.Err = %v, want %v", err.Err, innerErr)
		}
		if !errors.Is(err, innerErr) {
			t.Error("errors.Is failed to match wrapped error")
		}

		// Check that error message contains key information
		errMsg := err.Error()
		if !contains(errMsg, "192.168.1.100") {
			t.Errorf("ConnectionError.Error() should contain host")
		}
		if !contains(errMsg, "connection refused") {
			t.Errorf("ConnectionError.Error() should contain underlying error")
		}
		if !contains(errMsg, "provider \"trueform\"") {
			t.Errorf("ConnectionError.Error() should contain example configuration")
		}
	})

	t.Run("without wrapped error", func(t *testing.T) {
		err := NewConnectionError("truenas.local", nil)

		if err.Host != "truenas.local" {
			t.Errorf("ConnectionError.Host = %v, want 'truenas.local'", err.Host)
		}
		if err.Err != nil {
			t.Errorf("ConnectionError.Err = %v, want nil", err.Err)
		}

		errMsg := err.Error()
		if !contains(errMsg, "truenas.local") {
			t.Errorf("ConnectionError.Error() should contain host")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNewAPIError(t *testing.T) {
	t.Run("from JSONRPCError without data", func(t *testing.T) {
		rpcErr := &JSONRPCError{
			Code:    -32600,
			Message: "Invalid Request",
		}

		apiErr := NewAPIError(rpcErr)

		if apiErr.Code != -32600 {
			t.Errorf("APIError.Code = %v, want -32600", apiErr.Code)
		}
		if apiErr.Message != "Invalid Request" {
			t.Errorf("APIError.Message = %v, want 'Invalid Request'", apiErr.Message)
		}
		if apiErr.Details != "" {
			t.Errorf("APIError.Details = %v, want empty string", apiErr.Details)
		}
	})

	t.Run("from JSONRPCError with data", func(t *testing.T) {
		data, _ := json.Marshal("extra info")
		rpcErr := &JSONRPCError{
			Code:    -32602,
			Message: "Invalid params",
			Data:    data,
		}

		apiErr := NewAPIError(rpcErr)

		if apiErr.Code != -32602 {
			t.Errorf("APIError.Code = %v, want -32602", apiErr.Code)
		}
		if apiErr.Details != `"extra info"` {
			t.Errorf("APIError.Details = %v, want '\"extra info\"'", apiErr.Details)
		}
	})
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "API not found error",
			err:  &APIError{Code: ErrCodeNotFound, Message: "not found"},
			want: true,
		},
		{
			name: "API other error",
			err:  &APIError{Code: ErrCodeInternalError, Message: "internal"},
			want: false,
		},
		{
			name: "standard error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFoundError(tt.err); got != tt.want {
				t.Errorf("IsNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "not authenticated error",
			err:  &APIError{Code: ErrCodeNotAuthenticated, Message: "not authenticated"},
			want: true,
		},
		{
			name: "not authorized error",
			err:  &APIError{Code: ErrCodeNotAuthorized, Message: "not authorized"},
			want: true,
		},
		{
			name: "other API error",
			err:  &APIError{Code: ErrCodeNotFound, Message: "not found"},
			want: false,
		},
		{
			name: "standard error",
			err:  errors.New("some error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthError(tt.err); got != tt.want {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}
