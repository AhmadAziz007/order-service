package entity

import (
	"encoding/json"
	"testing"
)

func TestProductResponseEntityUnmarshalImageAliases(t *testing.T) {
	for _, test := range []struct {
		name string
		json string
	}{
		{name: "home image field", json: `{"product_name":"Apel","image":"home-image.jpg"}`},
		{name: "admin product image field", json: `{"product_name":"Apel","product_image":"admin-image.jpg"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var product ProductResponseEntity
			if err := json.Unmarshal([]byte(test.json), &product); err != nil {
				t.Fatalf("unmarshal product response: %v", err)
			}
			if product.ProductImage == "" {
				t.Fatal("expected product image to be populated")
			}
		})
	}
}
