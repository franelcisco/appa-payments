package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"appa_payments/internal/models"
	helpers "appa_payments/pkg"
	"appa_payments/pkg/bcv"
	"appa_payments/pkg/db"
	dbModels "appa_payments/pkg/db/models"
	"appa_payments/pkg/drive"
	"appa_payments/pkg/r4bank"
	"appa_payments/pkg/shopify"
)

type paymentService struct {
	shopifyRepo shopify.Repository
	r4Repo      r4bank.R4Repository
	bcvClient   bcv.Client
	driveClient drive.Client
	db          *gorm.DB
	location    *time.Location
	logger      *zap.Logger
}

const (
	mobilePaymentGenericErrorMessage  = "error interno al validar el pago, contacte soporte"
	mobilePaymentRegisterErrorMessage = "error interno al registrar su pago, contacte soporte"
	_debitImmediateGenericError       = "ocurrió un error al procesar la solicitud"
)

func NewPaymentService(
	db *gorm.DB,
	shopifyRepo shopify.Repository,
	r4Repo r4bank.R4Repository,
	bcvClient bcv.Client,
	driveClient drive.Client,
	location *time.Location,
	logger *zap.Logger,
) *paymentService {
	return &paymentService{
		shopifyRepo: shopifyRepo,
		r4Repo:      r4Repo,
		bcvClient:   bcvClient,
		driveClient: driveClient,
		db:          db,
		location:    location,
		logger:      logger,
	}
}

// ValidateMobilePayment validates a mobile payment
func (p *paymentService) ValidateMobilePayment(
	ctx context.Context,
	req models.ValidateMobilePaymentRequest,
) *models.MobilePaymentResponse {
	var (
		item      dbModels.R4AppaMobilePayment
		query     = p.db.Model(&dbModels.R4AppaMobilePayment{}).WithContext(ctx).Select("r4_appa_mobile_payments.*")
		count     = 0
		maxintent = 3
		response  = &models.MobilePaymentResponse{Success: false}
		tx        = p.db.Begin()
		errDB     error
	)
	defer db.DBRollback(tx, &errDB)

	// Get BCV Tasa
	BCVTasa, err := p.bcvClient.Get(ctx)
	if err != nil {
		response.Message = mobilePaymentGenericErrorMessage
		return response
	}

	p.logger.Info("validating mobile payment", zap.Any("request", req))

	// Apply filters
	query = p.getMobilePaymentsFilters(query, req)
	for count < maxintent {
		if err := query.Last(&item).Error; err != nil {
			count++
			// Add a 1 second delay before retrying
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}

	if item.ID == 0 {
		p.logger.Error("no mobile payment found with the provided data", zap.Any("filters", req))
		response.Message = "no se encontro ningun pago movil que coincida con los datos proporcionados"
		return response
	}

	// Get order details
	store, err := p.shopifyRepo.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		response.Message = "error interno al validar el pago, contacte soporte"
		return response
	}

	var currentOrderPrice float64
	if value, err := strconv.ParseFloat(store.Order.CurrentTotalPriceSet.ShopMoney.Amount, 64); err == nil {
		currentOrderPrice = value * BCVTasa
	}

	tolerancy := 0.1 * BCVTasa // 0.1 USD in VES
	dni := helpers.GetCustomerDNI(req.DNI, req.DNIType, store.Order.Customer.ParentID)

	if currentOrderPrice > item.Amount+tolerancy {
		response, err := p.mobilePaymentLessTotalAmount(
			ctx, tx, item, store.Order.Name, currentOrderPrice, dni,
		)
		errDB = err
		return response
	}

	orderID, err := strconv.Atoi(strings.TrimPrefix(store.Order.ID, "gid://shopify/Order/"))
	if err != nil {
		p.logger.Error("failed to parse order ID", zap.Error(err))
		response.Message = mobilePaymentRegisterErrorMessage
		return response
	}

	item.OrderID = &orderID
	item.OrderName = req.OrderName
	item.UpdatedAt = time.Now()
	// Amount is within the tolerancy range, proceed to link payment to order
	if err := tx.Save(&item).Error; err != nil {
		response.Message = mobilePaymentRegisterErrorMessage
		return response
	}

	response.Success = true
	if currentOrderPrice < item.Amount-tolerancy {
		response.Message = p.mobilePaymentGreaterTotalAmount(ctx, item, store.Order.Name, currentOrderPrice, dni)
	} else {
		response.Message = "Pago registrado correctamente"
	}

	err = p.markOrderAsPaid(ctx, store.Order.ID)
	if err != nil {
		response.Message = mobilePaymentRegisterErrorMessage
		return response
	}

	if !req.Automatic {
		go p.updateDebitDirectData(ctx, store.Order.Customer.ID, models.DebitDirect{
			Bank:    req.Bank,
			Phone:   req.Phone,
			DNI:     req.DNI,
			DNIType: req.DNIType,
		})
	}

	return response
}

