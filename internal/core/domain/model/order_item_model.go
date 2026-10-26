package model

import (
	"time"

	"gorm.io/gorm"
)

type OrderItem struct {
	ID        int64          `gorm:"primaryKey"`
	OrderID   int64          `gorm:"column:order_id;not null;references:orders.id;onDelete:CASCADE"`
	ProductID int64          `gorm:"column:product_id;not null"` // You might have a Product struct
	Quantity  int64          `gorm:"column:quantity;not null;default:1"`
	CreatedBy *int64         `gorm:"column:created_by"`
	UpdatedBy *int64         `gorm:"column:updated_by"`
	CreatedAt time.Time      `gorm:"column:created_at;default:CURRENT_TIMESTAMP"`
	UpdatedAt *time.Time     `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
	Order     Order          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
