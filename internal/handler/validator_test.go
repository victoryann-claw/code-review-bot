package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestValidateSignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"action": "opened"}`)

	// Generate valid signature
	hmac256 := hmac.New(sha256.New, []byte(secret))
	hmac256.Write(payload)
	validSignature := "sha256=" + hex.EncodeToString(hmac256.Sum(nil))

	tests := []struct {
		name      string
		signature string
		payload   []byte
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			signature: validSignature,
			payload:   payload,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			signature: "sha256=invalidsignature",
			payload:   payload,
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty signature",
			signature: "",
			payload:   payload,
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty payload",
			signature: validSignature,
			payload:   []byte{},
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty secret",
			signature: validSignature,
			payload:   payload,
			secret:    "",
			want:      false,
		},
		{
			name:      "wrong secret",
			signature: validSignature,
			payload:   payload,
			secret:    "wrong-secret",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateSignature(tt.signature, tt.payload, tt.secret)
			if got != tt.want {
				t.Errorf("ValidateSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSignature(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name:    "X-Hub-Signature-256 present",
			headers: map[string]string{"x-hub-signature-256": "sha256=abc123"},
			want:    "sha256=abc123",
		},
		{
			name:    "X-Hub-Signature present only",
			headers: map[string]string{"x-hub-signature": "sha1=abc123"},
			want:    "sha1=abc123",
		},
		{
			name:    "X-Hub-Signature-256 takes precedence",
			headers: map[string]string{"x-hub-signature-256": "sha256=abc123", "x-hub-signature": "sha1=abc123"},
			want:    "sha256=abc123",
		},
		{
			name:    "empty headers",
			headers: map[string]string{},
			want:    "",
		},
		{
			name:    "nil headers",
			headers: nil,
			want:    "",
		},
		{
			name:    "empty signature values",
			headers: map[string]string{"x-hub-signature-256": ""},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSignature(tt.headers)
			if got != tt.want {
				t.Errorf("GetSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	// Set a test environment variable
	os.Setenv("TEST_ENV_VAR", "test-value")
	defer os.Unsetenv("TEST_ENV_VAR")

	tests := []struct {
		name        string
		key         string
		defaultVal  string
		want        string
	}{
		{
			name:       "environment variable exists",
			key:        "TEST_ENV_VAR",
			defaultVal: "default",
			want:       "test-value",
		},
		{
			name:       "environment variable does not exist",
			key:        "NON_EXISTENT_VAR_12345",
			defaultVal: "default-value",
			want:       "default-value",
		},
		{
			name:       "empty default value",
			key:        "NON_EXISTENT_VAR_12345",
			defaultVal: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEnvOrDefault(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetEnvOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
