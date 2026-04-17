package keylime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateToString(t *testing.T) {
	tests := []struct {
		name     string
		state    int
		expected string
	}{
		{"Registered", StateRegistered, "Registered"},
		{"Start", StateStart, "Start"},
		{"Saved", StateSaved, "Saved"},
		{"GetQuote", StateGetQuote, "Get Quote"},
		{"GetQuoteRetry", StateGetQuoteRetry, "Get Quote (retry)"},
		{"ProvideV", StateProvideV, "Provide V"},
		{"ProvideVRetry", StateProvideVRetry, "Provide V (retry)"},
		{"Failed", StateFailed, "Failed"},
		{"Terminated", StateTerminated, "Terminated"},
		{"InvalidQuote", StateInvalidQuote, "Invalid Quote"},
		{"TenantFailed", StateTenantFailed, "Tenant Quote Failed"},
		{"Unknown positive", 99, "Unknown"},
		{"Unknown negative", -1, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StateToString(tt.state))
		})
	}
}

func TestIsFailedState(t *testing.T) {
	tests := []struct {
		name     string
		state    int
		expected bool
	}{
		{"Registered", StateRegistered, false},
		{"Start", StateStart, false},
		{"Saved", StateSaved, false},
		{"GetQuote", StateGetQuote, false},
		{"GetQuoteRetry", StateGetQuoteRetry, false},
		{"ProvideV", StateProvideV, false},
		{"ProvideVRetry", StateProvideVRetry, false},
		{"Failed", StateFailed, true},
		{"Terminated", StateTerminated, false},
		{"InvalidQuote", StateInvalidQuote, true},
		{"TenantFailed", StateTenantFailed, true},
		{"Unknown", 99, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsFailedState(tt.state))
		})
	}
}
