package repository

import (
	"testing"

	"ecommerce/order-service/internal/core/domain/entity"
)

func TestCanTransitionOrderStatus(t *testing.T) {
	tests := []struct {
		name    string
		current string
		next    string
		allowed bool
	}{
		{name: "pending to confirmed", current: entity.OrderStatusPending, next: entity.OrderStatusConfirmed, allowed: true},
		{name: "pending to cancelled", current: entity.OrderStatusPending, next: entity.OrderStatusCancelled, allowed: true},
		{name: "confirmed to process", current: entity.OrderStatusConfirmed, next: entity.OrderStatusProcess, allowed: true},
		{name: "process to sending", current: entity.OrderStatusProcess, next: entity.OrderStatusSending, allowed: true},
		{name: "sending to done", current: entity.OrderStatusSending, next: entity.OrderStatusDone, allowed: true},
		{name: "done is terminal", current: entity.OrderStatusDone, next: entity.OrderStatusCancelled, allowed: false},
		{name: "reject mixed case enum value", current: "Pending", next: entity.OrderStatusConfirmed, allowed: false},
		{name: "reject unknown target", current: entity.OrderStatusPending, next: "UNKNOWN", allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if allowed := canTransitionOrderStatus(test.current, test.next); allowed != test.allowed {
				t.Errorf("canTransitionOrderStatus(%q, %q) = %t, want %t", test.current, test.next, allowed, test.allowed)
			}
		})
	}
}
