package models

// Order represents a Store order Response
type OrderResponse struct {
	ID                       string       `json:"id"`
	Name                     string       `json:"name"`
	StatusPageUrl            string       `json:"statusPageUrl"`
	CreatedAt                string       `json:"createdAt"`
	DisplayFinancialStatus   string       `json:"displayFinancialStatus"`
	DisplayFulfillmentStatus string       `json:"displayFulfillmentStatus"`
	TotalPriceSetUSD         OrderPrice   `json:"totalPriceSetUSD"`
	TotalPriceSetVES         OrderPrice   `json:"totalPriceSetVES"`
	LineItems                []LineItem   `json:"lineItems"`
	Customer                 Customer     `json:"customer"`
	DebitDirect              *DebitDirect `json:"debitDirect,omitempty"`
}

type DebitDirect struct {
	Bank    string `json:"bank"`
	Phone   string `json:"phone"`
	DNI     string `json:"dni"`
	DNIType string `json:"dni_type"`
}

// OrderPrice represents the price details of an order
type OrderPrice struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

// LineItem represents an item in an order
type LineItem struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	SKU      string `json:"sku"`
}

// Customer represents a customer in the store
type Customer struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Phone       string `json:"phone"`
	DNI         string `json:"dni"`
	DNIType     string `json:"dniType"`
}

// UpdateCustomerParentIDRequest represents the request payload to update a customer's parent ID
type UpdateCustomerParentIDRequest struct {
	CustomerID string `json:"customerId" binding:"required"`
	DNI        string `json:"dni"`
	DNIType    string `json:"dniType"`
}
