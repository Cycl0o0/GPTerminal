package config

import (
	"fmt"
	"net/url"
)

// Validate checks the current configuration and returns a list of warnings.
func Validate() []string {
	var warnings []string

	// Validate api_base_url
	baseURL := APIBaseURL()
	if baseURL != "" {
		if w := validateURL(baseURL); w != "" {
			warnings = append(warnings, w)
		}
	}

	// Validate temperature
	temp := float64(Temperature())
	if temp < 0.0 || temp > 2.0 {
		warnings = append(warnings, fmt.Sprintf("temperature %.1f is out of range (0.0-2.0)", temp))
	}

	// Validate max_tokens
	tokens := MaxTokens()
	if tokens <= 0 {
		warnings = append(warnings, fmt.Sprintf("max_tokens %d must be a positive integer", tokens))
	}

	// Validate model
	model := Model()
	if model == "" {
		warnings = append(warnings, "model is empty")
	}

	return warnings
}

// ValidateURL checks if a URL is a valid http/https URL.
// Returns an error message if invalid, empty string if valid.
func ValidateURL(rawURL string) string {
	return validateURL(rawURL)
}

func validateURL(rawURL string) string {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Sprintf("api_base_url %q is not a valid URL: %v", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Sprintf("api_base_url %q must use http or https scheme", rawURL)
	}
	return ""
}