// ValidateDirectDebit validates a direct debit transaction
func (p *paymentService) ValidateDirectDebit(
	ctx context.Context,
	req models.ValidateOTPRequest,
) error {
	// Get BCV Tasa
	BCVTasa, err := p.bcvClient.Get(ctx)
	if err != nil {
		return errors.New(_debitImmediateGenericError)
	}

	// Get order details
	order, err := p.shopifyRepo.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		return errors.New(_debitImmediateGenericError)
	}

	var currentOrderPrice float64
	if value, err := strconv.ParseFloat(order.Order.CurrentTotalPriceSet.ShopMoney.Amount, 64); err == nil {
		currentOrderPrice = value * BCVTasa
	}
	p.logger.Debug("currentOrderPrice", zap.Any("currentOrderPrice", currentOrderPrice))
	r4Resp, err := p.r4Repo.ValidateImmediateDebit(ctx, r4bank.ValidateOTPRequest{
		Bank:    req.Bank,
		Amount:  currentOrderPrice,
		Phone:   req.Phone,
		DNI:     fmt.Sprintf("%s%s", req.DNIType, req.DNI),
		Name:    req.Name,
		OTP:     req.OTP,
		Concept: req.Concept,
	})
	if err != nil {
		p.logger.Error(err.Error())
		return errors.New(_debitImmediateGenericError)
	}

	go p.waitForOperationCompletion(
		r4Resp.ID,
		dbModels.R4AppaDebitDirect{
			SenderPhone: req.Phone,
			IssuingBank: req.Bank,
			Amount:      currentOrderPrice,
			Reference:   r4Resp.Reference,
			DNI:         fmt.Sprintf("%s-%s", req.DNIType, req.DNI),
			Code:        r4Resp.Code,
			Success:     r4Resp.Status,
			OrderName:   order.Order.Name,
			OrderID:     req.OrderID,
			Date:        time.Now().In(p.location),
			CreatedAt:   time.Now(),
		},
	)

	if r4Resp.Code == "AC00" {
		p.logger.Info("debit direct is being processed", zap.Any("response", r4Resp))
		return fmt.Errorf("EN_PROCESO")
	}

	go p.updateDebitDirectData(ctx, order.Order.Customer.ID, models.DebitDirect{
		Bank:    req.Bank,
		Phone:   req.Phone,
		DNI:     req.DNI,
		DNIType: req.DNIType,
	})

	return nil
}

// GenerateOTP generates an OTP for mobile payments
func (p *paymentService) GenerateOTP(
	ctx context.Context,
	req models.OTPRequest,
) error {
	// Get BCV Tasa
	BCVTasa, err := p.bcvClient.Get(ctx)
	if err != nil {
		return err
	}

	// Get order details
	store, err := p.shopifyRepo.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		return err
	}

	var currentOrderPrice float64
	if value, err := strconv.ParseFloat(store.Order.CurrentTotalPriceSet.ShopMoney.Amount, 64); err == nil {
		currentOrderPrice = value * BCVTasa
	}
	p.logger.Info("currentOrderPrice", zap.Any("currentOrderPrice", currentOrderPrice))
	return p.r4Repo.GenerateOTP(ctx, r4bank.OTPRequest{
		Bank:   req.Bank,
		Amount: currentOrderPrice,
		Phone:  req.Phone,
		DNI:    fmt.Sprintf("%s%s", req.DNIType, req.DNI),
	})
}

