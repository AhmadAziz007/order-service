package request

import "encoding/json"

type CreateOrderRequest struct {
	BuyerID       int64                `json:"buyer_id" validate:"required"`
	OrderDate     string               `json:"order_date" validate:"required"`
	TotalAmount   int64                `json:"total_amount,omitempty"`
	ShippingType  string               `json:"shipping_type" validate:"required"`
	PaymentMethod string               `json:"payment_method" validate:"required"`
	Remarks       string               `json:"remarks"`
	OrderTime     string               `json:"order_time" validate:"required"`
	OrderDetails  []OrderDetailRequest `json:"order_details" validate:"required"`
}

func (r *CreateOrderRequest) UnmarshalJSON(data []byte) error {
	type Alias CreateOrderRequest
	aux := struct {
		PaymentType string `json:"payment_type"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if r.PaymentMethod == "" {
		r.PaymentMethod = aux.PaymentType
	}
	return nil
}

type OrderDetailRequest struct {
	ProductID int64 `json:"product_id" validate:"required"`
	Quantity  int64 `json:"quantity" validate:"required"`
}

type OrderUpdateStatusRequest struct {
	Status  string `json:"status" validate:"required"`
	Remarks string `json:"remarks"`
}
