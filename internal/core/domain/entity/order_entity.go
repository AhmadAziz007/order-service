package entity

import "time"

type ShippingType string

const (
	OrderStatusPending   = "PENDING"
	OrderStatusDone      = "DONE"
	OrderStatusConfirmed = "CONFIRMED"
	OrderStatusCancelled = "CANCELLED"
	OrderStatusProcess   = "PROCESS"
	OrderStatusSending   = "SENDING"

	ShippingTypeDelivery ShippingType = "DELIVERY"
	ShippingTypePickup   ShippingType = "PICKUP"
)

type OrderEntity struct {
	ID            int64             `json:"id"`
	OrderCode     string            `json:"order_code"`
	BuyerId       int64             `json:"buyer_id"`
	OrderDate     string            `json:"order_date"`
	Status        string            `json:"status"`
	TotalAmount   int64             `json:"total_amount"`
	PaymentMethod string            `json:"payment_method"`
	ShippingType  ShippingType      `json:"shipping_type"`
	ShippingFee   int64             `json:"shipping_fee"`
	OrderTime     string            `json:"order_time"`
	Remarks       string            `json:"remarks"`
	CreatedAt     time.Time         `json:"created_at"`
	CreatedBy     int64             `json:"-"`
	UpdatedBy     int64             `json:"-"`
	OrderItems    []OrderItemEntity `json:"order_items"`
	BuyerName     string            `json:"buyer_name"`
	BuyerEmail    string            `json:"buyer_email"`
	BuyerPhone    string            `json:"buyer_phone"`
	BuyerAddress  string            `json:"buyer_address"`
	BuyerLat      string            `json:"buyer_lat"`
	BuyerLng      string            `json:"buyer_lng"`
	BuyerPhoto    string            `json:"buyer_photo"`
	Latitude      string            `json:"-"`
	Longitude     string            `json:"-"`
}

type CreatedOrderEntity struct {
	OrderID   int64  `json:"order_id"`
	OrderCode string `json:"order_code"`
}

type QueryStringEntity struct {
	Page    int64
	Search  string
	Limit   int64
	Status  string
	BuyerID int64
}