// updateDebitDirectData updates the debit direct data for a customer
func (p *paymentService) updateDebitDirectData(ctx context.Context, customerID string, json models.DebitDirect) {
	err := p.shopifyRepo.SetDebitDirect(ctx, customerID, shopify.DebitDirectJson{
		Bank:    json.Bank,
		Phone:   json.Phone,
		DNI:     json.DNI,
		DNIType: json.DNIType,
	})
	if err != nil {
		p.logger.Error("failed to update debit direct data", zap.Error(err), zap.Any("customer_id", customerID), zap.Any("json", json))
	}
}

// waitForOperationCompletion waits for the operation to complete
func (p *paymentService) waitForOperationCompletion(
	operationID string,
	log dbModels.R4AppaDebitDirect,
) {
	var intents = 0
	for log.Code == "AC00" && intents < 10 {
		resp, err := p.r4Repo.GetOperationByID(context.Background(), operationID)
		if err != nil {
			p.logger.Error(err.Error())
			log.Code = "ERROR"
			break
		}

		log.Code = resp.Code
		log.Reference = resp.Reference
		log.Success = resp.Success
		if log.Code != "AC00" {
			break
		}

		intents++
		time.Sleep(3 * time.Second)
	}

	if log.Code == "ACCP" {
		p.markOrderAsPaid(context.Background(), log.OrderID)
	}

	p.registerDebitDirectPayment(context.Background(), log)
}

// markOrderAsPaid marks an order as paid in Shopify
func (p *paymentService) markOrderAsPaid(ctx context.Context, orderID string) error {
	err := p.shopifyRepo.MarkOrderAsPaid(
		ctx,
		orderID,
	)
	if err != nil {
		p.logger.Error("failed to mark order as paid", zap.Error(err), zap.Any("order_id", orderID))
		return err
	}

	return nil
}

// registerDebitDirectPayment registers a debit direct payment
func (p *paymentService) registerDebitDirectPayment(
	ctx context.Context,
	req dbModels.R4AppaDebitDirect,
) {
	if err := p.db.WithContext(ctx).Create(&req).Error; err != nil {
		p.logger.Error("failed to register debit direct payment", zap.Error(err))
	}
}

// getMobilePaymentsFilters retrieves mobile payment filters
func (p *paymentService) getMobilePaymentsFilters(query *gorm.DB, filters models.ValidateMobilePaymentRequest) *gorm.DB {

	query = query.Where("order_id IS NULL") // only unlinked payments

	if filters.Bank != "" {
		query = query.Where("issuing_bank = ?", filters.Bank)
	}
	if filters.Phone != "" {
		query = query.Where("sender_phone = ?", filters.Phone)
	}
	if filters.Reference != "" {
		query = query.Where("reference LIKE ?", fmt.Sprintf("%%%s", filters.Reference))
	}

	if filters.Automatic {
		query = query.Where("date = ?", time.Now().In(p.location).Format("2006-01-02"))
	} else if filters.Date != "" {
		query = query.Where("date = ?", filters.Date)
	}

	return query
}

// mobilePaymentLessTotalAmount
func (p *paymentService) mobilePaymentLessTotalAmount(
	ctx context.Context,
	tx *gorm.DB,
	item dbModels.R4AppaMobilePayment,
	orderName string,
	currentOrderPrice float64,
	dni string,
) (*models.MobilePaymentResponse, error) {
	response := &models.MobilePaymentResponse{
		Success: false,
		Message: mobilePaymentGenericErrorMessage,
	}
	p.logger.Info("payment amount is less than order total", zap.String("order", orderName), zap.Float64("order_total", currentOrderPrice), zap.Float64("payment_amount", item.Amount))

	// Delete mobile payment to avoid future conflicts
	err := p.deleteMobilePayment(ctx, tx, item.ID)
	if err != nil {
		return response, err
	}
	// Return money to sender
	err = p.r4Repo.ChangePaid(ctx, r4bank.ChangePaidRequest{
		Bank:    item.IssuingBank,
		Amount:  item.Amount,
		Phone:   item.SenderPhone,
		DNI:     dni,
		Concept: fmt.Sprintf("DMT (%s)", orderName),
	})
	if err != nil {
		p.logger.Error("failed to return money to sender", zap.Error(err), zap.Any("payment", item))
		return response, err
	}

	response.Message = "Debe realizar el pago por el monto exacto de la orden, se ha realizado la devolución del mismo, a los datos utilizados en su pago"
	return response, nil
}

