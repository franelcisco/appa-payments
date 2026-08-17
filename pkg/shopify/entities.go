package shopify

import (
	"encoding/json"
	"strings"
)

// HasDirectDebitAccount checks if a customer has a direct debit account
func (c Customer) HasDirectDebitAccount() bool {
	if c.DirectDebitAccount == nil || c.DirectDebitAccount.JsonValue == nil {
		return false
	}
	return true
}

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
	OrderKind        = "Order"
	CustomerKind     = "Customer"
	draftOrderKind   = "DraftOrder"
	CustomerKindID   = "gid://shopify/Customer/"
	OrderKindID      = "gid://shopify/Order/"
	DraftOrderKindID = "gid://shopify/DraftOrder/"
)

// GetOrderByIDResponse constructs a global ID for Shopify entities
type GetOrderByIDResponse struct {
	Order *Order `json:"order"`
}

// GetCustomerByIDResponse constructs a global ID for Shopify entities
type GetCustomerByIDResponse struct {
	Customer *Customer `json:"customer"`
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
	Tags                     []string      `json:"tags"`
	App                      *App          `json:"app,omitempty"`
	StatusPageUrl            string        `json:"statusPageUrl"`
	CreatedAt                string        `json:"createdAt"`
	DisplayFinancialStatus   string        `json:"displayFinancialStatus"`
	DisplayFulfillmentStatus string        `json:"displayFulfillmentStatus"`
	CurrentTotalPriceSet     ShopMoney     `json:"currentTotalPriceSet"`
	LineItems                LineItemsEdge `json:"lineItems"`
	Customer                 Customer      `json:"customer"`
	Transactions             []Transaction `json:"transactions"`
}

// App represents the Shopify App that created an order
type App struct {
	ID string `json:"id"`
}

const appGIDPrefix = "gid://shopify/App/"

// IsID reports whether the receiver matches the given app id. It accepts the
// input either as a numeric id (e.g. "293844877313") or as a full Shopify GID
// ("gid://shopify/App/293844877313"); the prefix is stripped from both sides
// before comparison so callers don't have to normalise.
func (a App) IsID(id string) bool {
	if id == "" {
		return false
	}
	return strings.TrimPrefix(a.ID, appGIDPrefix) == strings.TrimPrefix(id, appGIDPrefix)
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
	Name             string    `json:"name"`
	Quantity         int       `json:"quantity"`
	SKU              string    `json:"sku"`
	TotalDiscountSet ShopMoney `json:"totalDiscountSet"`
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
	Email              string                      `json:"email"`
	DefaultPhoneNumber *CustomerDefaultPhoneNumber `json:"defaultPhoneNumber"`
	ParentID           *Metafield                  `json:"parentId"`
	DirectDebit        *Metafield                  `json:"directDebit"`
	DirectDebitAccount *Metafield                  `json:"directDebitAccount"`
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

type DebitDirectAccountJson struct {
	Account string `json:"account"`
	DNI     string `json:"dni"`
}

type AddOrderTagsResponse struct {
	TagsAdd struct {
		UserErrors []UserErrors `json:"userErrors"`
	} `json:"tagsAdd"`
}

type CalculatedLineItem struct {
	ID string `json:"id"`
}

type CalculatedOrder struct {
	ID        string `json:"id"`
	LineItems struct {
		Nodes []CalculatedLineItem `json:"nodes"`
	} `json:"lineItems"`
}

type BeginOrderEditResponse struct {
	OrderEditBegin struct {
		CalculatedOrder CalculatedOrder `json:"calculatedOrder"`
		UserErrors      []UserErrors    `json:"userErrors"`
	} `json:"orderEditBegin"`
}

type AddThirtyPercentDiscountToLineItemResponse struct {
	OrderEditAddLineItemDiscount struct {
		UserErrors []UserErrors `json:"userErrors"`
	} `json:"orderEditAddLineItemDiscount"`
}

type CommitOrderEditResponse struct {
	OrderEditCommit struct {
		UserErrors []UserErrors `json:"userErrors"`
	} `json:"orderEditCommit"`
}

type DeleteCustomerMetafieldResponse struct {
	MetafieldsDelete struct {
		UserErrors []UserErrors `json:"userErrors"`
	} `json:"metafieldsDelete"`
}

type MarkOrderAsPaidResponse struct {
	Order      Order        `json:"order"`
	UserErrors []UserErrors `json:"userErrors"`
}

// DraftOrder represents a Shopify draft order
type DraftOrder struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Tags          []string  `json:"tags"`
	TotalPriceSet ShopMoney `json:"totalPriceSet"`
	Customer      Customer  `json:"customer"`
}

// GetDraftOrderByIDResponse is the GraphQL response wrapper for a draftOrder query
type GetDraftOrderByIDResponse struct {
	DraftOrder *DraftOrder `json:"draftOrder"`
}

// CompletedOrderNode is the minimal shape draftOrderComplete returns for the
// order it just created.
type CompletedOrderNode struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	LegacyResourceID       string `json:"legacyResourceId"`
	StatusPageUrl          string `json:"statusPageUrl"`
	DisplayFinancialStatus string `json:"displayFinancialStatus"`
}

// CompleteDraftOrderResponse is the GraphQL response wrapper for draftOrderComplete
type CompleteDraftOrderResponse struct {
	DraftOrderComplete struct {
		DraftOrder *struct {
			ID    string              `json:"id"`
			Order *CompletedOrderNode `json:"order"`
		} `json:"draftOrder"`
		UserErrors []UserErrors `json:"userErrors"`
	} `json:"draftOrderComplete"`
}
