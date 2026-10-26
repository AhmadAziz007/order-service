package service

import (
	"context"
	"ecommerce/order-service/config"
	httpclient "ecommerce/order-service/internal/adapter/http_client"
	"ecommerce/order-service/internal/adapter/message"
	"ecommerce/order-service/internal/adapter/repository"
	"ecommerce/order-service/internal/core/domain/entity"
	"ecommerce/order-service/utils"
	"ecommerce/order-service/utils/conv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/gommon/log"
)

type OrderServiceInterface interface {
	GetAll(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error)
	GetByID(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error)
	CreateOrder(ctx context.Context, req entity.OrderEntity, accessToken string) ([]entity.CreatedOrderEntity, error)
	UpdateStatus(ctx context.Context, req entity.OrderEntity, accessToken string) error
	GetAllCustomer(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error)
	GetDetailCustomer(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error)
	DeleteByID(ctx context.Context, orderID int64) error
	ConfirmInventoryOnPaymentSuccess(ctx context.Context, orderCode string) error
	ExpirePayment(ctx context.Context, orderCode string) error
	GetOrderByOrderCode(ctx context.Context, orderCode, accessToken string) (*entity.OrderEntity, error)
	GetPublicOrderIDByOrderCode(ctx context.Context, orderCode string) (int64, error)
	HttpClientUserService(userID int64, accessToken string, isCustomer bool) (*entity.CustomerResponseEntity, error)
	HttpClientProductService(productID int64, accessToken string, isCustomer bool) (*entity.ProductResponseEntity, error)
}

type orderService struct {
	repo              repository.OrderRepositoryInterface
	cfg               *config.Config
	httpClient        httpclient.HttpClient
	publisherRabbitMQ message.PublishRabbitMQInterface
	elasticRepo       repository.ElasticRepositoryInterface
}

// HttpClientProductService implements [OrderServiceInterface].
func (o *orderService) HttpClientProductService(productID int64, accessToken string, isCustomer bool) (*entity.ProductResponseEntity, error) {
	baseUrlProduct := fmt.Sprintf("%s/%s", o.cfg.App.ProductServiceUrl, "admin/products/"+strconv.FormatInt(productID, 10))
	if isCustomer {
		baseUrlProduct = fmt.Sprintf("%s/%s", o.cfg.App.ProductServiceUrl, "products/home/"+strconv.FormatInt(productID, 10))
	}
	header := map[string]string{
		"Authorization":         "Bearer " + accessToken,
		"Accept":                "application/json",
		"X-API-Gateway":         "true",
		"X-API-Gateway-Version": "order-service",
	}
	if o.cfg.App.GatewaySecretKey != "" {
		header["X-Gateway-Secret"] = o.cfg.App.GatewaySecretKey
	}
	dataProduct, err := o.httpClient.CallURL("GET", baseUrlProduct, header, nil)
	if err != nil {
		log.Errorf("[OrderService-1] httpClientProductService: %v", err)
		return nil, err
	}

	defer dataProduct.Body.Close()
	if dataProduct.StatusCode < 200 || dataProduct.StatusCode >= 300 {
		return nil, fmt.Errorf("product-service returned HTTP %d", dataProduct.StatusCode)
	}

	body, err := io.ReadAll(dataProduct.Body)
	if err != nil {
		log.Errorf("[OrderService-2] httpClientProductService: %v", err)
		return nil, err
	}

	var productResponse entity.ProductHttpClientResponse
	err = json.Unmarshal(body, &productResponse)
	if err != nil {
		log.Errorf("[OrderService-3] httpClientProductService: %v", err)
		return nil, err
	}

	return &productResponse.Data, nil
}

