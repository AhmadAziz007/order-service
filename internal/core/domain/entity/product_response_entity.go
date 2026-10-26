package entity

import (
	"encoding/json"
	"time"
)

type ProductHttpClientResponse struct {
	Message string                `json:"message"`
	Data    ProductResponseEntity `json:"data"`
}

type ChildProductResponseEntity struct {
	ID           int     `json:"id"`
	Weight       int     `json:"weight"`
	Stock        int     `json:"stock"`
	RegulerPrice float64 `json:"reguler_price"`
	SalePrice    float64 `json:"sale_price"`
	Unit         string  `json:"unit"`
	Image        string  `json:"image"`
}

type ProductResponseEntity struct {
	ID            int                          `json:"id"`
	ProductName   string                       `json:"product_name"`
	ParentID      int                          `json:"parent_id"`
	ProductImage  string                       `json:"product_image"`
	CategoryName  string                       `json:"category_name"`
	ProductStatus string                       `json:"product_status"`
	SalePrice     float64                      `json:"sale_price"`
	RegulerPrice  float64                      `json:"reguler_price"`
	CreatedAt     time.Time                    `json:"created_at"`
	Unit          string                       `json:"unit"`
	Weight        int                          `json:"weight"`
	Stock         int                          `json:"stock"`
	Child         []ChildProductResponseEntity `json:"child"`
}

func (p *ProductResponseEntity) UnmarshalJSON(data []byte) error {
	type Alias ProductResponseEntity
	aux := struct {
		Image string `json:"image"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if p.ProductImage == "" {
		p.ProductImage = aux.Image
	}
	if p.ProductImage == "" && len(p.Child) > 0 && p.Child[0].Image != "" {
		p.ProductImage = p.Child[0].Image
	}
	return nil
}
