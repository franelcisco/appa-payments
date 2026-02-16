package models

import "time"

type R4AppaMobilePayment struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	IDCommerce    string    `gorm:"column:id_commerce" json:"idCommerce"`
	CommercePhone string    `gorm:"column:commerce_phone" json:"commercePhone"`
	SenderPhone   string    `gorm:"column:sender_phone" json:"senderPhone"`
	IssuingBank   string    `gorm:"column:issuing_bank" json:"issuingBank"`
	Amount        float64   `gorm:"column:amount" json:"amount"`
	Reference     string    `gorm:"column:reference" json:"reference"`
	OrderID       *int      `gorm:"column:order_id" json:"orderId"`
	OrderName     string    `gorm:"column:order_name" json:"orderName"`
	Date          time.Time `gorm:"column:date" json:"date"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (R4AppaMobilePayment) TableName() string {
	return "r4_appa_mobile_payments"
}