// HttpClientUserService implements [OrderServiceInterface].
func (o *orderService) HttpClientUserService(userID int64, accessToken string, isCustomer bool) (*entity.CustomerResponseEntity, error) {
	baseUrlUser := fmt.Sprintf("%s/%s", o.cfg.App.UserServiceUrl, "admin/customers/"+strconv.FormatInt(userID, 10))
	if isCustomer {
		baseUrlUser = fmt.Sprintf("%s/%s", o.cfg.App.UserServiceUrl, "auth/customers/"+strconv.FormatInt(userID, 10))
	}
	header := map[string]string{
		"Authorization":         "Bearer " + accessToken,
		"Accept":                "application/json",
		"X-API-Gateway":         "true",
		"X-API-Gateway-Version": "order-service",
	}
	if o.cfg.App.GatewaySecretKey != "" {
		header["X-Gateway-Secret"] = o.cfg.App.GatewaySecretKey
	}
	dataUser, err := o.httpClient.CallURL("GET", baseUrlUser, header, nil)
	if err != nil {
		log.Errorf("[OrderService-1] httpClientUserService: %v", err)
		return nil, err
	}

	defer dataUser.Body.Close()
	if dataUser.StatusCode < 200 || dataUser.StatusCode >= 300 {
		return nil, fmt.Errorf("user-service returned HTTP %d", dataUser.StatusCode)
	}

	body, err := io.ReadAll(dataUser.Body)
	if err != nil {
		log.Errorf("[OrderService-2] httpClientUserService: %v", err)
		return nil, err
	}

	var userResponse entity.UserHttpClientResponse
	err = json.Unmarshal(body, &userResponse)
	if err != nil {
		log.Errorf("[OrderService-3] httpClientUserService: %v", err)
		return nil, err
	}

	return &userResponse.Data, nil
}

