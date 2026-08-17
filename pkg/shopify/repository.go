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
	customerNamespace             = "customer_fields"
	customNamespace               = "custom"
	customerParentKey             = "parent_id"
	customerDirectDebitKey        = "direct_debit"
	customerDirectDebitAccountKey = "direct_debit_account"
)

// Repository defines methods to interact with Shopify API
type Repository interface {
	GetOrderByID(ctx context.Context, id string) (*GetOrderByIDResponse, error)
	GetOrderByQuery(ctx context.Context, filters QueryOrderFilter, first int) (*GetOrderByQueryResponse, error)
	GetCustomerByID(ctx context.Context, customerID string) (*Customer, error)
	SetCustomerParentID(ctx context.Context, customerID, parentID string) error
	GetCustomerParentID(ctx context.Context, customerID string) (*Metafield, error)
	GetCustomerDebitDirect(ctx context.Context, customerID string) (*Metafield, error)
	SetDebitDirect(ctx context.Context, customerID string, jsonValue DebitDirectJson) error
	SetCustomerDebitDirectAccount(ctx context.Context, customerID string, jsonValue DebitDirectAccountJson) error
	DeleteCustomerDebitDirectAccount(ctx context.Context, customerID string) error
	AddOrderTags(ctx context.Context, orderID string, tags []string) error
	AddThirtyPercentDiscountToOrder(ctx context.Context, orderID string, porcentValue float64, description string) error
	MarkOrderAsPaid(ctx context.Context, orderID string) error
	GetDraftOrderByID(ctx context.Context, id string) (*GetDraftOrderByIDResponse, error)
	AddDraftOrderTags(ctx context.Context, gid string, tags []string) error
	CompleteDraftOrder(ctx context.Context, draftGID string, paymentPending bool) (*CompletedOrder, error)
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

// GetCustomerByID retrieves a customer by its ID
func (r *repository) GetCustomerByID(ctx context.Context, id string) (*Customer, error) {
	gid := EnsureGID(CustomerKind, id)

	var resp GetCustomerByIDResponse
	if err := r.gql.Do(ctx, getCustomerByID, map[string]any{"id": gid}, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", gid))
		return nil, err
	}

	return resp.Customer, nil
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
	gid := EnsureGID(CustomerKind, customerID)
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerParentKey,
		"value":     parentID,
	}
	var resp SetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, setCustomerMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", customerID), zap.String("gid", gid))
		return err
	}

	if len(resp.UserErrors) > 0 {
		r.Logger.Error("failed to set customer parent ID", zap.Any("errors", resp.UserErrors), zap.String("customerID", customerID), zap.String("gid", gid))
		return errors.New("failed to set customer parent ID")
	}

	return nil
}

func (r *repository) GetCustomerParentID(
	ctx context.Context, customerID string,
) (*Metafield, error) {
	gid := EnsureGID(CustomerKind, customerID)
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerParentKey,
	}

	var resp GetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, getCustomerMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", customerID), zap.String("gid", gid))
		return nil, err
	}

	if resp.Customer.Metafield == nil {
		r.Logger.Error("customer parent ID metafield not found", zap.String("customerID", customerID), zap.String("gid", gid))
		return nil, errors.New("customer parent ID metafield not found")
	}

	return resp.Customer.Metafield, nil
}

func (r *repository) GetCustomerDebitDirect(
	ctx context.Context, gid string,
) (*Metafield, error) {
	gid = EnsureGID(CustomerKind, gid)
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

	gid = EnsureGID(CustomerKind, gid)
	vars := map[string]any{
		"id":        gid,
		"namespace": customerNamespace,
		"key":       customerDirectDebitKey,
		"type":      "json",
		"value":     string(jsonValue), // Deprecated but required by Shopify API
	}
	var resp SetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, setCustomerMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", gid))
		return err
	}

	if len(resp.UserErrors) > 0 {
		r.Logger.Error("failed to set customer direct debit", zap.Any("errors", resp.UserErrors), zap.String("customerID", gid))
		return errors.New("failed to set customer direct debit")
	}

	r.Logger.Info("successfully set customer direct debit", zap.String("customerID", gid), zap.Any("data", jsonData))

	return nil
}

