package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
)

// ValidateSignature validates GitHub webhook signature
func ValidateSignature(signature string, payload []byte, secret string) bool {
	if signature == "" || len(payload) == 0 || secret == "" {
		return false
	}

	hmac256 := hmac.New(sha256.New, []byte(secret))
	hmac256.Write(payload)
	digest := "sha256=" + hex.EncodeToString(hmac256.Sum(nil))

	// Use constant time comparison to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(digest))
}

// GetSignature extracts signature from request headers
func GetSignature(headers map[string]string) string {
	// Check X-Hub-Signature-256 first (recommended)
	if sig, ok := headers["x-hub-signature-256"]; ok && sig != "" {
		return sig
	}
	// Fall back to X-Hub-Signature (deprecated but still used)
	if sig, ok := headers["x-hub-signature"]; ok && sig != "" {
		return sig
	}
	return ""
}

// GetEnvOrDefault returns environment variable or default value
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// DebugLog prints debug message if debug mode is enabled
func DebugLog(format string, args ...interface{}) {
	debug := os.Getenv("DEBUG") == "true"
	if debug {
		log.Printf("[DEBUG] "+format, args...)
	}
}