// deleteMobilePayment deletes a mobile payment by ID
func (p *paymentService) deleteMobilePayment(ctx context.Context, tx *gorm.DB, id int) error {
	if err := tx.WithContext(ctx).Delete(&dbModels.R4AppaMobilePayment{}, id).Error; err != nil {
		p.logger.Error("failed to delete mobile payment", zap.Error(err), zap.Any("id", id))
		return err
	}

	p.logger.Info("mobile payment deleted", zap.Any("id", id))
	return nil
}

// mobilePaymentGreaterTotalAmount
func (p *paymentService) mobilePaymentGreaterTotalAmount(
	ctx context.Context, item dbModels.R4AppaMobilePayment, orderName string, currentOrderPrice float64, dni string,
) string {
	p.logger.Error("payment amount is greater than order total", zap.String("order", orderName), zap.Float64("order_total", currentOrderPrice), zap.Float64("payment_amount", item.Amount))

	err := p.r4Repo.ChangePaid(ctx, r4bank.ChangePaidRequest{
		Bank:    item.IssuingBank,
		Amount:  item.Amount - currentOrderPrice,
		Phone:   item.SenderPhone,
		DNI:     dni,
		Concept: fmt.Sprintf("DMT (%s)", orderName),
	})
	if err != nil {
		p.logger.Error("failed to return money to sender", zap.Error(err), zap.Any("payment", item))
		return "su pago fue registrado, pero hubo un error al devolver el excedente, contacte soporte"
	}

	return fmt.Sprintf(
		"el monto del pago fue mayor al total del pedido, se ha realizado la devolución del excedente (Bs.S %.2f), a los datos utilizados en su pago",
		item.Amount-currentOrderPrice,
	)
}

// ValidateMobilePaymentManual validates a manual mobile payment
func (p *paymentService) ValidateMobilePaymentManual(
	ctx context.Context,
	req models.ValidateMobilePaymentManualRequest,
) error {
	var dbError error

	order, err := p.shopifyRepo.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		return err // or custom error
	}

	tasaBCV, err := p.bcvClient.Get(ctx)
	if err != nil {
		return err
	}

	orderID, err := strconv.Atoi(strings.TrimPrefix(order.Order.ID, "gid://shopify/Order/"))
	if err != nil {
		p.logger.Error(err.Error(), zap.Any("order", order))
		return errors.New("invalid order ID")
	}

	var amount float64
	if value, err := strconv.ParseFloat(order.Order.CurrentTotalPriceSet.ShopMoney.Amount, 64); err == nil {
		amount = value
	}

	manualOrder := dbModels.ManualOrder{
		OrderName:        req.OrderName,
		OrderID:          orderID,
		Amount:           amount * tasaBCV,
		OrderTotalAmount: amount,
		ValidateStatus:   "PENDING",
		PaymentMethodID:  4, // Pago Móvil
	}

	url, err := p.driveClient.UploadFile(ctx, req.BillImageFile)
	if err != nil {
		return err
	}
	defer func() {
		if dbError == nil {
			return
		}

		err := p.driveClient.DeleteFile(ctx, url)
		if err != nil {
			p.logger.Error("failed to delete file from google drive", zap.Any("url", url))
		}
	}()

	manualOrder.BillImageURL = url

	dbError = p.db.Create(&manualOrder).Error
	if dbError != nil {
		p.logger.Error(dbError.Error(), zap.Any("order", manualOrder))
		return dbError
	}

	return nil
}