// SetCustomerDebitDirectAccount sets the customer's direct debit account number in a metafield
func (r *repository) SetCustomerDebitDirectAccount(
	ctx context.Context, gid string, jsonValues DebitDirectAccountJson,
) error {
	jsonValue, err := json.Marshal(jsonValues)
	if err != nil {
		return err
	}

	gid = EnsureGID(CustomerKind, gid)
	vars := map[string]any{
		"id":        gid,
		"namespace": customNamespace,
		"key":       customerDirectDebitAccountKey,
		"type":      "json",
		"value":     string(jsonValue), // Deprecated but required by Shopify API
	}
	var resp SetCustomerMetafieldResponse
	if err := r.gql.Do(ctx, setCustomerMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", gid), zap.String("account", string(jsonValue)))
		return err
	}

	if len(resp.UserErrors) > 0 {
		r.Logger.Error("failed to set customer direct debit account", zap.Any("errors", resp.UserErrors))
		return errors.New("failed to set customer direct debit account")
	}

	return nil
}

// DeleteCustomerDebitDirectAccount deletes the customer's direct debit account metafield
func (r *repository) DeleteCustomerDebitDirectAccount(ctx context.Context, gid string) error {
	gid = EnsureGID(CustomerKind, gid)
	vars := map[string]any{
		"id":        gid,
		"namespace": customNamespace,
		"key":       customerDirectDebitAccountKey,
	}

	var resp DeleteCustomerMetafieldResponse
	if err := r.gql.Do(ctx, deleteMetafield, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("customerID", gid))
		return err
	}

	if len(resp.MetafieldsDelete.UserErrors) > 0 {
		r.Logger.Error("failed to delete customer direct debit account", zap.Any("errors", resp.MetafieldsDelete.UserErrors))
		return errors.New("failed to delete customer direct debit account")
	}

	return nil
}

// AddOrderTags adds tags to an order
func (r *repository) AddOrderTags(ctx context.Context, gid string, tags []string) error {
	gid = EnsureGID(OrderKind, gid)
	vars := map[string]any{
		"id":   gid,
		"tags": tags,
	}

	var resp AddOrderTagsResponse
	if err := r.gql.Do(ctx, addOrderTags, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("orderID", gid), zap.Any("tags", tags))
		return err
	}

	if len(resp.TagsAdd.UserErrors) > 0 {
		r.Logger.Error("failed to add order tags", zap.Any("errors", resp.TagsAdd.UserErrors))
		return errors.New("failed to add order tags")
	}

	return nil
}

// BeginOrderEdit begins an order edit and returns the calculated order
func (r *repository) BeginOrderEdit(ctx context.Context, gid string) (*CalculatedOrder, error) {
	gid = EnsureGID(OrderKind, gid)
	vars := map[string]any{
		"id": gid,
	}

	var resp BeginOrderEditResponse
	if err := r.gql.Do(ctx, beginOrderEdit, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("orderID", gid))
		return nil, err
	}

	if len(resp.OrderEditBegin.UserErrors) > 0 {
		r.Logger.Error("failed to begin order edit", zap.Any("errors", resp.OrderEditBegin.UserErrors))
		return nil, errors.New("failed to begin order edit")
	}

	r.Logger.Info("successfully began order edit", zap.Any("response", resp))

	return &resp.OrderEditBegin.CalculatedOrder, nil
}

// addThirtyPercentDiscountToLineItem adds a discount to a line item in an order edit
func (r *repository) addThirtyPercentDiscountToLineItem(
	ctx context.Context, calculatedOrderID, calculatedLineItemID, description string, percentValue float64,
) error {
	vars := map[string]any{
		"calculatedOrderId":    calculatedOrderID,
		"calculatedLineItemId": calculatedLineItemID,
		"description":          description,
		"percentValue":         percentValue,
	}

	var resp AddThirtyPercentDiscountToLineItemResponse
	if err := r.gql.Do(ctx, addThirtyPercentDiscountToLineItem, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("calculatedOrderID", calculatedOrderID), zap.String("calculatedLineItemID", calculatedLineItemID))
		return err
	}

	if len(resp.OrderEditAddLineItemDiscount.UserErrors) > 0 {
		r.Logger.Error("failed to add discount to line item", zap.Any("errors", resp.OrderEditAddLineItemDiscount.UserErrors))
		return errors.New("failed to add discount to line item")
	}

	return nil
}

// commitOrderEdit commits an order edit to apply discounts
func (r *repository) commitOrderEdit(ctx context.Context, calculatedOrderID string) error {
	vars := map[string]any{
		"calculatedOrderId": calculatedOrderID,
	}

	var resp CommitOrderEditResponse
	if err := r.gql.Do(ctx, commitOrderEdit, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("calculatedOrderID", calculatedOrderID))
		return err
	}

	if len(resp.OrderEditCommit.UserErrors) > 0 {
		r.Logger.Error("failed to commit order edit", zap.Any("errors", resp.OrderEditCommit.UserErrors))
		return errors.New("failed to commit order edit")
	}

	return nil
}

