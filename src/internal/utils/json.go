package utils

import (
	"bytes"
)

// SanitizeJSON attempts to find the first '{' or '[' and the last '}' or ']'
// to extract a valid JSON string from potentially noisy output (like pip warnings).
func SanitizeJSON(data []byte) []byte {
	startChar := byte('{')
	endChar := byte('}')

	startIdx := bytes.IndexByte(data, startChar)
	startArrayIdx := bytes.IndexByte(data, '[')

	if startIdx == -1 || (startArrayIdx != -1 && startArrayIdx < startIdx) {
		startIdx = startArrayIdx
		endChar = byte(']')
	}

	if startIdx == -1 {
		return data
	}

	endIdx := bytes.LastIndexByte(data, endChar)
	if endIdx == -1 || endIdx < startIdx {
		return data
	}

	return data[startIdx : endIdx+1]
}
