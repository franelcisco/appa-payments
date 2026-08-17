package handlers

import (
	"context"
	"net/http"
	"strconv"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	webhookJobBuffer = 32
	webhookWorkers   = 4
)

type WebhookHandler struct {
	IsRecurrentAppID string
	WebhookService   domains.WebhookService
	jobQueue         chan webhookJob
	logger           *zap.Logger
}

type webhookJob struct {
	OrderID int
}

// NewWebhookHandler builds the handler and starts the background worker pool.
// Workers run for the lifetime of the process.
func NewWebhookHandler(isRecurrentAppID string, webhookService domains.WebhookService, logger *zap.Logger) *WebhookHandler {
	h := &WebhookHandler{
		IsRecurrentAppID: isRecurrentAppID,
		WebhookService:   webhookService,
		jobQueue:         make(chan webhookJob, webhookJobBuffer),
		logger:           logger,
	}
	for i := range webhookWorkers {
		go h.worker(i)
	}
	return h
}

func (h *WebhookHandler) worker(id int) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("webhook worker panicked", zap.Int("workerID", id), zap.Any("panic", r))
		}
	}()
	for job := range h.jobQueue {
		orderID := strconv.Itoa(job.OrderID)
		if err := h.WebhookService.OrdersCreated(context.Background(), orderID); err != nil {
			h.logger.Error("webhook: OrdersCreated returned error",
				zap.Int("workerID", id),
				zap.String("orderID", orderID),
				zap.Error(err))
		}
	}
}

// HandleOrdersCreated binds the Shopify orders/create payload, enqueues a job
// for asynchronous processing, and always returns 200 so Shopify does not retry.
// HMAC validation is performed upstream by the middleware on the route.
func (h *WebhookHandler) HandleOrdersCreated(c *gin.Context) {
	var payload models.Webhook
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.logger.Error("webhook: failed to bind JSON", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if strconv.Itoa(payload.AppID) != h.IsRecurrentAppID {
		h.logger.Info("webhook: received order created for different app, ignoring", zap.Int("payloadAppID", payload.AppID), zap.String("expectedAppID", h.IsRecurrentAppID))
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	h.jobQueue <- webhookJob{OrderID: payload.ID}

	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}