// AddThirtyPercentDiscountToOrder adds a percent discount to all line items in an order by performing an order edit
func (r *repository) AddThirtyPercentDiscountToOrder(
	ctx context.Context, gid string, porcentValue float64, description string,
) error {
	gid = EnsureGID(OrderKind, gid)
	// 1. Begin order edit to get the calculated order and line item IDs
	calculatedOrder, err := r.BeginOrderEdit(ctx, gid)
	if err != nil {
		return err
	}

	if calculatedOrder.ID == "" {
		r.Logger.Error("calculated order ID is empty", zap.String("orderID", gid), zap.Any("c", calculatedOrder))
		return errors.New("calculated order ID is empty")
	}

	// 2. Loop through line items and add discount to each
	for _, lineItem := range calculatedOrder.LineItems.Nodes {
		if err := r.addThirtyPercentDiscountToLineItem(
			ctx, calculatedOrder.ID, lineItem.ID, description, porcentValue,
		); err != nil {
			return err
		}
	}

	// 3. Commit the order edit to apply the discounts
	if err := r.commitOrderEdit(ctx, calculatedOrder.ID); err != nil {
		return err
	}

	return nil
}

// MarkOrderAsPaid marks an order as paid
func (r *repository) MarkOrderAsPaid(ctx context.Context, gid string) error {
	gid = EnsureGID(OrderKind, gid)

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

// GetDraftOrderByID retrieves a draft order by its ID. Accepts either a
// numeric ID or a full Shopify GID.
func (r *repository) GetDraftOrderByID(
	ctx context.Context, id string,
) (*GetDraftOrderByIDResponse, error) {
	gid := id
	if !strings.HasPrefix(gid, DraftOrderKindID) {
		gid = GID(draftOrderKind, id)
	}

	var resp GetDraftOrderByIDResponse
	if err := r.gql.Do(ctx, getDraftOrderByID, map[string]any{"id": gid}, &resp); err != nil {
		return nil, err
	}

	if resp.DraftOrder == nil {
		r.Logger.Error("draft order not found", zap.String("id", id))
		return nil, fmt.Errorf("draft order %s not found", id)
	}

	return &resp, nil
}

// AddDraftOrderTags adds tags to a draft order. Accepts either a numeric ID
// or a full Shopify GID.
func (r *repository) AddDraftOrderTags(ctx context.Context, gid string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	if !strings.HasPrefix(gid, DraftOrderKindID) {
		gid = GID(draftOrderKind, gid)
	}
	vars := map[string]any{
		"id":   gid,
		"tags": tags,
	}

	var resp AddOrderTagsResponse
	if err := r.gql.Do(ctx, addOrderTags, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("draftOrderID", gid), zap.Any("tags", tags))
		return err
	}

	if len(resp.TagsAdd.UserErrors) > 0 {
		r.Logger.Error("failed to add draft order tags", zap.Any("errors", resp.TagsAdd.UserErrors))
		return errors.New("failed to add draft order tags")
	}

	return nil
}

// CompletedOrder is what CompleteDraftOrder hands back once a draft becomes a
// real order.
type CompletedOrder struct {
	OrderGID               string
	LegacyOrderID          string
	Name                   string
	StatusPageURL          string
	DisplayFinancialStatus string
}

// CompleteDraftOrder turns a draft into a real order via draftOrderComplete.
// paymentPending: false means "born paid".
func (r *repository) CompleteDraftOrder(
	ctx context.Context, draftGID string, paymentPending bool,
) (*CompletedOrder, error) {
	if !strings.HasPrefix(draftGID, DraftOrderKindID) {
		draftGID = GID(draftOrderKind, draftGID)
	}
	vars := map[string]any{"id": draftGID, "paymentPending": paymentPending}

	var resp CompleteDraftOrderResponse
	if err := r.gql.Do(ctx, draftOrderComplete, vars, &resp); err != nil {
		r.Logger.Error(err.Error(), zap.String("draftOrderID", draftGID))
		return nil, err
	}
	if len(resp.DraftOrderComplete.UserErrors) > 0 {
		r.Logger.Error("failed to complete draft order", zap.Any("errors", resp.DraftOrderComplete.UserErrors))
		return nil, errors.New("failed to complete draft order")
	}
	draftOrder := resp.DraftOrderComplete.DraftOrder
	if draftOrder == nil || draftOrder.Order == nil {
		return nil, errors.New("draft order completed without an order")
	}
	order := draftOrder.Order
	return &CompletedOrder{
		OrderGID:               order.ID,
		LegacyOrderID:          order.LegacyResourceID,
		Name:                   order.Name,
		StatusPageURL:          order.StatusPageUrl,
		DisplayFinancialStatus: order.DisplayFinancialStatus,
	}, nil
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
