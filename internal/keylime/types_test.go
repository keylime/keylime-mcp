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
	assert.False(t, IsFailedState(StateRegistered))
	assert.False(t, IsFailedState(StateStart))
	assert.False(t, IsFailedState(StateSaved))
	assert.False(t, IsFailedState(StateGetQuote))
	assert.False(t, IsFailedState(StateGetQuoteRetry))
	assert.False(t, IsFailedState(StateProvideV))
	assert.False(t, IsFailedState(StateProvideVRetry))
	assert.True(t, IsFailedState(StateFailed))
	assert.False(t, IsFailedState(StateTerminated))
	assert.True(t, IsFailedState(StateInvalidQuote))
	assert.True(t, IsFailedState(StateTenantFailed))
	assert.False(t, IsFailedState(99))
	assert.False(t, IsFailedState(-1))
}
