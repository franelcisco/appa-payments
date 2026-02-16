package helpers

import (
	"appa_payments/pkg/shopify"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// SameDay checks if two timestamps are on the same day.
func SameDay(t1, t2 time.Time) bool {
	if t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day() {
		return true
	}
	return false
}

func GenerateAuthToken(key, message string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// GetCustomerDNI extracts the DNI and DNIType from the customer ParentID
func GetCustomerDNI(dni, dniType string, parentID *shopify.Metafield) string {
	if dni != "" && dniType != "" {
		return fmt.Sprintf("%s%s", dniType, dni)
	}

	if parentID != nil {
		parentID := strings.Split(parentID.Value, "-")
		if len(parentID) == 2 {
			return fmt.Sprintf("%s%s", parentID[1], parentID[0])
		}
	}

	return ""
}