// CreateOrder implements [OrderServiceInterface].
func (o *orderService) CreateOrder(ctx context.Context, req entity.OrderEntity, accessToken string) ([]entity.CreatedOrderEntity, error) {
	req.PaymentMethod = normalizePaymentMethod(req.PaymentMethod)
	if req.PaymentMethod != "CASH" && req.PaymentMethod != "DEBIT" {
		return nil, errors.New("payment_method must be CASH or DEBIT")
	}

	var token map[string]interface{}
	if err := json.Unmarshal([]byte(accessToken), &token); err != nil {
		return nil, err
	}
	serviceToken, ok := token["token"].(string)
	if !ok || serviceToken == "" {
		return nil, errors.New("invalid user token")
	}
	userID, ok := token["user_id"].(float64)
	if !ok || userID <= 0 {
		return nil, errors.New("invalid user id in session")
	}
	if len(req.OrderItems) == 0 {
		return nil, errors.New("order_details must contain at least one product")
	}
	if req.ShippingType != entity.ShippingTypeDelivery && req.ShippingType != entity.ShippingTypePickup {
		return nil, errors.New("shipping_type must be DELIVERY or PICKUP")
	}
	roleName, _ := token["role_name"].(string)
	isCustomer := roleName == "Customer"

	productPrices := make([]int64, len(req.OrderItems))
	var subtotal int64
	for i, item := range req.OrderItems {
		if item.ProductID <= 0 || item.Quantity <= 0 {
			return nil, errors.New("product_id and quantity must be greater than zero")
		}
		product, err := o.HttpClientProductService(item.ProductID, serviceToken, isCustomer)
		if err != nil {
			log.Errorf("[OrderService-1] CreateOrder: %v", err)
			return nil, err
		}
		if product.ProductStatus != "" && product.ProductStatus != "APPROVED" {
			return nil, fmt.Errorf("product %d is not approved", item.ProductID)
		}
		price := int64(math.Round(product.SalePrice))
		productPrices[i] = price
		subtotal += price * item.Quantity
	}

	shippingFee := int64(0)
	switch req.ShippingType {
	case entity.ShippingTypeDelivery:
		lat, err := strconv.ParseFloat(req.Latitude, 64)
		if err != nil {
			return nil, errors.New("invalid latitude")
		}
		lng, err := strconv.ParseFloat(req.Longitude, 64)
		if err != nil {
			return nil, errors.New("invalid longitude")
		}
		latRef, err := strconv.ParseFloat(o.cfg.App.LatitudeRef, 64)
		if err != nil {
			return nil, errors.New("invalid configured reference latitude")
		}
		lngRef, err := strconv.ParseFloat(o.cfg.App.LongitudeRef, 64)
		if err != nil {
			return nil, errors.New("invalid configured reference longitude")
		}
		distance := conv.HaversineDistance(latRef, lngRef, lat, lng)
		shippingFee = int64(math.Ceil(distance)) * 5000
	case entity.ShippingTypePickup:
	default:
		return nil, errors.New("shipping_type must be DELIVERY or PICKUP")
	}

	orders := make([]entity.OrderEntity, len(req.OrderItems))
	remainingShippingFee := shippingFee
	for i, item := range req.OrderItems {
		itemShippingFee := int64(0)
		if i == len(req.OrderItems)-1 {
			itemShippingFee = remainingShippingFee
		} else if subtotal > 0 {
			itemShippingFee = shippingFee * productPrices[i] * item.Quantity / subtotal
			remainingShippingFee -= itemShippingFee
		}
		orders[i] = entity.OrderEntity{
			OrderCode:     conv.GenerateOrderCode(),
			BuyerId:       req.BuyerId,
			OrderDate:     req.OrderDate,
			OrderTime:     req.OrderTime,
			Status:        entity.OrderStatusPending,
			TotalAmount:   productPrices[i]*item.Quantity + itemShippingFee,
			PaymentMethod: req.PaymentMethod,
			ShippingType:  req.ShippingType,
			ShippingFee:   itemShippingFee,
			Remarks:       req.Remarks,
			CreatedBy:     int64(userID),
			OrderItems:    []entity.OrderItemEntity{item},
		}
	}

	reservedOrderCodes := make([]string, 0, len(orders))
	for _, order := range orders {
		item := order.OrderItems[0]
		if err := o.reserveInventory(ctx, order.OrderCode, item.ProductID, item.Quantity); err != nil {
			_ = o.releaseInventory(ctx, order.OrderCode)
			for _, reservedOrderCode := range reservedOrderCodes {
				if releaseErr := o.releaseInventory(ctx, reservedOrderCode); releaseErr != nil {
					log.Errorf("[OrderService] failed to rollback inventory reservation %s: %v", reservedOrderCode, releaseErr)
				}
			}
			return nil, fmt.Errorf("unable to reserve stock for product %d: %w", item.ProductID, err)
		}
		reservedOrderCodes = append(reservedOrderCodes, order.OrderCode)
	}

	orderIDs, err := o.repo.CreateOrders(ctx, orders)
	if err != nil {
		for _, reservedOrderCode := range reservedOrderCodes {
			if releaseErr := o.releaseInventory(ctx, reservedOrderCode); releaseErr != nil {
				log.Errorf("[OrderService] failed to rollback inventory reservation %s: %v", reservedOrderCode, releaseErr)
			}
		}
		log.Errorf("[OrderService-1] CreateOrder: %v", err)
		return nil, err
	}

	createdOrders := make([]entity.CreatedOrderEntity, 0, len(orders))
	for i, orderID := range orderIDs {
		createdOrders = append(createdOrders, entity.CreatedOrderEntity{
			OrderID:   orderID,
			OrderCode: orders[i].OrderCode,
		})
		resultData, getErr := o.GetByID(ctx, orderID, accessToken)
		if getErr == nil && resultData != nil {
			if publishErr := o.publisherRabbitMQ.PublishOrderToQueue(*resultData); publishErr != nil {
				log.Errorf("[OrderService-3] CreateOrder: %v", publishErr)
			}
		} else {
			log.Errorf("[OrderService-2] CreateOrder: %v", getErr)
		}
	}
	return createdOrders, nil
}

func (o *orderService) ConfirmInventoryOnPaymentSuccess(ctx context.Context, orderCode string) error {
	order, err := o.repo.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		return err
	}
	if err := o.repo.MarkPaymentSucceeded(ctx, orderCode); err != nil {
		return err
	}
	if err := o.commitInventory(ctx, orderCode); err != nil {
		return err
	}
	return o.clearBuyerCart(ctx, order.BuyerId)
}

