package bcv

import "time"

type Rate struct {
	Date time.Time `json:"date"`
	Rate float64   `json:"rate"`
}
