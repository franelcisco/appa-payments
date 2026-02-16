package shopify

import (
	"encoding/json"
)

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// enum shopify kind
const (
	OrderKind    = "Order"
	CustomerKind = "Customer"
)

// GetOrderByIDResponse constructs a global ID for Shopify entities
type GetOrderByIDResponse struct {
	Order *Order `json:"order"`
}

// GetOrderByQueryResponse represents the response for querying multiple orders
type GetOrderByQueryResponse struct {
	Orders OrdersNodes `json:"orders"`
}

// OrdersNodes represents a list of order nodes
type OrdersNodes struct {
	Nodes []Order `json:"nodes"`
}

// OrdersByPage represents a paginated list of orders
type OrdersByPage struct {
	Order
	PageInfo PageInfo `json:"pageInfo"`
}

// PageInfo represents pagination information for a list of orders
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	StartCursor     string `json:"startCursor"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	EndCursor       string `json:"endCursor"`
}

// Order represents a Shopify order
type Order struct {
	ID                       string        `json:"id"`
	Name                     string        `json:"name"`
	StatusPageUrl            string        `json:"statusPageUrl"`
	CreatedAt                string        `json:"createdAt"`
	DisplayFinancialStatus   string        `json:"displayFinancialStatus"`
	DisplayFulfillmentStatus string        `json:"displayFulfillmentStatus"`
	CurrentTotalPriceSet     ShopMoney     `json:"currentTotalPriceSet"`
	LineItems                LineItemsEdge `json:"lineItems"`
	Customer                 Customer      `json:"customer"`
	Transactions             []Transaction `json:"transactions"`
}

// Transaction represents a transaction in an order
type Transaction struct {
	AmountSet ShopMoney `json:"amountSet"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
}

// LineItemsEdge represents the edge of line items in an order
type LineItemsEdge struct {
	Edges []LineItemsNode `json:"edges"`
}

// LineItemsNode represents a node in the line items edge
type LineItemsNode struct {
	Node LineItem `json:"node"`
}

// LineItem represents a line item in an order
type LineItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	SKU      string `json:"sku"`
}

// Variant represents a product variant
type Variant struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ShopMoney represents the total or subtotal price set of an order
type ShopMoney struct {
	ShopMoney ShopMoneyProps `json:"shopMoney"`
}

// ShopMoneyProps represents an amount of money in a specific currency
type ShopMoneyProps struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

// Customer represents a Shopify customer
type Customer struct {
	ID                 string                      `json:"id"`
	DisplayName        string                      `json:"displayName"`
	DefaultPhoneNumber *CustomerDefaultPhoneNumber `json:"defaultPhoneNumber"`
	ParentID           *Metafield                  `json:"parentId"`
	DirectDebit        *Metafield                  `json:"directDebit"`
}

type CustomerDefaultPhoneNumber struct {
	PhoneNumber string `json:"phoneNumber"`
}

// Metafield represents a Shopify metafield
type Metafield struct {
	Key       string          `json:"key"`
	Value     string          `json:"value"`
	JsonValue json.RawMessage `json:"jsonValue,omitempty"`
}

// QueryOrderFilter represents the filters for querying orders
type QueryOrderFilter struct {
	Name string
}

// SetCustomerMetafieldResponse
type SetCustomerMetafieldResponse struct {
	CustomerUpdate
}

type CustomerUpdate struct {
	Customer   Customer     `json:"customer"`
	UserErrors []UserErrors `json:"userErrors"`
}

type UserErrors struct {
	Message string `json:"message"`
}

// GetCustomerMetafieldResponse
type GetCustomerMetafieldResponse struct {
	Customer struct {
		Metafield *Metafield `json:"metafield"`
	} `json:"customer"`
}

type DebitDirectJson struct {
	Bank    string `json:"bank"`
	Phone   string `json:"phone"`
	DNI     string `json:"dni"`
	DNIType string `json:"dni_type"`
}

type MarkOrderAsPaidResponse struct {
	Order      Order        `json:"order"`
	UserErrors []UserErrors `json:"userErrors"`
}
