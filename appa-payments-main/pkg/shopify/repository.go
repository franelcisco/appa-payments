package shopify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

const (
	customerNamespace      = "customer_fields"
	customNamespace        = "custom"
	customerParentKey      = "parent_id"
	customerDirectDebitKey = "direct_debit"
)

// Repository defines methods to interact with Shopify API
type Repository interface {
	GetOrderByID(ctx context.Context, id string) (*GetOrderByIDResponse, error)
	GetOrderByQuery(ctx context.Context, filters QueryOrderFilter, first int) (*GetOrderByQueryResponse, error)
	SetCustomerParentID(ctx context.Context, customerID, parentID string) error
	GetCustomerParentID(ctx context.Context, customerID string) (*Metafield, error)
	GetCustomerDebitDirect(ctx context.Context, customerID string) (*Metafield, error)
	SetDebitDirect(ctx context.Context, customerID string, jsonValue DebitDirectJson) error
	MarkOrderAsPaid(ctx context.Context, orderID string) error
}

// Repository is a Shopify API repository
type repository struct {
	gql    *GraphQLClient
	Logger *zap.Logger
}

// NewRepository creates a new Shopify API repository
func NewRepository(
	shopDomain, apiVersion, adminToken string, logger *zap.Logger,
) Repository {
	return &repository{
		gql:    NewGraphQLClient(shopDomain, apiVersion, adminToken, logger),
		Logger: logger,
	}
}

func getQueryOrderByFilters(filters QueryOrderFilter) string {
	var query string

	if filters.Name != "" {
		query += fmt.Sprintf("name:%s ", filters.Name)
	}

	return query
}

// GetOrderByID retrieves an order by its ID
func (r *repository) GetOrderByID(
	ctx context.Context, id string,
) (*GetOrderByIDResponse, error) {
	gid := GID(OrderKind, id)
	var resp GetOrderByIDResponse
	if err := r.gql.Do(ctx, getOrderByIDQuery, map[string]any{"id": gid}, &resp); err != nil {
		return nil, err
	}

	if resp.Order == nil {
		r.Logger.Error("order not found", zap.String("id", id))
		return nil, fmt.Errorf("order %s not found", id)
	}

	finalPrice, err := getOrderFinalPrice(
		resp.Order.CurrentTotalPriceSet.ShopMoney.Amount,
		resp.Order.Transactions,
	)
	if err != nil {
		r.Logger.Error(err.Error(), zap.Any("currentPrice", resp.Order.CurrentTotalPriceSet.ShopMoney.Amount), zap.Any("tx", resp.Order.Transactions))
		return nil, err
	}

	resp.Order.CurrentTotalPriceSet.ShopMoney.Amount = finalPrice

	return &resp, nil
}

// GetOrderByQuery retrieves orders based on the provided filters
func (r *repository) GetOrderByQuery(
	ctx context.Context, filters QueryOrderFilter, first int,
) (*GetOrderByQueryResponse, error) {
	query := getQueryOrderByFilters(filters)
	var resp GetOrderByQueryResponse
	if err := r.gql.Do(ctx, getOrderByName, map[string]any{"query": query, "first": first}, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.Any("filters", filters))
		return nil, err
	}

	if len(resp.Orders.Nodes) == 0 {
		r.Logger.Info("no orders found", zap.Any("filters", filters))
		return nil, nil
	}

	respOrder := &resp.Orders.Nodes[0]
	finalPrice, err := getOrderFinalPrice(
		respOrder.CurrentTotalPriceSet.ShopMoney.Amount,
		respOrder.Transactions,
	)
	if err != nil {
		r.Logger.Error(err.Error(), zap.Any("currentPrice", respOrder.CurrentTotalPriceSet.ShopMoney.Amount), zap.Any("tx", respOrder.Transactions))
		return nil, err
	}

	respOrder.CurrentTotalPriceSet.ShopMoney.Amount = finalPrice
	resp.Orders.Nodes[0] = *respOrder
	return &resp, nil
}

