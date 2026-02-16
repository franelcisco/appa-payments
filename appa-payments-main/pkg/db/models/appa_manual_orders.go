package models

import (
	"time"
)

type ManualOrder struct {
	ID               int            `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderName        string         `gorm:"column:order_name;size:128;unique;not null" json:"orderName"`
	OrderID          int            `gorm:"column:order_id;not null" json:"orderId"`
	BillImageURL     string         `gorm:"column:bill_image_url;size:128;not null" json:"billImageUrl"`
	Amount           float64        `gorm:"column:amount;type:decimal(10,2);not null" json:"amount"`
	OrderTotalAmount float64        `gorm:"column:order_total_amount;type:decimal(10,2);not null" json:"orderTotalAmount"`
	RequiresChange   bool           `gorm:"column:requires_change;not null" json:"requiresChange"`
	ValidateStatus   string         `gorm:"column:validate_status;size:32;not null" json:"validateStatus"`
	ReturnData       *[]byte        `gorm:"column:return_data;type:jsonb" json:"returnData,omitempty"`
	PaymentMethodID  int            `gorm:"column:payment_method_id" json:"paymentMethodId,omitempty"`
	PaymentMethod    *PaymentMethod `gorm:"foreignKey:PaymentMethodID" json:"paymentMethod,omitempty"`
	CreatedAt        time.Time      `gorm:"column:created_at;type:timestamp;default:now()" json:"createdAt"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;type:timestamp;default:now()" json:"updatedAt"`
}

func (ManualOrder) TableName() string {
	return "appa_manual_orders"
}
