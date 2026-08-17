package models

// Webhook is the minimal Shopify orders/create payload shape used by the
// webhook handler. Only the order ID is consumed: the handler enqueues a job
// that re-fetches the full order from Shopify before processing it.
type Webhook struct {
	ID    int `json:"id"`
	AppID int `json:"app_id"`
}
