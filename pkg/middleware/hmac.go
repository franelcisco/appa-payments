package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ValidateHMAC is a middleware to validate HMAC-SHA256 for public endpoints
func ValidateHMAC(secret, header string) gin.HandlerFunc {
	return func(c *gin.Context) {
		hmacHeader := c.GetHeader(header)
		if hmacHeader == "" {
			fmt.Println("Missing HMAC header")
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing HMAC"})
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			fmt.Println("Error reading body:", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to read request body"})
			return
		}

		c.Request.Body = io.NopCloser(NewBuffer(bodyBytes))
		if !isValidHMAC(bodyBytes, hmacHeader, secret) {
			fmt.Println("Invalid HMAC")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid HMAC"})
			return
		}
		c.Next()
	}
}

func isValidHMAC(body []byte, hmacHeader string, secret string) bool {
	decodedHMAC, err := base64.StdEncoding.DecodeString(hmacHeader)
	if err != nil {
		return false
	}

	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedHMAC := h.Sum(nil)

	return hmac.Equal(decodedHMAC, expectedHMAC)
}

// Buffer to restore request body
type Buffer struct {
	data []byte
	pos  int
}

func NewBuffer(data []byte) *Buffer {
	return &Buffer{data: data}
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *Buffer) Close() error { return nil }