func (o *orderService) ExpirePayment(ctx context.Context, orderCode string) error {
	buyerID, expired, err := o.repo.CancelExpiredPayment(ctx, orderCode)
	if err != nil || !expired {
		return err
	}
	if err := o.releaseInventory(ctx, orderCode); err != nil {
		return err
	}
	return o.clearBuyerCart(ctx, buyerID)
}

func (o *orderService) reserveInventory(ctx context.Context, orderCode string, productID, quantity int64) error {
	return o.callInventoryService(ctx, "reserve", map[string]interface{}{
		"order_code": orderCode,
		"product_id": productID,
		"quantity":   quantity,
	})
}

func (o *orderService) commitInventory(ctx context.Context, orderCode string) error {
	return o.callInventoryService(ctx, "commit", map[string]string{"order_code": orderCode})
}

func (o *orderService) releaseInventory(ctx context.Context, orderCode string) error {
	return o.callInventoryService(ctx, "release", map[string]string{"order_code": orderCode})
}

func (o *orderService) callInventoryService(ctx context.Context, action string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	header := map[string]string{
		"Content-Type":          "application/json",
		"X-API-Gateway":         "true",
		"X-API-Gateway-Version": "order-service",
	}
	if o.cfg.App.GatewaySecretKey != "" {
		header["X-Gateway-Secret"] = o.cfg.App.GatewaySecretKey
	}
	url := fmt.Sprintf("%s/internal/inventory/%s", o.cfg.App.ProductServiceUrl, action)
	resp, err := o.httpClient.CallURL("POST", url, header, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("product-service inventory %s returned HTTP %d", action, resp.StatusCode)
		}
		return fmt.Errorf("product-service inventory %s returned HTTP %d: %s", action, resp.StatusCode, string(responseBody))
	}
	return nil
}

