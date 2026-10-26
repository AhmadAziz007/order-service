package model

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	ID            int64          `gorm:"primaryKey"`
	OrderCode     string         `gorm:"column:order_code;unique;not null;size:64"`
	BuyerId       int64          `gorm:"column:buyer_id;not null"` // Assuming buyer_id is a user ID
	OrderDate     time.Time      `gorm:"column:order_date;not null;default:CURRENT_TIMESTAMP"`
	Status        string         `gorm:"column:status;not null;default:'pending';size:20"`
	PaymentMethod string         `gorm:"column:payment_method;not null;default:'CASH';size:20"`
	TotalAmount   float64        `gorm:"column:total_amount;not null;default:0"`
	ShippingType  string         `gorm:"column:shipping_type;type:order_shipping_type;not null;default:'PICKUP'"`
	ShippingFee   float64        `gorm:"column:shipping_fee;not null;default:0"`
	OrderTime     string         `gorm:"column:order_time"`
	Remarks       string         `gorm:"column:remarks"`
	CreatedAt     time.Time      `gorm:"column:created_at;default:CURRENT_TIMESTAMP"`
	CreatedBy     *int64         `gorm:"column:created_by"`
	UpdatedBy     *int64         `gorm:"column:updated_by"`
	UpdatedAt     *time.Time     `gorm:"column:updated_at"`
	StockDeducted bool           `gorm:"column:stock_deducted;not null;default:false"`
	PaidAt        *time.Time     `gorm:"column:paid_at"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index"`
	OrderItems    []OrderItem    `gorm:"foreignKey:OrderID"`
}
