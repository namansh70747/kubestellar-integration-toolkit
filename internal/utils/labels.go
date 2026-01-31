package utils

import (
	"regexp"
	"strings"
)

const (
	LabelKeyMaxLength    = 63
	LabelValueMaxLength  = 63
	LabelPrefixMaxLength = 253
)

var (
	labelKeyRegex   = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?[a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?$`)
	labelValueRegex = regexp.MustCompile(`^([a-zA-Z0-9]([-a-zA-Z0-9_.]*[a-zA-Z0-9])?)?$`)
)

func ValidateLabelKey(key string) []string {
	var errors []string

	if key == "" {
		errors = append(errors, "label key cannot be empty")
		return errors
	}

	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		prefix := parts[0]
		name := parts[1]

		if len(prefix) > LabelPrefixMaxLength {
			errors = append(errors, "label key prefix exceeds maximum length")
		}

		if len(name) > LabelKeyMaxLength {
			errors = append(errors, "label key name exceeds maximum length")
		}
	} else {
		if len(key) > LabelKeyMaxLength {
			errors = append(errors, "label key exceeds maximum length")
		}
	}

	if !labelKeyRegex.MatchString(key) {
		errors = append(errors, "label key contains invalid characters")
	}

	return errors
}

func ValidateLabelValue(value string) []string {
	var errors []string

	if len(value) > LabelValueMaxLength {
		errors = append(errors, "label value exceeds maximum length")
	}

	if value != "" && !labelValueRegex.MatchString(value) {
		errors = append(errors, "label value contains invalid characters")
	}

	return errors
}

func ValidateLabels(labels map[string]string) []string {
	var errors []string

	for key, value := range labels {
		keyErrors := ValidateLabelKey(key)
		for _, e := range keyErrors {
			errors = append(errors, e+" for key: "+key)
		}

		valueErrors := ValidateLabelValue(value)
		for _, e := range valueErrors {
			errors = append(errors, e+" for value: "+value)
		}
	}

	return errors
}

func NormalizeLabel(input string) string {
	input = strings.ToLower(input)
	input = strings.ReplaceAll(input, " ", "-")
	input = strings.ReplaceAll(input, "_", "-")

	reg := regexp.MustCompile(`[^a-z0-9-.]`)
	input = reg.ReplaceAllString(input, "")

	input = strings.Trim(input, "-.")

	if len(input) > LabelKeyMaxLength {
		input = input[:LabelKeyMaxLength]
	}

	return input
}

func MergeLabels(base, override map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range base {
		result[k] = v
	}

	for k, v := range override {
		result[k] = v
	}

	return result
}

func FilterLabels(labels map[string]string, prefix string) map[string]string {
	result := make(map[string]string)

	for k, v := range labels {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}

	return result
}

func RemoveLabelsByPrefix(labels map[string]string, prefix string) map[string]string {
	result := make(map[string]string)

	for k, v := range labels {
		if !strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}

	return result
}

func ContainsLabel(labels map[string]string, key, value string) bool {
	if v, exists := labels[key]; exists {
		return v == value
	}
	return false
}

func MatchesSelector(labels map[string]string, selector map[string]string) bool {
	for k, v := range selector {
		if labelValue, exists := labels[k]; !exists || labelValue != v {
			return false
		}
	}
	return true
}
