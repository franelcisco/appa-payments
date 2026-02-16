package models

import "time"

type R4AppaDebitDirect struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SenderPhone string    `gorm:"column:sender_phone" json:"senderPhone"`
	IssuingBank string    `gorm:"column:issuing_bank" json:"issuingBank"`
	Amount      float64   `gorm:"column:amount" json:"amount"`
	Reference   string    `gorm:"column:reference" json:"reference"`
	DNI         string    `gorm:"column:dni" json:"dni"`
	Code        string    `gorm:"column:code" json:"code"`
	Success     bool      `gorm:"column:success" json:"success"`
	OrderID     string    `gorm:"column:order_id" json:"orderId"`
	OrderName   string    `gorm:"column:order_name" json:"orderName"`
	Date        time.Time `gorm:"column:date" json:"date"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (R4AppaDebitDirect) TableName() string {
	return "r4_appa_debits_direct"
}
