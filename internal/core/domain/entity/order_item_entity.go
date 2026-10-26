package entity

import "time"

type OrderItemEntity struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	ProductID     int64     `json:"product_id"`
	Quantity      int64     `json:"quantity"`
	OrderCode     string    `json:"order_code"`
	ProductName   string    `json:"product_name"`
	ProductImage  string    `json:"product_image"`
	Price         int64     `json:"price"`
	ProductUnit   string    `json:"product_unit"`
	ProductWeight int64     `json:"product_weight"`
	OrderDate     time.Time `json:"order_date"`
}

type PublishOrderItemEntity struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}