func (o *orderService) clearBuyerCart(ctx context.Context, buyerID int64) error {
	body, err := json.Marshal(map[string]int64{"buyer_id": buyerID})
	if err != nil {
		return err
	}
	header := map[string]string{
		"Content-Type":          "application/json",
		"X-API-Gateway":         "true",
		"X-API-Gateway-Version": "order-service",
	}
	if o.cfg.App.GatewaySecretKey != "" {
		header["X-Gateway-Secret"] = o.cfg.App.GatewaySecretKey
	}
	resp, err := o.httpClient.CallURL("POST", fmt.Sprintf("%s/internal/cart/clear", o.cfg.App.ProductServiceUrl), header, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("product-service cart clear returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// DeleteByID implements [OrderServiceInterface].
func (o *orderService) DeleteByID(ctx context.Context, orderID int64) error {
	order, getErr := o.repo.GetByID(ctx, orderID)
	if getErr == nil && order != nil {
		if err := o.releaseInventory(ctx, order.OrderCode); err != nil {
			return err
		}
	} else if getErr != nil && getErr.Error() != "404" {
		return getErr
	}
	err := o.repo.DeleteOrder(ctx, orderID)
	if err != nil && err.Error() != "404" {
		log.Errorf("[OrderService-1] DeleteByID: %v", err)
		return err
	}

	if err := o.publisherRabbitMQ.PublishDeleteOrderFromQueue(orderID); err != nil {
		log.Errorf("[OrderService-2] DeleteByID: %v", err)
		return err
	}

	return nil
}

// GetAll implements [OrderServiceInterface].
func (o *orderService) GetAll(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error) {
	// PostgreSQL is the source of truth for order status. Elasticsearch is
	// updated asynchronously, so reading it first can show an old status in
	// the list while the detail page already shows the current value.
	results, count, total, err := o.repo.GetAll(ctx, queryString)
	if err != nil {
		log.Errorf("[OrderService-1] GetAll: %v", err)
		return nil, 0, 0, err
	}
	normalizeOrderPaymentMethods(results)

	var token map[string]interface{}
	err = json.Unmarshal([]byte(accessToken), &token)
	if err != nil {
		log.Errorf("[OrderService-3] GetAll: %v", err)
		return nil, 0, 0, err
	}

	isCustomer := token["role_name"].(string) == "Customer"

	for key, val := range results {

		userResponse, err := o.HttpClientUserService(val.BuyerId, token["token"].(string), isCustomer)
		if err != nil {
			log.Errorf("[OrderService-4] GetAll: %v", err)
			return nil, 0, 0, err
		}
		results[key].BuyerName = userResponse.Name

		for key2, res := range val.OrderItems {

			productResponse, err := o.HttpClientProductService(res.ProductID, token["token"].(string), isCustomer)
			if err != nil {
				log.Errorf("[OrderService-5] GetAll: %v", err)
				return nil, 0, 0, err
			}

			results[key].OrderItems[key2].ProductImage = productResponse.ProductImage
			results[key].OrderItems[key2].ProductName = productResponse.ProductName
			results[key].OrderItems[key2].Price = int64(math.Round(productResponse.SalePrice))
		}
	}

	return results, count, total, nil
}

// GetAllCustomer implements [OrderServiceInterface].
func (o *orderService) GetAllCustomer(ctx context.Context, queryString entity.QueryStringEntity, accessToken string) ([]entity.OrderEntity, int64, int64, error) {
	// Use the transactional database here as well so customers do not see a
	// stale status while Elasticsearch catches up with the order event.
	results, count, total, err := o.repo.GetAll(ctx, queryString)
	if err != nil {
		log.Errorf("[OrderService-1] GetAllCustomer: %v", err)
		return nil, 0, 0, err
	}
	normalizeOrderPaymentMethods(results)

	var token map[string]interface{}
	err = json.Unmarshal([]byte(accessToken), &token)
	if err != nil {
		log.Errorf("[OrderService-3] GetAllCustomer: %v", err)
		return nil, 0, 0, err
	}

	for key, val := range results {
		userResponse, err := o.HttpClientUserService(val.BuyerId, token["token"].(string), true)
		if err != nil {
			log.Errorf("[OrderService-4] GetAllCustomer: %v", err)
			return nil, 0, 0, err
		}

		results[key].BuyerName = userResponse.Name
		results[key].BuyerEmail = userResponse.Email
		results[key].BuyerPhone = userResponse.Phone
		results[key].BuyerAddress = userResponse.Address
		results[key].BuyerLat = userResponse.Lat
		results[key].BuyerLng = userResponse.Lng
		results[key].BuyerPhoto = userResponse.Photo

		for key2, res := range val.OrderItems {

			productResponse, err := o.HttpClientProductService(res.ProductID, token["token"].(string), true)
			if err != nil {
				log.Errorf("[OrderService-5] GetAllCustomer: %v", err)
				return nil, 0, 0, err
			}

			results[key].OrderItems[key2].ProductImage = productResponse.ProductImage
			results[key].OrderItems[key2].ProductName = productResponse.ProductName
			results[key].OrderItems[key2].Price = int64(math.Round(productResponse.SalePrice))
			results[key].OrderItems[key2].Quantity = res.Quantity
			results[key].OrderItems[key2].ProductUnit = productResponse.Unit
			results[key].OrderItems[key2].ProductWeight = int64(productResponse.Weight)
		}
	}

	return results, count, total, nil
}

// GetByID implements [OrderServiceInterface].
func (o *orderService) GetByID(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error) {
	result, err := o.repo.GetByID(ctx, orderID)
	if err != nil {
		log.Errorf("[OrderService-1] GetByID: %v", err)
		return nil, err
	}
	result.PaymentMethod = normalizePaymentMethod(result.PaymentMethod)

	var token map[string]interface{}
	err = json.Unmarshal([]byte(accessToken), &token)
	if err != nil {
		log.Errorf("[OrderService-2] GetByID: %v", err)
		return nil, err
	}

	isCustomer := token["role_name"].(string) == "Customer"

	userResponse, err := o.HttpClientUserService(result.BuyerId, token["token"].(string), isCustomer)
	if err != nil {
		log.Errorf("[OrderService-2] GetByID: %v", err)
		return nil, err
	}

	result.BuyerName = userResponse.Name
	result.BuyerEmail = userResponse.Email
	result.BuyerPhone = userResponse.Phone
	result.BuyerAddress = userResponse.Address
	result.BuyerLat = userResponse.Lat
	result.BuyerLng = userResponse.Lng
	result.BuyerPhoto = userResponse.Photo

	for key, val := range result.OrderItems {
		productResponse, err := o.HttpClientProductService(val.ProductID, token["token"].(string), isCustomer)
		if err != nil {
			log.Errorf("[OrderService-3] GetByID: %v", err)
			return nil, err
		}

		result.OrderItems[key].ProductImage = productResponse.ProductImage
		result.OrderItems[key].ProductName = productResponse.ProductName
		result.OrderItems[key].Price = int64(math.Round(productResponse.SalePrice))
		result.OrderItems[key].ProductWeight = int64(productResponse.Weight)
		result.OrderItems[key].ProductUnit = productResponse.Unit
	}

	return result, nil
}

// GetDetailCustomer implements [OrderServiceInterface].
func (o *orderService) GetDetailCustomer(ctx context.Context, orderID int64, accessToken string) (*entity.OrderEntity, error) {
	result, err := o.repo.GetByID(ctx, orderID)
	if err != nil {
		log.Errorf("[OrderService-1] GetByID: %v", err)
		return nil, err
	}
	result.PaymentMethod = normalizePaymentMethod(result.PaymentMethod)

	var token map[string]interface{}
	err = json.Unmarshal([]byte(accessToken), &token)
	if err != nil {
		log.Errorf("[OrderService-2] GetByID: %v", err)
		return nil, err
	}

	roleName, _ := token["role_name"].(string)
	userID, _ := token["user_id"].(float64)
	if roleName == "Customer" && int64(userID) != result.BuyerId {
		log.Errorf("[OrderService-2] GetDetailCustomer: customer cannot access another customer's order")
		return nil, errors.New("403")
	}

	userResponse, err := o.HttpClientUserService(result.BuyerId, token["token"].(string), roleName == "Customer")
	if err != nil {
		log.Errorf("[OrderService-3] GetByID: %v", err)
		return nil, err
	}

	result.BuyerName = userResponse.Name
	result.BuyerEmail = userResponse.Email
	result.BuyerPhone = userResponse.Phone
	result.BuyerAddress = userResponse.Address
	result.BuyerLat = userResponse.Lat
	result.BuyerLng = userResponse.Lng
	result.BuyerPhoto = userResponse.Photo

	for key, val := range result.OrderItems {
		productResponse, err := o.HttpClientProductService(val.ProductID, token["token"].(string), true)
		if err != nil {
			log.Errorf("[OrderService-4] GetByID: %v", err)
			return nil, err
		}

		result.OrderItems[key].ProductImage = productResponse.ProductImage
		if productResponse.Child != nil {
			result.OrderItems[key].ProductImage = productResponse.Child[0].Image
		}
		result.OrderItems[key].ProductName = productResponse.ProductName
		result.OrderItems[key].Price = int64(math.Round(productResponse.SalePrice))
		result.OrderItems[key].ProductWeight = int64(productResponse.Weight)
		result.OrderItems[key].ProductUnit = productResponse.Unit
	}

	return result, nil
}

// GetOrderByOrderCode implements [OrderServiceInterface].
func (o *orderService) GetOrderByOrderCode(ctx context.Context, orderCode string, accessToken string) (*entity.OrderEntity, error) {
	result, err := o.repo.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		log.Errorf("[OrderService-1] GetOrderByOrderCode: %v", err)
		return nil, err
	}
	result.PaymentMethod = normalizePaymentMethod(result.PaymentMethod)

	var token map[string]interface{}
	err = json.Unmarshal([]byte(accessToken), &token)
	if err != nil {
		log.Errorf("[OrderService-2] GetOrderByOrderCode: %v", err)
		return nil, err
	}

	roleName, _ := token["role_name"].(string)
	userID, _ := token["user_id"].(float64)
	if roleName != "Super Admin" && int64(userID) != result.BuyerId {
		log.Errorf("[OrderService-2] GetOrderByOrderCode: customer cannot access another customer's order")
		return nil, errors.New("403")
	}

	isCustomer := roleName == "Customer"

	userResponse, err := o.HttpClientUserService(result.BuyerId, token["token"].(string), isCustomer)
	if err != nil {
		log.Errorf("[OrderService-3] GetOrderByOrderCode: %v", err)
		return nil, err
	}

	result.BuyerName = userResponse.Name
	result.BuyerEmail = userResponse.Email
	result.BuyerPhone = userResponse.Phone
	result.BuyerAddress = userResponse.Address
	result.BuyerLat = userResponse.Lat
	result.BuyerLng = userResponse.Lng
	result.BuyerPhoto = userResponse.Photo

	for key, val := range result.OrderItems {
		productResponse, err := o.HttpClientProductService(val.ProductID, token["token"].(string), isCustomer)
		if err != nil {
			log.Errorf("[OrderService-4] GetOrderByOrderCode: %v", err)
			return nil, err
		}

		result.OrderItems[key].ProductImage = productResponse.ProductImage
		result.OrderItems[key].ProductName = productResponse.ProductName
		result.OrderItems[key].Price = int64(math.Round(productResponse.SalePrice))
	}

	return result, nil
}

// GetPublicOrderIDByOrderCode implements [OrderServiceInterface].
func (o *orderService) GetPublicOrderIDByOrderCode(ctx context.Context, orderCode string) (int64, error) {
	result, err := o.repo.GetOrderByOrderCode(ctx, orderCode)
	if err != nil {
		log.Errorf("[OrderService-1] GetPublicOrderIDByOrderCode: %v", err)
		return 0, err
	}

	return result.ID, nil
}

// UpdateStatus implements [OrderServiceInterface].
func (o *orderService) UpdateStatus(ctx context.Context, req entity.OrderEntity, accessToken string) error {
	var session entity.JwtUserData
	if err := json.Unmarshal([]byte(accessToken), &session); err != nil {
		log.Errorf("[OrderService-1] UpdateStatus: %v", err)
		return err
	}
	if session.UserID <= 0 {
		return errors.New("invalid user id in session")
	}
	req.UpdatedBy = session.UserID

	buyerID, statusOrder, orderCode, err := o.repo.UpdateStatus(ctx, req)
	if err != nil {
		log.Errorf("[OrderService-1] UpdateStatus: %v", err)
		return err
	}
	if req.Status == entity.OrderStatusCancelled {
		if err := o.releaseInventory(ctx, orderCode); err != nil {
			return err
		}
	}

	userResponse, err := o.HttpClientUserService(buyerID, session.Token, false)
	if err != nil {
		log.Errorf("[OrderService-3] UpdateStatus: %v", err)
		return err
	}

	message := fmt.Sprintf("Hello,\n\nYour order with ID %s has been updated to status: %s.\n\nThank you for shopping with us!", orderCode, statusOrder)
	go o.publisherRabbitMQ.PublishSendEmailUpdateStatus(userResponse.Email, message, o.cfg.PublisherName.EmailUpdateStatus, buyerID)
	go o.publisherRabbitMQ.PublishSendPushNotifUpdateStatus(message, utils.PUSH_NOTIF, buyerID)
	go o.publisherRabbitMQ.PublishUpdateStatus(o.cfg.PublisherName.PublisherUpdateStatus, req.ID, req.Status)

	return nil
}

func NewOrderService(repo repository.OrderRepositoryInterface, cfg *config.Config, httpClient httpclient.HttpClient, publisherRabbitMQ message.PublishRabbitMQInterface, elasticRepo repository.ElasticRepositoryInterface) OrderServiceInterface {
	return &orderService{
		repo:              repo,
		cfg:               cfg,
		httpClient:        httpClient,
		publisherRabbitMQ: publisherRabbitMQ,
		elasticRepo:       elasticRepo,
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

func normalizeOrderPaymentMethods(orders []entity.OrderEntity) {
	for index := range orders {
		orders[index].PaymentMethod = normalizePaymentMethod(orders[index].PaymentMethod)
	}
}
