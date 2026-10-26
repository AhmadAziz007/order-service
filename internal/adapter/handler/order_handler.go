package handler

import (
	"ecommerce/order-service/config"
	"ecommerce/order-service/internal/adapter"
	"ecommerce/order-service/internal/adapter/handler/request"
	"ecommerce/order-service/internal/adapter/handler/response"
	"ecommerce/order-service/internal/core/domain/entity"
	"ecommerce/order-service/internal/core/service"
	"ecommerce/order-service/utils/conv"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

type OrderHandlerInterface interface {
	GetAllAdmin(c echo.Context) error
	GetByIDAdmin(c echo.Context) error
	CreateOrder(c echo.Context) error
	UpdateStatus(c echo.Context) error
	GetAllCustomer(c echo.Context) error
	GetDetailCustomer(c echo.Context) error
	DeleteByID(c echo.Context) error
	GetOrderByOrderCode(c echo.Context) error
	GetPublicOrderByOrderCode(c echo.Context) error
	PaymentSucceeded(c echo.Context) error
	PaymentExpired(c echo.Context) error
}

type orderHandler struct {
	orderService service.OrderServiceInterface
	cfg          *config.Config
}

func (o *orderHandler) PaymentSucceeded(c echo.Context) error {
	if !o.isTrustedServiceRequest(c) {
		return c.JSON(http.StatusUnauthorized, response.ResponseError("unauthorized service request"))
	}

	var req struct {
		OrderCode string `json:"order_code"`
	}
	if err := c.Bind(&req); err != nil || req.OrderCode == "" {
		return c.JSON(http.StatusBadRequest, response.ResponseError("order_code is required"))
	}
	if err := o.orderService.ConfirmInventoryOnPaymentSuccess(c.Request().Context(), req.OrderCode); err != nil {
		log.Errorf("[OrderHandler] PaymentSucceeded: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("order not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}
	return c.JSON(http.StatusOK, response.ResponseSuccess("success", nil))
}

func (o *orderHandler) PaymentExpired(c echo.Context) error {
	if !o.isTrustedServiceRequest(c) {
		return c.JSON(http.StatusUnauthorized, response.ResponseError("unauthorized service request"))
	}
	var req struct {
		OrderCode string `json:"order_code"`
	}
	if err := c.Bind(&req); err != nil || req.OrderCode == "" {
		return c.JSON(http.StatusBadRequest, response.ResponseError("order_code is required"))
	}
	if err := o.orderService.ExpirePayment(c.Request().Context(), req.OrderCode); err != nil {
		log.Errorf("[OrderHandler] PaymentExpired: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("order not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}
	return c.JSON(http.StatusOK, response.ResponseSuccess("success", nil))
}

func (o *orderHandler) isTrustedServiceRequest(c echo.Context) bool {
	return c.Request().Header.Get("X-API-Gateway") == "true" && o.cfg.App.GatewaySecretKey != "" &&
		c.Request().Header.Get("X-Gateway-Secret") == o.cfg.App.GatewaySecretKey
}

func (o *orderHandler) GetPublicOrderByOrderCode(c echo.Context) error {
	var (
		ctx = c.Request().Context()
	)

	orderCode := c.Param("orderCode")
	if orderCode == "" {
		log.Errorf("[OrderHandler-1] GetOrderByOrderCode: %s", "orderCode not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("orderCode not found"))
	}

	order, err := o.orderService.GetPublicOrderIDByOrderCode(ctx, orderCode)
	if err != nil {
		log.Errorf("[OrderHandler-2] GetOrderByOrderCode: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	return c.JSON(http.StatusOK, response.ResponseSuccess("success", map[string]int64{
		"orderID": order,
	}))
}

func (o *orderHandler) GetOrderByOrderCode(c echo.Context) error {
	var (
		ctx       = c.Request().Context()
		respOrder = response.OrderAdminDetail{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] GetOrderByOrderCode: %s", "data token not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("data token not found"))
	}

	orderCode := c.Param("orderCode")
	if orderCode == "" {
		log.Errorf("[OrderHandler-2] GetOrderByOrderCode: %s", "orderCode not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("orderCode not found"))
	}

	order, err := o.orderService.GetOrderByOrderCode(ctx, orderCode, user)
	if err != nil {
		log.Errorf("[OrderHandler-3] GetOrderByOrderCode: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		if err.Error() == "403" {
			return c.JSON(http.StatusForbidden, response.ResponseError("customer cannot access another customer's order"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	respOrder.ID = order.ID
	respOrder.OrderCode = order.OrderCode
	respOrder.Status = order.Status
	respOrder.TotalAmount = order.TotalAmount
	respOrder.OrderDatetime = order.OrderDate
	respOrder.PaymentMethod = order.PaymentMethod
	respOrder.ShippingFee = order.ShippingFee
	respOrder.ShippingType = string(order.ShippingType)
	respOrder.Remarks = order.Remarks
	respOrder.Customer = response.CustomerOrder{
		CustomerName:    order.BuyerName,
		CustomerPhone:   order.BuyerPhone,
		CustomerAddress: order.BuyerAddress,
		CustomerEmail:   order.BuyerEmail,
		CustomerID:      order.BuyerId,
		CustomerLat:     order.BuyerLat,
		CustomerLng:     order.BuyerLng,
		CustomerPhoto:   order.BuyerPhoto,
	}

	for _, item := range order.OrderItems {
		if respOrder.ProductImage == "" {
			respOrder.ProductImage = item.ProductImage
		}
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})
	}

	return c.JSON(http.StatusOK, response.ResponseSuccess("success", respOrder))
}

func (o *orderHandler) DeleteByID(c echo.Context) error {
	var (
		ctx = c.Request().Context()
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] DeleteByID: %s", "data token not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("data token not found"))
	}

	idParams := c.Param("orderID")
	if idParams == "" {
		log.Errorf("[OrderHandler-2] DeleteByID: %s", "orderID not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("orderID not found"))
	}

	orderID, err := conv.StringToInt64(idParams)
	if err != nil {
		log.Errorf("[OrderHandler-3] DeleteByID: %v", err)
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	err = o.orderService.DeleteByID(ctx, orderID)
	if err != nil {
		log.Errorf("[OrderHandler-4] DeleteByID: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	return c.JSON(http.StatusOK, response.ResponseSuccess("success", nil))
}

func (o *orderHandler) GetDetailCustomer(c echo.Context) error {
	var (
		ctx       = c.Request().Context()
		respOrder = response.OrderAdminDetail{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] GetDetailCustomer: %s", "data token not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("data token not found"))
	}

	orderIDStr := c.Param("orderID")
	if orderIDStr == "" {
		log.Errorf("[OrderHandler-2] GetDetailCustomer: %s", "orderID not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("orderID not found"))
	}

	orderID, err := conv.StringToInt64(orderIDStr)
	if err != nil {
		log.Errorf("[OrderHandler-3] GetDetailCustomer: %v", err)
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	order, err := o.orderService.GetDetailCustomer(ctx, orderID, user)
	if err != nil {
		log.Errorf("[OrderHandler-4] GetDetailCustomer: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		if err.Error() == "403" {
			return c.JSON(http.StatusForbidden, response.ResponseError("customer cannot access another customer's order"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	respOrder.ID = order.ID
	respOrder.OrderCode = order.OrderCode
	respOrder.Status = order.Status
	respOrder.TotalAmount = order.TotalAmount
	respOrder.OrderDatetime = order.OrderDate
	respOrder.PaymentMethod = order.PaymentMethod
	respOrder.ShippingFee = order.ShippingFee
	respOrder.ShippingType = string(order.ShippingType)
	respOrder.Remarks = order.Remarks
	respOrder.Customer = response.CustomerOrder{
		CustomerName:    order.BuyerName,
		CustomerPhone:   order.BuyerPhone,
		CustomerAddress: order.BuyerAddress,
		CustomerEmail:   order.BuyerEmail,
		CustomerID:      order.BuyerId,
	}

	for _, item := range order.OrderItems {
		if respOrder.ProductImage == "" {
			respOrder.ProductImage = item.ProductImage
		}
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})
	}

	return c.JSON(http.StatusOK, response.ResponseSuccess("success", respOrder))
}

// GetAllCustomer implements OrderHandlerInterface.
func (o *orderHandler) GetAllCustomer(c echo.Context) error {
	var (
		ctx         = c.Request().Context()
		respOrders  = []response.OrderCustomerList{}
		jwtUserData = entity.JwtUserData{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] GetAllCustomer: %s", "data token not found")
		return c.JSON(http.StatusUnauthorized, response.ResponseError("data token not found"))
	}

	err := json.Unmarshal([]byte(user), &jwtUserData)
	if err != nil {
		log.Errorf("[OrderHandler-2] GetAllCustomer: %v", err)
		return c.JSON(http.StatusBadRequest, response.ResponseError(err.Error()))
	}

	userID := jwtUserData.UserID

	search := c.QueryParam("search")
	var page int64 = 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		page, _ = conv.StringToInt64(pageStr)
		if page <= 0 {
			page = 1
		}
	}

	var perPage int64 = 10
	perPageStr := c.QueryParam("perPage")
	if perPageStr == "" {
		perPageStr = c.QueryParam("limit")
	}
	if perPageStr != "" {
		perPage, _ = conv.StringToInt64(perPageStr)
		if perPage <= 0 {
			perPage = 10
		}
	}

	status := ""
	if statusStr := c.QueryParam("status"); statusStr != "" {
		status = statusStr
	}

	reqEntity := entity.QueryStringEntity{
		Search:  search,
		Status:  status,
		Page:    page,
		Limit:   perPage,
		BuyerID: userID,
	}

	results, totalData, totalPage, err := o.orderService.GetAllCustomer(ctx, reqEntity, user)
	if err != nil {
		log.Errorf("[OrderHandler-3] GetAllCustomer: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	for _, result := range results {
		for _, item := range result.OrderItems {
			respOrders = append(respOrders, response.OrderCustomerList{
				ID:            result.ID,
				OrderCode:     result.OrderCode,
				Status:        result.Status,
				PaymentMethod: result.PaymentMethod,
				ProductName:   item.ProductName,
				TotalAmount:   result.TotalAmount,
				ProductPrice:  item.Price,
				ProductImage:  item.ProductImage,
				Weight:        item.ProductWeight,
				Unit:          item.ProductUnit,
				Quantity:      item.Quantity,
				OrderDateTime: result.OrderDate,
			})
		}
	}

	return c.JSON(http.StatusOK, response.ResponseSuccessWithPagination("success", respOrders, page, totalData, totalPage, perPage))
}

// UpdateStatus implements OrderHandlerInterface.
func (o *orderHandler) UpdateStatus(c echo.Context) error {
	var (
		ctx = c.Request().Context()
		req = request.OrderUpdateStatusRequest{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] UpdateStatus: %s", "data token not found")
		return c.JSON(http.StatusUnauthorized, response.ResponseError("data token not found"))
	}

	if err := c.Bind(&req); err != nil {
		log.Errorf("[OrderHandler-2] UpdateStatus: %v", err)
		return c.JSON(http.StatusBadRequest, response.ResponseError(err.Error()))
	}

	if err := c.Validate(&req); err != nil {
		log.Errorf("[OrderHandler-3] UpdateStatus: %v", err)
		return c.JSON(http.StatusUnprocessableEntity, response.ResponseError(err.Error()))
	}

	idParams := c.Param("orderID")
	if idParams == "" {
		log.Errorf("[OrderHandler-4] UpdateStatus: %s", "orderID not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("orderID not found"))
	}

	orderID, err := conv.StringToInt64(idParams)
	if err != nil {
		log.Errorf("[OrderHandler-5] UpdateStatus: %v", err)
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	reqEntity := entity.OrderEntity{
		Remarks: req.Remarks,
		Status:  req.Status,
		ID:      orderID,
	}

	err = o.orderService.UpdateStatus(ctx, reqEntity, user)
	if err != nil {
		log.Errorf("[OrderHandler-6] UpdateStatus: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}

		if err.Error() == "400" {
			return c.JSON(http.StatusBadRequest, response.ResponseError("Invalid status transition"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	return c.JSON(http.StatusOK, response.ResponseSuccess("success", nil))
}

// GetAllAdmin implements OrderHandlerInterface.
func (o *orderHandler) CreateOrder(c echo.Context) error {
	var (
		ctx = c.Request().Context()
		req = request.CreateOrderRequest{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] CreateOrder: %s", "data token not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("data token not found"))
	}

	if err := c.Bind(&req); err != nil {
		log.Errorf("[OrderHandler-2] CreateOrder: %v", err)
		return c.JSON(http.StatusBadRequest, response.ResponseError(err.Error()))
	}

	if err := c.Validate(&req); err != nil {
		log.Errorf("[OrderHandler-3] CreateOrder: %v", err)
		return c.JSON(http.StatusUnprocessableEntity, response.ResponseError(err.Error()))
	}

	reqEntity := entity.OrderEntity{
		BuyerId:       req.BuyerID,
		OrderDate:     req.OrderDate,
		ShippingType:  entity.ShippingType(req.ShippingType),
		PaymentMethod: req.PaymentMethod,
		Remarks:       req.Remarks,
		OrderTime:     req.OrderTime,
		Latitude:      c.QueryParam("lat"),
		Longitude:     c.QueryParam("lng"),
	}

	orderDetails := []entity.OrderItemEntity{}
	for _, val := range req.OrderDetails {
		orderDetails = append(orderDetails, entity.OrderItemEntity{
			ProductID: val.ProductID,
			Quantity:  val.Quantity,
		})
	}

	reqEntity.OrderItems = orderDetails

	createdOrders, err := o.orderService.CreateOrder(ctx, reqEntity, user)
	if err != nil {
		log.Errorf("[OrderHandler-4] CreateOrder: %v", err)
		if strings.Contains(err.Error(), "unable to reserve stock") {
			return c.JSON(http.StatusConflict, response.ResponseError(err.Error()))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	return c.JSON(http.StatusCreated, response.ResponseSuccess("success", map[string]interface{}{
		"orders": createdOrders,
	}))
}

// GetAllAdmin implements OrderHandlerInterface.
func (o *orderHandler) GetByIDAdmin(c echo.Context) error {
	var (
		ctx       = c.Request().Context()
		respOrder = response.OrderAdminDetail{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] GetByIDAdmin: %s", "data token not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("data token not found"))
	}

	orderIDStr := c.Param("orderID")
	if orderIDStr == "" {
		log.Errorf("[OrderHandler-2] GetByIDAdmin: %s", "orderID not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("orderID not found"))
	}

	orderID, err := conv.StringToInt64(orderIDStr)
	if err != nil {
		log.Errorf("[OrderHandler-3] GetByIDAdmin: %v", err)
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	order, err := o.orderService.GetByID(ctx, orderID, user)
	if err != nil {
		log.Errorf("[OrderHandler-4] GetByIDAdmin: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	respOrder.ID = order.ID
	respOrder.OrderCode = order.OrderCode
	respOrder.Status = order.Status
	respOrder.TotalAmount = order.TotalAmount
	respOrder.OrderDatetime = order.OrderDate
	respOrder.PaymentMethod = order.PaymentMethod
	respOrder.ShippingFee = order.ShippingFee
	respOrder.ShippingType = string(order.ShippingType)
	respOrder.Remarks = order.Remarks
	respOrder.Customer = response.CustomerOrder{
		CustomerName:    order.BuyerName,
		CustomerPhone:   order.BuyerPhone,
		CustomerAddress: order.BuyerAddress,
		CustomerEmail:   order.BuyerEmail,
		CustomerID:      order.BuyerId,
	}

	for _, item := range order.OrderItems {
		if respOrder.ProductImage == "" {
			respOrder.ProductImage = item.ProductImage
		}
		respOrder.OrderDetail = append(respOrder.OrderDetail, response.OrderDetail{
			ProductName:  item.ProductName,
			ProductImage: item.ProductImage,
			ProductPrice: item.Price,
			Quantity:     item.Quantity,
		})
	}

	return c.JSON(http.StatusOK, response.ResponseSuccess("success", respOrder))
}

// GetAllAdmin implements OrderHandlerInterface.
func (o *orderHandler) GetAllAdmin(c echo.Context) error {
	var (
		ctx        = c.Request().Context()
		respOrders = []response.OrderAdminList{}
	)

	user := c.Get("user").(string)
	if user == "" {
		log.Errorf("[OrderHandler-1] GetAllAdmin: %s", "data token not found")
		return c.JSON(http.StatusNotFound, response.ResponseError("data token not found"))
	}

	search := c.QueryParam("search")
	var page int64 = 1
	if pageStr := c.QueryParam("page"); pageStr != "" {
		page, _ = conv.StringToInt64(pageStr)
		if page <= 0 {
			page = 1
		}
	}

	var perPage int64 = 10
	perPageStr := c.QueryParam("perPage")
	if perPageStr == "" {
		perPageStr = c.QueryParam("limit")
	}
	if perPageStr != "" {
		perPage, _ = conv.StringToInt64(perPageStr)
		if perPage <= 0 {
			perPage = 10
		}
	}

	status := ""
	if statusStr := c.QueryParam("status"); statusStr != "" {
		status = statusStr
	}

	reqEntity := entity.QueryStringEntity{
		Search: search,
		Status: status,
		Page:   page,
		Limit:  perPage,
	}

	results, totalData, totalPage, err := o.orderService.GetAll(ctx, reqEntity, user)
	if err != nil {
		log.Errorf("[OrderHandler-1] GetAllAdmin: %v", err)
		if err.Error() == "404" {
			return c.JSON(http.StatusNotFound, response.ResponseError("data not found"))
		}
		return c.JSON(http.StatusInternalServerError, response.ResponseError(err.Error()))
	}

	for _, result := range results {
		for _, item := range result.OrderItems {
			respOrders = append(respOrders, response.OrderAdminList{
				ID:            result.ID,
				OrderCode:     result.OrderCode,
				ProductName:   item.ProductName,
				Status:        result.Status,
				PaymentMethod: result.PaymentMethod,
				ProductImage:  item.ProductImage,
				CustomerName:  result.BuyerName,
				ProductPrice:  item.Price,
				Quantity:      item.Quantity,
				OrderDatetime: result.OrderDate,
			})
		}
	}

	return c.JSON(http.StatusOK, response.ResponseSuccessWithPagination("success", respOrders, page, totalData, totalPage, perPage))
}

func NewOrderHandler(orderService service.OrderServiceInterface, e *echo.Echo, cfg *config.Config) OrderHandlerInterface {
	ordHandler := &orderHandler{orderService: orderService, cfg: cfg}

	e.Use(middleware.Recover())
	mid := adapter.NewMiddlewareAdapter(cfg)
	e.GET("public/orders/:orderCode/code", ordHandler.GetPublicOrderByOrderCode)
	e.POST("internal/orders/payment-success", ordHandler.PaymentSucceeded)
	e.POST("internal/orders/payment-expired", ordHandler.PaymentExpired)
	authGroup := e.Group("auth", mid.CheckToken())
	authGroup.POST("/orders", ordHandler.CreateOrder, mid.DistanceCheck())
	authGroup.GET("/orders", ordHandler.GetAllCustomer)
	authGroup.GET("/orders/:orderID", ordHandler.GetDetailCustomer)
	authGroup.GET("/orders/:orderCode/code", ordHandler.GetOrderByOrderCode)

	adminGroup := e.Group("/admin", mid.CheckToken(), mid.RequireAdminRole())
	adminGroup.GET("/orders", ordHandler.GetAllAdmin)
	adminGroup.GET("/orders/:orderID", ordHandler.GetByIDAdmin)
	adminGroup.PUT("/orders/:orderID/status", ordHandler.UpdateStatus)
	adminGroup.DELETE("/orders/:orderID", ordHandler.DeleteByID)

	return ordHandler
}
