package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

type APIResponse struct {
	Valid  bool   `json:"valid"`
	Error  string `json:"error,omitempty"`
	UserID string `json:"userId,omitempty"`
}

func IsValidToken(token string) bool {
	apiURL := os.Getenv("API_VALIDATE_URL")
	if apiURL == "" {
		LogError("API_VALIDATE_URL is not set")
		return false
	}

	payload := map[string]string{"key": token}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result APIResponse
	json.NewDecoder(resp.Body).Decode(&result)
	LogInfo("API Response: %+v", result)
	return result.Valid
}