func (r *repository) SetCustomerParentID(
	ctx context.Context, customerID, parentID string,
) error {
	gid := GID(CustomerKind, customerID)
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerParentKey,
		"value":     parentID,
	}
	var resp SetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, setCustomerMetafield, vars, &resp); err != nil {
		return err
	}

	if len(resp.UserErrors) > 0 {
		r.Logger.Error("failed to set customer parent ID", zap.Any("errors", resp.UserErrors))
		return errors.New("failed to set customer parent ID")
	}

	return nil
}

func (r *repository) GetCustomerParentID(
	ctx context.Context, customerID string,
) (*Metafield, error) {
	gid := GID(CustomerKind, customerID)
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerParentKey,
	}

	var resp GetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, getCustomerMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", customerID))
		return nil, err
	}

	if resp.Customer.Metafield == nil {
		r.Logger.Error("customer parent ID metafield not found", zap.String("customerID", customerID))
		return nil, errors.New("customer parent ID metafield not found")
	}

	return resp.Customer.Metafield, nil
}

func (r *repository) GetCustomerDebitDirect(
	ctx context.Context, gid string,
) (*Metafield, error) {
	if !strings.Contains(gid, CustomerKind) {
		gid = GID(CustomerKind, gid)
	}
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerDirectDebitKey,
	}

	var resp GetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, getCustomerMetafield, vars, &resp); err != nil {
		return nil, err
	}

	if resp.Customer.Metafield == nil {
		return nil, errors.New("customer direct debit metafield not found")
	}

	return resp.Customer.Metafield, nil
}

func (r *repository) SetDebitDirect(
	ctx context.Context, gid string, jsonData DebitDirectJson,
) error {
	jsonValue, err := json.Marshal(jsonData)
	if err != nil {
		return err
	}

	// jsonValue := fmt.Sprintf(`"{\"bank\": \"%s\", \"phone\": \"%s\", \"dni\": \"%s\", \"dni_type\": \"%s\"}"`,
	// 	jsonData.Bank, jsonData.Phone, jsonData.DNI, jsonData.DNIType)

	if !strings.Contains(gid, CustomerKind) {
		gid = GID(CustomerKind, gid)
	}
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerDirectDebitKey,
		"type":      "json",
		"value":     string(jsonValue), // Deprecated but required by Shopify API
	}
	var resp SetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, setCustomerMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", gid), zap.Any("jsonValue", string(jsonValue)))
		return err
	}

	if len(resp.UserErrors) > 0 {
		r.Logger.Error("failed to set customer parent ID", zap.Any("errors", resp.UserErrors))
		return errors.New("failed to set customer parent ID")
	}

	return nil
}

// MarkOrderAsPaid marks an order as paid
func (r *repository) MarkOrderAsPaid(ctx context.Context, gid string) error {
	if !strings.Contains(gid, OrderKind) {
		gid = GID(OrderKind, gid)
	}

	vars := map[string]any{
		"id": gid,
	}
	var resp MarkOrderAsPaidResponse
	if err := r.gql.Do(ctx, markOrderAsPaid, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.Any("vars", vars))
		return err
	}

	if resp.UserErrors != nil {
		r.Logger.Error("failed to mark order as paid", zap.Any("errors", resp.UserErrors))
		return errors.New("failed to mark order as paid")
	}

	return nil
}

// GetOrderFinalPrice calculates the final price of an order after successful transactions
func getOrderFinalPrice(currentPriceStr string, transactions []Transaction) (string, error) {
	currentPrice, err := strconv.ParseFloat(currentPriceStr, 64)
	if err != nil {
		return "", err
	}

	for _, tx := range transactions {
		if strings.ToLower(tx.Status) == "success" && (strings.ToLower(tx.Kind) == "sale" || strings.ToLower(tx.Kind) == "capture") {
			txPrice, err := strconv.ParseFloat(tx.AmountSet.ShopMoney.Amount, 64)
			if err != nil {
				return "", err
			}
			currentPrice = currentPrice - txPrice
		}
	}

	return fmt.Sprintf("%.2f", currentPrice), nil
}
