package models

import "time"

// Payout statuses. "pending" is written before R4 is called; a row still
// pending afterwards means the process died mid-call and is treated as unknown.
const (
	PayoutPending   = "pending"
	PayoutConfirmed = "confirmed"
	PayoutRejected  = "rejected"
	PayoutUnknown   = "unknown"
)

// R4AppaPayout is one Vuelto sent on behalf of a caller (APPA claims). PayoutID
// is the caller's own id for the payout and is UNIQUE: it is the idempotency key
// MBvuelto itself doesn't have. A second request with the same id never reaches
// R4 — it gets the stored outcome back.
type R4AppaPayout struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	PayoutID  string    `gorm:"column:payout_id;uniqueIndex" json:"payoutId"`
	Status    string    `gorm:"column:status" json:"status"`
	Bank      string    `gorm:"column:bank" json:"bank"`
	Phone     string    `gorm:"column:phone" json:"phone"`
	DNI       string    `gorm:"column:dni" json:"dni"`
	Amount    float64   `gorm:"column:amount" json:"amount"`
	Concept   string    `gorm:"column:concept" json:"concept"`
	Reference string    `gorm:"column:reference" json:"reference"`
	Detail    string    `gorm:"column:detail" json:"detail"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (R4AppaPayout) TableName() string {
	return "r4_appa_payouts"
}
