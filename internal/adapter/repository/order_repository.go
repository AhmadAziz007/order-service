package repository

import (
	"context"
	"ecommerce/order-service/internal/core/domain/entity"
	"ecommerce/order-service/internal/core/domain/model"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/labstack/gommon/log"
	"gorm.io/gorm"
)

type OrderRepositoryInterface interface {
	GetAll(ctx context.Context, queryString entity.QueryStringEntity) ([]entity.OrderEntity, int64, int64, error)
	GetByID(ctx context.Context, orderID int64) (*entity.OrderEntity, error)
	CreateOrders(ctx context.Context, orders []entity.OrderEntity) ([]int64, error)
	UpdateStatus(ctx context.Context, req entity.OrderEntity) (int64, string, string, error)
	DeleteOrder(ctx context.Context, orderID int64) error
	MarkPaymentSucceeded(ctx context.Context, orderCode string) error
	CancelExpiredPayment(ctx context.Context, orderCode string) (int64, bool, error)

	GetOrderByOrderCode(ctx context.Context, orderCode string) (*entity.OrderEntity, error)
}

type orderRepository struct {
	db *gorm.DB
}

// CreateOrders persists one order per product detail in a single transaction.
func (o *orderRepository) CreateOrders(ctx context.Context, orders []entity.OrderEntity) ([]int64, error) {
	ids := make([]int64, 0, len(orders))
	err := o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, req := range orders {
			orderDate, err := time.Parse("2006-01-02", req.OrderDate)
			if err != nil {
				return err
			}
			orderTime := req.OrderTime
			if parsedTime, parseErr := time.Parse("15:04:05", orderTime); parseErr == nil {
				orderTime = parsedTime.Format("15:04:05")
			} else if parsedTime, parseErr := time.Parse("15.04.05", orderTime); parseErr == nil {
				orderTime = parsedTime.Format("15:04:05")
			} else {
				return parseErr
			}

			orderItems := make([]model.OrderItem, 0, len(req.OrderItems))
			createdBy := req.CreatedBy
			for _, item := range req.OrderItems {
				orderItems = append(orderItems, model.OrderItem{
					ProductID: item.ProductID,
					Quantity:  item.Quantity,
					CreatedBy: &createdBy,
					UpdatedBy: nil,
				})
			}

			modelOrder := model.Order{
				OrderCode:     req.OrderCode,
				BuyerId:       req.BuyerId,
				OrderDate:     orderDate,
				OrderTime:     orderTime,
				Status:        req.Status,
				PaymentMethod: normalizePaymentMethod(req.PaymentMethod),
				TotalAmount:   float64(req.TotalAmount),
				ShippingType:  string(req.ShippingType),
				ShippingFee:   float64(req.ShippingFee),
				Remarks:       req.Remarks,
				CreatedBy:     &createdBy,
				UpdatedBy:     nil,
				OrderItems:    orderItems,
			}
			if err := tx.Create(&modelOrder).Error; err != nil {
				return err
			}
			ids = append(ids, modelOrder.ID)
		}
		return nil
	})
	if err != nil {
		log.Errorf("[OrderRepository] CreateOrders: %v", err)
		return nil, err
	}
	return ids, nil
}

// DeleteOrder implements [NewOrderRepositoryInterface].
func (o *orderRepository) DeleteOrder(ctx context.Context, orderID int64) error {
	modelOrder := model.Order{}
	if err := o.db.Preload("OrderItems").Where("id = ?", orderID).First(&modelOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[OrderRepository-1] DeleteOrder: %v", err)
			return errors.New("404")
		}
		log.Errorf("[OrderRepository-2] DeleteOrder: %v", err)
		return err
	}

	if err := o.db.Select("orderItems").Delete(&modelOrder).Error; err != nil {
		log.Errorf("[OrderRepository-3] DeleteOrder: %v", err)
		return err
	}

	return nil
}

func (o *orderRepository) MarkPaymentSucceeded(ctx context.Context, orderCode string) error {
	paidAt := time.Now()
	result := o.db.WithContext(ctx).Model(&model.Order{}).
		Where("order_code = ? AND status <> ?", orderCode, entity.OrderStatusCancelled).
		Where("paid_at IS NULL").Update("paid_at", paidAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var order model.Order
		if err := o.db.WithContext(ctx).Select("status", "paid_at").Where("order_code = ?", orderCode).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("404")
			}
			return err
		}
		if order.Status == entity.OrderStatusCancelled && order.PaidAt == nil {
			return errors.New("payment arrived after order expiration")
		}
	}
	return nil
}

