package models

import "time"

type R4AppaMobilePaymentReversal struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Reference      string    `gorm:"column:reference" json:"reference"`
	OrderName      string    `gorm:"column:order_name" json:"orderName"`
	OrderAmount    float64   `gorm:"column:order_amount" json:"orderAmount"`
	ReversalAmount float64   `gorm:"column:reversal_amount" json:"reversalAmount"`
	Reason         string    `gorm:"column:reason" json:"reason"`
	Success        bool      `gorm:"column:success;default:false" json:"success"`
	ErrorDetail    string    `gorm:"column:error_detail" json:"errorDetail"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (R4AppaMobilePaymentReversal) TableName() string {
	return "r4_appa_mobile_payments_reversals"
}
