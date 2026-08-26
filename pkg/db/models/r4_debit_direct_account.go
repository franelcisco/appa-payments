package models

import "time"

type R4DebitDirectAccount struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	StoreClientID string    `gorm:"column:store_client_id" json:"storeClientId"`
	Account       string    `gorm:"column:account" json:"account"`
	Amount        float64   `gorm:"column:amount" json:"amount"`
	Reference     string    `gorm:"column:reference" json:"reference"`
	DNI           string    `gorm:"column:dni" json:"dni"`
	Code          string    `gorm:"column:code" json:"code"`
	OperationID   string    `gorm:"column:operation_id;index" json:"operationId,omitempty"`
	Success       bool      `gorm:"column:success" json:"success"`
	OrderID       string    `gorm:"column:order_id;default:null"  json:"orderId,omitempty"`
	OrderName     string    `gorm:"column:order_name;default:null" json:"orderName,omitempty"`
	IsRecurring   bool      `gorm:"column:is_recurring;default:false" json:"isRecurring"`
	DraftID       *string   `gorm:"column:draft_id;default:null" json:"draftId,omitempty"`
	CartID        string    `gorm:"column:cart_id" json:"cartId,omitempty"`
	Date          time.Time `gorm:"column:date" json:"date"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (R4DebitDirectAccount) TableName() string {
	return "r4_appa_debits_direct_account"
}
