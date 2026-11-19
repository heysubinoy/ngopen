package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

type APIResponse struct {
	Valid  bool   `json:"valid"`
	Error  string `json:"error,omitempty"`
	UserID string `json:"userId,omitempty"`
}

// validationClient bounds how long a tunnel handshake can wait on the
// external auth service.
var validationClient = &http.Client{Timeout: 5 * time.Second}

// IsValidToken checks a client auth token against the service at
// API_VALIDATE_URL. Any failure (unset URL, network error, non-200, bad body)
// is treated as invalid. The token itself is never logged.
func IsValidToken(token string) bool {
	apiURL := os.Getenv("API_VALIDATE_URL")
	if apiURL == "" {
		LogError("API_VALIDATE_URL is not set")
		return false
	}

	body, err := json.Marshal(map[string]string{"key": token})
	if err != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		LogError("Token validation: bad request: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := validationClient.Do(req)
	if err != nil {
		LogError("Token validation: request failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		LogWarn("Token validation: auth service returned %d", resp.StatusCode)
		return false
	}

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		LogError("Token validation: bad response body: %v", err)
		return false
	}
	LogDebug("Token validation: valid=%t user=%s", result.Valid, result.UserID)
	return result.Valid
}
