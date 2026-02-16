package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	"appa_payments/pkg/bcv"
	dbModels "appa_payments/pkg/db/models"
	"appa_payments/pkg/r4bank"
	"appa_payments/pkg/shopify"
)

type storeService struct {
	ShopifyRepository shopify.Repository
	Logger            *zap.Logger
	R4Repository      r4bank.R4Repository
	DB                *gorm.DB
	bcvClient         bcv.Client
}

// NewStoreService creates a new StoreService
func NewStoreService(
	shopifyRepo shopify.Repository,
	R4Repository r4bank.R4Repository,
	DB *gorm.DB,
	bcvClient bcv.Client,
	logger *zap.Logger,
) domains.StoreService {
	return &storeService{
		ShopifyRepository: shopifyRepo,
		R4Repository:      R4Repository,
		DB:                DB,
		bcvClient:         bcvClient,
		Logger:            logger,
	}
}

// getLineItems converts Shopify line items to models.LineItem
func (s *storeService) getLineItems(
	lineItems []shopify.LineItemsNode,
) []models.LineItem {
	items := make([]models.LineItem, 0, len(lineItems))
	for _, edge := range lineItems {
		item := edge.Node
		items = append(items, models.LineItem{
			Name:     item.Name,
			Quantity: item.Quantity,
			SKU:      item.SKU,
		})
	}
	return items
}

// getOrderResponse converts a Shopify order to models.OrderResponse
func (s *storeService) getOrderResponse(
	ctx context.Context,
	order shopify.Order,
) (*models.OrderResponse, error) {

	// Get BCV Tasa
	tasaBCV, err := s.bcvClient.Get(ctx)
	if err != nil {
		s.Logger.Error("Failed to get BCV tasa", zap.Error(err))
		return nil, err
	}

	lineItems := s.getLineItems(order.LineItems.Edges)

	var (
		totalAmount float64
		phone,
		dni,
		dniType string
	)

	if value, err := strconv.ParseFloat(order.CurrentTotalPriceSet.ShopMoney.Amount, 64); err == nil {
		totalAmount = value
	}
	// Preferentially use Venezuelan phone numbers
	if order.Customer.DefaultPhoneNumber != nil && strings.Contains(order.Customer.DefaultPhoneNumber.PhoneNumber, "+58") {
		phone = order.Customer.DefaultPhoneNumber.PhoneNumber
	}
	if order.Customer.ParentID != nil {
		parentID := strings.Split(order.Customer.ParentID.Value, "-")
		s.Logger.Info("ParentID", zap.Strings("parentID", parentID))
		if len(parentID) == 2 {
			dni = parentID[1]
			dniType = parentID[0]
		}
	}

	var directDebit models.DebitDirect
	if order.Customer.DirectDebit != nil && order.Customer.DirectDebit.JsonValue != nil {
		err := json.Unmarshal([]byte(order.Customer.DirectDebit.JsonValue), &directDebit)
		if err != nil {
			s.Logger.Error(err.Error(), zap.Any("json", order.Customer.DirectDebit.JsonValue))
		}
	}

	response := &models.OrderResponse{
		ID:                       strings.TrimPrefix(order.ID, "gid://shopify/Order/"),
		Name:                     order.Name,
		StatusPageUrl:            order.StatusPageUrl,
		CreatedAt:                order.CreatedAt,
		DisplayFinancialStatus:   order.DisplayFinancialStatus,
		DisplayFulfillmentStatus: order.DisplayFulfillmentStatus,
		TotalPriceSetUSD: models.OrderPrice{
			Amount:       order.CurrentTotalPriceSet.ShopMoney.Amount,
			CurrencyCode: order.CurrentTotalPriceSet.ShopMoney.CurrencyCode,
		},
		TotalPriceSetVES: models.OrderPrice{
			Amount:       fmt.Sprintf("%.2f", totalAmount*tasaBCV),
			CurrencyCode: "VES",
		},
		LineItems: lineItems,
		Customer: models.Customer{
			ID:          strings.TrimPrefix(order.Customer.ID, "gid://shopify/Customer/"),
			DisplayName: order.Customer.DisplayName,
			Phone:       phone,
			DNI:         dni,
			DNIType:     dniType,
		},
		DebitDirect: &directDebit,
	}

	return response, nil
}