func (o *orderRepository) CancelExpiredPayment(ctx context.Context, orderCode string) (int64, bool, error) {
	var order model.Order
	if err := o.db.WithContext(ctx).Select("id", "buyer_id", "status", "paid_at").
		Where("order_code = ?", orderCode).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, errors.New("404")
		}
		return 0, false, err
	}
	if order.PaidAt != nil || (order.Status != entity.OrderStatusPending && order.Status != entity.OrderStatusCancelled) {
		return order.BuyerId, false, nil
	}
	if order.Status == entity.OrderStatusPending {
		result := o.db.WithContext(ctx).Model(&model.Order{}).
			Where("id = ? AND status = ? AND paid_at IS NULL", order.ID, entity.OrderStatusPending).
			Update("status", entity.OrderStatusCancelled)
		if result.Error != nil {
			return 0, false, result.Error
		}
		if result.RowsAffected == 0 {
			return order.BuyerId, false, nil
		}
	}
	return order.BuyerId, true, nil
}

// GetAll implements [NewOrderRepositoryInterface].
func (o *orderRepository) GetAll(ctx context.Context, queryString entity.QueryStringEntity) ([]entity.OrderEntity, int64, int64, error) {
	var modelOrders []model.Order
	var countData int64
	offset := (queryString.Page - 1) * queryString.Limit

	sqlMain := o.db.Preload("OrderItems").Where("order_code ILIKE ? OR status ILIKE ?", "%"+queryString.Search+"%", "%"+queryString.Search+"%")

	if queryString.BuyerID != 0 {
		sqlMain = sqlMain.Where("buyer_id = ?", queryString.BuyerID)
	}
	if queryString.Status != "" {
		sqlMain = sqlMain.Where("status = ?", queryString.Status)
	}

	if err := sqlMain.Model(&modelOrders).Count(&countData).Error; err != nil {
		log.Errorf("[OrderRepository-1] GetAll: %v", err)
		return nil, 0, 0, err
	}

	totalpage := int(math.Ceil(float64(countData) / float64(queryString.Limit)))
	if err := sqlMain.Order("order_date DESC").Limit(int(queryString.Limit)).Offset(int(offset)).Find(&modelOrders).Error; err != nil {
		log.Errorf("[OrderRepository-2] GetAll: %v", err)
		return nil, 0, 0, err
	}

	if len(modelOrders) == 0 {
		err := errors.New("404")
		log.Infof("[OrderRepository-3] GetAll: No order found")
		return nil, 0, 0, err
	}

	entities := []entity.OrderEntity{}
	for _, val := range modelOrders {
		orderItemEntities := []entity.OrderItemEntity{}
		for _, item := range val.OrderItems {
			orderItemEntities = append(orderItemEntities, entity.OrderItemEntity{
				ID:        item.ID,
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
			})
		}

		entities = append(entities, entity.OrderEntity{
			ID:            val.ID,
			OrderCode:     val.OrderCode,
			BuyerId:       val.BuyerId,
			Status:        val.Status,
			PaymentMethod: val.PaymentMethod,
			ShippingType:  entity.ShippingType(val.ShippingType),
			ShippingFee:   int64(val.ShippingFee),
			OrderDate:     formatOrderDateTime(val.OrderDate, val.OrderTime),
			TotalAmount:   int64(val.TotalAmount),
			OrderItems:    orderItemEntities,
		})
	}

	return entities, countData, int64(totalpage), nil
}

