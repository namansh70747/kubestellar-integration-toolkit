package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateLabelKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantErrs bool
	}{
		{"valid key", "app.kubernetes.io/name", false},
		{"valid short key", "app", false},
		{"empty key", "", true},
		{"too long prefix", string(make([]byte, 254)) + "/key", true},
		{"invalid characters", "app@name", true},
		{"invalid start", "-app", true},
		{"invalid end", "app-", true},
		{"uppercase prefix", "APP.io/name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateLabelKey(tt.key)
			if tt.wantErrs {
				assert.NotEmpty(t, errs, "Expected validation errors but got none")
			} else {
				assert.Empty(t, errs, "Expected no validation errors but got: %v", errs)
			}
		})
	}
}

func TestValidateLabelValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantErrs bool
	}{
		{"valid value", "production", false},
		{"valid with dash", "prod-env", false},
		{"empty value", "", false}, // Empty is allowed
		{"too long", string(make([]byte, 64)), true},
		{"invalid characters", "prod@env", true},
		{"invalid start", "-prod", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateLabelValue(tt.value)
			if tt.wantErrs {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with spaces", "my app name", "my-app-name"},
		{"with underscores", "my_app_name", "my-app-name"},
		{"mixed case", "MyAppName", "myappname"},
		{"invalid chars", "my@app#name", "myappname"},
		{"leading dash", "-myapp", "myapp"},
		{"trailing dash", "myapp-", "myapp"},
		{"too long", string(make([]byte, 100)), string(make([]byte, 63))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeLabel(tt.input)
			if tt.name != "too long" {
				assert.Equal(t, tt.expected, result)
			} else {
				assert.LessOrEqual(t, len(result), 63)
			}
		})
	}
}

func TestMergeLabels(t *testing.T) {
	base := map[string]string{
		"app":     "myapp",
		"version": "v1",
		"env":     "dev",
	}
	override := map[string]string{
		"version": "v2",
		"env":     "prod",
		"tier":    "frontend",
	}

	result := MergeLabels(base, override)

	assert.Equal(t, "myapp", result["app"], "Base label should be preserved")
	assert.Equal(t, "v2", result["version"], "Override should replace base")
	assert.Equal(t, "prod", result["env"], "Override should replace base")
	assert.Equal(t, "frontend", result["tier"], "New label should be added")
	assert.Len(t, result, 4, "Result should have 4 labels")
}

func TestFilterLabels(t *testing.T) {
	labels := map[string]string{
		"app.kubernetes.io/name":    "myapp",
		"app.kubernetes.io/version": "v1",
		"custom.io/label":           "value",
		"environment":               "prod",
	}

	result := FilterLabels(labels, "app.kubernetes.io/")

	assert.Len(t, result, 2, "Should filter to 2 labels")
	assert.Equal(t, "myapp", result["app.kubernetes.io/name"])
	assert.Equal(t, "v1", result["app.kubernetes.io/version"])
	assert.NotContains(t, result, "custom.io/label")
	assert.NotContains(t, result, "environment")
}

func TestRemoveLabelsByPrefix(t *testing.T) {
	labels := map[string]string{
		"app.kubernetes.io/name":    "myapp",
		"app.kubernetes.io/version": "v1",
		"custom.io/label":           "value",
		"environment":               "prod",
	}

	result := RemoveLabelsByPrefix(labels, "app.kubernetes.io/")

	assert.Len(t, result, 2, "Should have 2 labels remaining")
	assert.NotContains(t, result, "app.kubernetes.io/name")
	assert.NotContains(t, result, "app.kubernetes.io/version")
	assert.Contains(t, result, "custom.io/label")
	assert.Contains(t, result, "environment")
}

func TestContainsLabel(t *testing.T) {
	labels := map[string]string{
		"app":     "myapp",
		"version": "v1",
	}

	assert.True(t, ContainsLabel(labels, "app", "myapp"))
	assert.False(t, ContainsLabel(labels, "app", "other"))
	assert.False(t, ContainsLabel(labels, "missing", "value"))
}

func TestMatchesSelector(t *testing.T) {
	labels := map[string]string{
		"app":         "myapp",
		"version":     "v1",
		"environment": "prod",
	}

	tests := []struct {
		name     string
		selector map[string]string
		expected bool
	}{
		{
			name:     "exact match",
			selector: map[string]string{"app": "myapp", "version": "v1"},
			expected: true,
		},
		{
			name:     "partial match",
			selector: map[string]string{"app": "myapp"},
			expected: true,
		},
		{
			name:     "no match",
			selector: map[string]string{"app": "other"},
			expected: false,
		},
		{
			name:     "missing key",
			selector: map[string]string{"tier": "frontend"},
			expected: false,
		},
		{
			name:     "empty selector",
			selector: map[string]string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesSelector(labels, tt.selector)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		wantErrs bool
	}{
		{
			name: "valid labels",
			labels: map[string]string{
				"app":                       "myapp",
				"app.kubernetes.io/name":    "test",
				"app.kubernetes.io/version": "v1",
			},
			wantErrs: false,
		},
		{
			name: "invalid key",
			labels: map[string]string{
				"app":      "myapp",
				"@invalid": "value",
			},
			wantErrs: true,
		},
		{
			name: "invalid value",
			labels: map[string]string{
				"app": "@invalid",
			},
			wantErrs: true,
		},
		{
			name: "too long key",
			labels: map[string]string{
				string(make([]byte, 64)): "value",
			},
			wantErrs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateLabels(tt.labels)
			if tt.wantErrs {
				assert.NotEmpty(t, errs)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}
