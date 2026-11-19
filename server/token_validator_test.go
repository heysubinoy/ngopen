package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsValidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		switch body["key"] {
		case "good":
			json.NewEncoder(w).Encode(APIResponse{Valid: true, UserID: "u1"})
		case "bad":
			json.NewEncoder(w).Encode(APIResponse{Valid: false, Error: "unknown key"})
		case "boom":
			http.Error(w, "internal", http.StatusInternalServerError)
		default:
			w.Write([]byte("not json"))
		}
	}))
	defer srv.Close()

	t.Setenv("API_VALIDATE_URL", srv.URL)
	cases := map[string]bool{
		"good":    true,
		"bad":     false,
		"boom":    false, // non-200 from auth service
		"garbage": false, // undecodable body
	}
	for token, want := range cases {
		if got := IsValidToken(token); got != want {
			t.Errorf("IsValidToken(%q) = %t, want %t", token, got, want)
		}
	}
}

func TestIsValidTokenWithoutURL(t *testing.T) {
	t.Setenv("API_VALIDATE_URL", "")
	if IsValidToken("good") {
		t.Fatal("expected false when API_VALIDATE_URL is unset")
	}
}