// GetByID implements [NewOrderRepositoryInterface].
func (o *orderRepository) GetByID(ctx context.Context, orderID int64) (*entity.OrderEntity, error) {
	var modelOrder model.Order
	if err := o.db.Preload("OrderItems").Where("id = ?", orderID).First(&modelOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := errors.New("404")
			log.Errorf("[OrderRepository-1] GetByID: %v", err)
			return nil, err
		}
		log.Errorf("[OrderRepository-2] GetByID: %v", err)
		return nil, err
	}

	orderItemEntities := []entity.OrderItemEntity{}
	for _, item := range modelOrder.OrderItems {
		orderItemEntities = append(orderItemEntities, entity.OrderItemEntity{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return &entity.OrderEntity{
		ID:            modelOrder.ID,
		OrderCode:     modelOrder.OrderCode,
		BuyerId:       modelOrder.BuyerId,
		Status:        modelOrder.Status,
		PaymentMethod: normalizePaymentMethod(modelOrder.PaymentMethod),
		OrderDate:     formatOrderDateTime(modelOrder.OrderDate, modelOrder.OrderTime),
		TotalAmount:   int64(modelOrder.TotalAmount),
		OrderItems:    orderItemEntities,
		Remarks:       modelOrder.Remarks,
		ShippingType:  entity.ShippingType(modelOrder.ShippingType),
		ShippingFee:   int64(modelOrder.ShippingFee),
	}, nil
}

// GetOrderByOrderCode implements [NewOrderRepositoryInterface].
func (o *orderRepository) GetOrderByOrderCode(ctx context.Context, orderCode string) (*entity.OrderEntity, error) {
	var modelOrder model.Order
	if err := o.db.Preload("OrderItems").Where("order_code = ?", orderCode).First(&modelOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := errors.New("404")
			log.Errorf("[OrderRepository-1] GetOrderByOrderCode: %v", err)
			return nil, err
		}
		log.Errorf("[OrderRepository-2] GetOrderByOrderCode: %v", err)
		return nil, err
	}

	orderItemEntities := []entity.OrderItemEntity{}
	for _, item := range modelOrder.OrderItems {
		orderItemEntities = append(orderItemEntities, entity.OrderItemEntity{
			ID:        item.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	return &entity.OrderEntity{
		ID:            modelOrder.ID,
		OrderCode:     modelOrder.OrderCode,
		BuyerId:       modelOrder.BuyerId,
		Status:        modelOrder.Status,
		PaymentMethod: normalizePaymentMethod(modelOrder.PaymentMethod),
		OrderDate:     formatOrderDateTime(modelOrder.OrderDate, modelOrder.OrderTime),
		TotalAmount:   int64(modelOrder.TotalAmount),
		OrderItems:    orderItemEntities,
		Remarks:       modelOrder.Remarks,
		ShippingType:  entity.ShippingType(modelOrder.ShippingType),
		ShippingFee:   int64(modelOrder.ShippingFee),
	}, nil
}

func formatOrderDateTime(orderDate time.Time, orderTime string) string {
	if orderTime == "" {
		orderTime = "00:00:00"
	}
	return orderDate.Format("2006-01-02") + " " + orderTime
}

// UpdateStatus implements [NewOrderRepositoryInterface].
func (o *orderRepository) UpdateStatus(ctx context.Context, req entity.OrderEntity) (int64, string, string, error) {
	modelOrder := model.Order{}
	if err := o.db.Select("id", "order_code", "status", "buyer_id", "remarks").Where("id = ?", req.ID).First(&modelOrder).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := errors.New("404")
			log.Errorf("[OrderRepository-1] UpdateStatus: Order not found")
			return 0, "", "", err
		}
		log.Errorf("[OrderRepository-2] UpdateStatus: %v", err)
		return 0, "", "", err
	}

	if !canTransitionOrderStatus(modelOrder.Status, req.Status) {
		log.Infof("[OrderRepository-3] UpdateStatus: Invalid status transition")
		return 0, "", "", errors.New("400")
	}

	updatedAt := time.Now()
	updatedBy := req.UpdatedBy
	if err := o.db.Model(&modelOrder).Updates(map[string]interface{}{
		"status":     req.Status,
		"remarks":    req.Remarks,
		"updated_by": updatedBy,
		"updated_at": updatedAt,
	}).Error; err != nil {
		log.Errorf("[OrderRepository-7] UpdateStatus: %v", err)
		return 0, "", "", err
	}

	return modelOrder.BuyerId, req.Status, modelOrder.OrderCode, nil
}

func canTransitionOrderStatus(currentStatus, nextStatus string) bool {
	switch currentStatus {
	case entity.OrderStatusPending:
		return nextStatus == entity.OrderStatusConfirmed || nextStatus == entity.OrderStatusCancelled
	case entity.OrderStatusConfirmed:
		return nextStatus == entity.OrderStatusProcess || nextStatus == entity.OrderStatusCancelled
	case entity.OrderStatusProcess:
		return nextStatus == entity.OrderStatusSending || nextStatus == entity.OrderStatusCancelled
	case entity.OrderStatusSending:
		return nextStatus == entity.OrderStatusDone || nextStatus == entity.OrderStatusCancelled
	default:
		return false
	}
}

func normalizePaymentMethod(paymentMethod string) string {
	switch strings.ToUpper(strings.TrimSpace(paymentMethod)) {
	case "COD", "CASH":
		return "CASH"
	case "MIDTRANS", "DEBIT":
		return "DEBIT"
	default:
		return strings.ToUpper(strings.TrimSpace(paymentMethod))
	}
}

func NewOrderRepository(db *gorm.DB) OrderRepositoryInterface {
	return &orderRepository{
		db: db,
	}
}
