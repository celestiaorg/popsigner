package banhbaoring

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaoError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *BaoError
		expected string
	}{
		{
			name:     "with message",
			err:      &BaoError{StatusCode: 403, Errors: []string{"denied"}},
			expected: "OpenBao error (HTTP 403): denied",
		},
		{
			name:     "without message",
			err:      &BaoError{StatusCode: 500},
			expected: "OpenBao error (HTTP 500)",
		},
		{
			name:     "empty errors slice",
			err:      &BaoError{StatusCode: 404, Errors: []string{}},
			expected: "OpenBao error (HTTP 404)",
		},
		{
			name:     "multiple errors uses first",
			err:      &BaoError{StatusCode: 400, Errors: []string{"first error", "second error"}},
			expected: "OpenBao error (HTTP 400): first error",
		},
		{
			name:     "with request ID",
			err:      &BaoError{StatusCode: 503, RequestID: "req-123"},
			expected: "OpenBao error (HTTP 503)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestBaoError_Is(t *testing.T) {
	tests := []struct {
		name       string
		baoErr     *BaoError
		target     error
		shouldMatch bool
	}{
		{
			name:        "403 matches ErrBaoAuth",
			baoErr:      &BaoError{StatusCode: 403},
			target:      ErrBaoAuth,
			shouldMatch: true,
		},
		{
			name:        "404 matches ErrKeyNotFound",
			baoErr:      &BaoError{StatusCode: 404},
			target:      ErrKeyNotFound,
			shouldMatch: true,
		},
		{
			name:        "503 matches ErrBaoSealed",
			baoErr:      &BaoError{StatusCode: 503},
			target:      ErrBaoSealed,
			shouldMatch: true,
		},
		{
			name:        "403 does not match ErrKeyNotFound",
			baoErr:      &BaoError{StatusCode: 403},
			target:      ErrKeyNotFound,
			shouldMatch: false,
		},
		{
			name:        "500 does not match any sentinel",
			baoErr:      &BaoError{StatusCode: 500},
			target:      ErrBaoAuth,
			shouldMatch: false,
		},
		{
			name:        "200 does not match any sentinel",
			baoErr:      &BaoError{StatusCode: 200},
			target:      ErrSigningFailed,
			shouldMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.baoErr, tt.target)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestNewBaoError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errs       []string
		requestID  string
	}{
		{
			name:       "with all fields",
			statusCode: 403,
			errs:       []string{"permission denied"},
			requestID:  "req-12345",
		},
		{
			name:       "with empty errors",
			statusCode: 500,
			errs:       nil,
			requestID:  "",
		},
		{
			name:       "with multiple errors",
			statusCode: 400,
			errs:       []string{"error1", "error2"},
			requestID:  "req-multi",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBaoError(tt.statusCode, tt.errs, tt.requestID)
			require.NotNil(t, err)
			assert.Equal(t, tt.statusCode, err.StatusCode)
			assert.Equal(t, tt.errs, err.Errors)
			assert.Equal(t, tt.requestID, err.RequestID)
		})
	}
}

func TestKeyError_Error(t *testing.T) {
	tests := []struct {
		name     string
		keyErr   *KeyError
		expected string
	}{
		{
			name:     "sign operation",
			keyErr:   &KeyError{Op: "sign", KeyName: "mykey", Err: ErrSigningFailed},
			expected: `sign key "mykey": banhbaoring: signing failed`,
		},
		{
			name:     "get operation",
			keyErr:   &KeyError{Op: "get", KeyName: "testkey", Err: ErrKeyNotFound},
			expected: `get key "testkey": banhbaoring: key not found`,
		},
		{
			name:     "export operation",
			keyErr:   &KeyError{Op: "export", KeyName: "secretkey", Err: ErrKeyNotExportable},
			expected: `export key "secretkey": banhbaoring: key is not exportable`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.keyErr.Error())
		})
	}
}

func TestKeyError_Unwrap(t *testing.T) {
	t.Run("unwraps to underlying error", func(t *testing.T) {
		keyErr := &KeyError{Op: "sign", KeyName: "mykey", Err: ErrSigningFailed}
		unwrapped := keyErr.Unwrap()
		assert.Equal(t, ErrSigningFailed, unwrapped)
	})

	t.Run("errors.Is works through unwrap", func(t *testing.T) {
		keyErr := &KeyError{Op: "sign", KeyName: "mykey", Err: ErrSigningFailed}
		assert.True(t, errors.Is(keyErr, ErrSigningFailed))
	})

	t.Run("nested unwrap chain", func(t *testing.T) {
		innerErr := &KeyError{Op: "inner", KeyName: "inner-key", Err: ErrKeyNotFound}
		outerErr := &KeyError{Op: "outer", KeyName: "outer-key", Err: innerErr}
		
		assert.True(t, errors.Is(outerErr, ErrKeyNotFound))
	})
}

