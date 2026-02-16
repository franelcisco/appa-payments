package models

import "time"

type PaymentMethod struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name;size:64;not null;unique" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:now()" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp;default:now()" json:"updatedAt"`
}

func (PaymentMethod) TableName() string {
	return "payment_methods"
}
