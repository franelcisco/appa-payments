package models

import "time"

// RecurrentPendingPayment tracks a recurrent direct-debit charge that was
// declined by R4 at webhook time and is awaiting the daily retry job.
type RecurrentPendingPayment struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID       string    `gorm:"column:order_id;unique" json:"orderId"`
	OrderName     string    `gorm:"column:order_name" json:"orderName"`
	Attempts      int       `gorm:"column:attempts;default:1" json:"attempts"`
	LastAttemptAt time.Time `gorm:"column:last_attempt_at" json:"lastAttemptAt"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (RecurrentPendingPayment) TableName() string {
	return "r4_appa_recurrent_pending_payments"
}