func TestWrapKeyError(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		keyName  string
		err      error
		wantNil  bool
	}{
		{
			name:    "wraps non-nil error",
			op:      "sign",
			keyName: "mykey",
			err:     ErrSigningFailed,
			wantNil: false,
		},
		{
			name:    "returns nil for nil error",
			op:      "sign",
			keyName: "mykey",
			err:     nil,
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapKeyError(tt.op, tt.keyName, tt.err)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				keyErr, ok := result.(*KeyError)
				require.True(t, ok)
				assert.Equal(t, tt.op, keyErr.Op)
				assert.Equal(t, tt.keyName, keyErr.KeyName)
				assert.Equal(t, tt.err, keyErr.Err)
			}
		})
	}
}

func TestWrapKeyError_ErrorsIs(t *testing.T) {
	err := WrapKeyError("sign", "mykey", ErrSigningFailed)
	assert.True(t, errors.Is(err, ErrSigningFailed))
	assert.Contains(t, err.Error(), "mykey")
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		valErr   *ValidationError
		expected string
	}{
		{
			name:     "missing field",
			valErr:   &ValidationError{Field: "BaoAddr", Message: "is required"},
			expected: "validation error: BaoAddr - is required",
		},
		{
			name:     "invalid format",
			valErr:   &ValidationError{Field: "Timeout", Message: "must be positive"},
			expected: "validation error: Timeout - must be positive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.valErr.Error())
		})
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("TestField", "test message")
	require.NotNil(t, err)
	assert.Equal(t, "TestField", err.Field)
	assert.Equal(t, "test message", err.Message)
}

func TestSentinelErrors(t *testing.T) {
	// Test that sentinel errors are distinct
	sentinelErrors := []error{
		ErrMissingBaoAddr,
		ErrMissingBaoToken,
		ErrMissingStorePath,
		ErrKeyNotFound,
		ErrKeyExists,
		ErrKeyNotExportable,
		ErrBaoConnection,
		ErrBaoAuth,
		ErrBaoSealed,
		ErrBaoUnavailable,
		ErrSigningFailed,
		ErrInvalidSignature,
		ErrUnsupportedAlgo,
		ErrStorePersist,
		ErrStoreCorrupted,
	}

	for i, err1 := range sentinelErrors {
		for j, err2 := range sentinelErrors {
			if i == j {
				assert.True(t, errors.Is(err1, err2), "error should match itself: %v", err1)
			} else {
				assert.False(t, errors.Is(err1, err2), "error %v should not match %v", err1, err2)
			}
		}
	}
}

func TestSentinelErrorMessages(t *testing.T) {
	tests := []struct {
		err      error
		contains string
	}{
		{ErrMissingBaoAddr, "BaoAddr"},
		{ErrMissingBaoToken, "BaoToken"},
		{ErrMissingStorePath, "StorePath"},
		{ErrKeyNotFound, "key not found"},
		{ErrKeyExists, "key already exists"},
		{ErrKeyNotExportable, "not exportable"},
		{ErrBaoConnection, "connect"},
		{ErrBaoAuth, "authentication"},
		{ErrBaoSealed, "sealed"},
		{ErrBaoUnavailable, "unavailable"},
		{ErrSigningFailed, "signing"},
		{ErrInvalidSignature, "signature"},
		{ErrUnsupportedAlgo, "algorithm"},
		{ErrStorePersist, "persist"},
		{ErrStoreCorrupted, "corrupted"},
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			assert.Contains(t, tt.err.Error(), tt.contains)
			assert.Contains(t, tt.err.Error(), "banhbaoring:")
		})
	}
}

func TestErrorsAs(t *testing.T) {
	t.Run("BaoError can be extracted with errors.As", func(t *testing.T) {
		var baoErr *BaoError
		err := NewBaoError(404, []string{"not found"}, "req-123")
		
		assert.True(t, errors.As(err, &baoErr))
		assert.Equal(t, 404, baoErr.StatusCode)
	})

	t.Run("KeyError can be extracted with errors.As", func(t *testing.T) {
		var keyErr *KeyError
		err := WrapKeyError("get", "testkey", ErrKeyNotFound)
		
		assert.True(t, errors.As(err, &keyErr))
		assert.Equal(t, "testkey", keyErr.KeyName)
	})

	t.Run("ValidationError can be extracted with errors.As", func(t *testing.T) {
		var valErr *ValidationError
		err := NewValidationError("Field", "is required")
		
		assert.True(t, errors.As(err, &valErr))
		assert.Equal(t, "Field", valErr.Field)
	})
}

func TestBaoError_HTTPStatusCodeMapping(t *testing.T) {
	// Test the full range of HTTP status code mappings
	tests := []struct {
		statusCode int
		sentinel   error
		shouldMap  bool
	}{
		{400, ErrBaoAuth, false},
		{401, ErrBaoAuth, false},
		{403, ErrBaoAuth, true},
		{404, ErrKeyNotFound, true},
		{500, ErrBaoSealed, false},
		{502, ErrBaoSealed, false},
		{503, ErrBaoSealed, true},
		{504, ErrBaoSealed, false},
	}
	for _, tt := range tests {
		t.Run(tt.sentinel.Error(), func(t *testing.T) {
			baoErr := NewBaoError(tt.statusCode, nil, "")
			assert.Equal(t, tt.shouldMap, errors.Is(baoErr, tt.sentinel))
		})
	}
}