// getManualOrderByFilter retrieves manual orders based on the provided filter
func (s *storeService) getManualOrderByFilter(
	ctx context.Context,
	filter dbModels.ManualOrder,
) (*models.OrderResponse, error) {
	var item dbModels.ManualOrder
	err := s.DB.Model(&dbModels.ManualOrder{}).WithContext(ctx).
		Where(filter).Where("validate_status <> ?", "CANCELED").
		First(&item).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.Logger.Error("failed to get manual order", zap.Error(err))
		return nil, err
	}

	if item.ID != 0 {
		return &models.OrderResponse{
			ID:   fmt.Sprintf("%d", item.OrderID),
			Name: item.OrderName,
		}, nil
	}

	return nil, nil
}

func (s *storeService) GetOrderByID(
	ctx context.Context,
	id string,
) (*models.OrderResponse, error) {
	orderID, err := strconv.Atoi(id)
	if err != nil {
		s.Logger.Error("invalid order ID", zap.String("id", id), zap.Error(err))
		return nil, fmt.Errorf("invalid order ID")
	}

	store, err := s.ShopifyRepository.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err // or custom error
	}

	manualOrder, err := s.getManualOrderByFilter(ctx, dbModels.ManualOrder{
		OrderID: orderID,
	})
	if err != nil {
		s.Logger.Error("failed to get manual order", zap.Error(err))
		return nil, err
	}

	if manualOrder != nil {
		manualOrder.DisplayFinancialStatus = store.Order.DisplayFinancialStatus
		manualOrder.DisplayFulfillmentStatus = "MANUAL"
		return manualOrder, nil
	}

	orderResponse, err := s.getOrderResponse(ctx, *store.Order)
	if err != nil {
		return nil, err
	}

	return orderResponse, nil
}

// GetOrderByName
func (s *storeService) GetOrderByName(
	ctx context.Context,
	name string,
) (*models.OrderResponse, error) {

	filters := shopify.QueryOrderFilter{
		Name: name,
	}
	store, err := s.ShopifyRepository.GetOrderByQuery(ctx, filters, 1)
	if err != nil {
		return nil, err
	}

	if len(store.Orders.Nodes) == 0 {
		s.Logger.Error("order not found", zap.String("name", name))
		return nil, fmt.Errorf("order not found")
	}

	order := store.Orders.Nodes[0]

	manualOrder, err := s.getManualOrderByFilter(ctx, dbModels.ManualOrder{
		OrderName: fmt.Sprintf("#%s", name),
	})
	if err != nil {
		s.Logger.Error("failed to get manual order", zap.Error(err))
		return nil, err
	}

	if manualOrder != nil {
		manualOrder.DisplayFinancialStatus = order.DisplayFinancialStatus
		manualOrder.DisplayFulfillmentStatus = "MANUAL"
		return manualOrder, nil
	}

	orderResponse, err := s.getOrderResponse(ctx, order)
	if err != nil {
		return nil, err
	}

	return orderResponse, nil
}

// UpdateCustomerParentID updates the parent customer ID for a given customer
func (s *storeService) UpdateCustomerParentID(
	ctx context.Context,
	req models.UpdateCustomerParentIDRequest,
) error {
	err := s.ShopifyRepository.SetCustomerParentID(ctx, req.CustomerID, fmt.Sprintf("%s-%s", req.DNIType, req.DNI))
	if err != nil {
		s.Logger.Error("failed to update customer parent ID", zap.Error(err))
		return err
	}

	return nil
}
