package domains

import "context"

type WebhookService interface {
	OrdersCreated(ctx context.Context, orderId string) error
}
