package utils

import (
	"bytes"
	"encoding/json"
)

// SanitizeJSON extracts the first valid JSON object or array from a byte slice.
// It is useful for cleaning output from tools like pip that might append warnings or other noise.
func SanitizeJSON(data []byte) []byte {
	// Find the first occurrence of '{' or '['
	startIdx := bytes.IndexAny(data, "{[")
	if startIdx == -1 {
		return data
	}

	trimmed := data[startIdx:]
	decoder := json.NewDecoder(bytes.NewReader(trimmed))

	var raw json.RawMessage
	err := decoder.Decode(&raw)
	if err != nil {
		// Fallback: if decoding fails, just return original (or trimmed)
		return trimmed
	}

	// The decoder only reads one JSON value.
	// We return exactly that value.
	return []byte(raw)
}
