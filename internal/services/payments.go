package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	helpers "appa_payments/pkg"
	"appa_payments/pkg/bcv"
	"appa_payments/pkg/db"
	dbModels "appa_payments/pkg/db/models"
	"appa_payments/pkg/drive"
	"appa_payments/pkg/mailgun"
	"appa_payments/pkg/r4bank"
	"appa_payments/pkg/shopify"
)

type paymentService struct {
	shopifyRepo               shopify.Repository
	r4Repo                    r4bank.R4Repository
	bcvClient                 bcv.Client
	driveClient               drive.Client
	mailgunRepo               mailgun.Repository
	db                        *gorm.DB
	location                  *time.Location
	logger                    *zap.Logger
	otpCache                  *otpCache
	recurrentDirectDebitAppID string
}

const (
	mobilePaymentGenericErrorMessage  = "error interno al validar el pago, contacte soporte"
	mobilePaymentRegisterErrorMessage = "error interno al registrar su pago, contacte soporte"
	_debitImmediateGenericError       = "ocurrió un error al procesar la solicitud"
)

func (p *paymentService) debitImmediateGenericError() error {
	return errors.New(_debitImmediateGenericError)
}

func NewPaymentService(
	db *gorm.DB,
	shopifyRepo shopify.Repository,
	r4Repo r4bank.R4Repository,
	bcvClient bcv.Client,
	driveClient drive.Client,
	mailgunRepo mailgun.Repository,
	location *time.Location,
	recurrentDirectDebitAppID string,
	logger *zap.Logger,
) *paymentService {
	return &paymentService{
		shopifyRepo:               shopifyRepo,
		r4Repo:                    r4Repo,
		bcvClient:                 bcvClient,
		driveClient:               driveClient,
		mailgunRepo:               mailgunRepo,
		db:                        db,
		location:                  location,
		logger:                    logger,
		otpCache:                  newOTPCache(),
		recurrentDirectDebitAppID: recurrentDirectDebitAppID,
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

	orderType := models.OrderTypeOrDefault(req.TypeOrder)

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

	target, err := p.GetChargeableByID(ctx, req.OrderID, orderType)
	if err != nil {
		response.Message = "error interno al validar el pago, contacte soporte"
		return response
	}

	var currentOrderPrice float64
	if value, err := strconv.ParseFloat(target.AmountUSD, 64); err == nil {
		currentOrderPrice = value * BCVTasa
	}

	dni := helpers.GetCustomerDNI(req.DNI, req.DNIType, target.Customer.ParentID)
	verdict := domains.ClassifyCharge(currentOrderPrice, item.Amount, BCVTasa)

	if verdict == domains.Underpaid {
		response, err := p.mobilePaymentLessTotalAmount(
			ctx, tx, item, target.Name, currentOrderPrice, dni,
		)
		errDB = err
		return response
	}

	orderID, err := strconv.Atoi(req.OrderID)
	if err != nil {
		p.logger.Error("failed to parse order ID", zap.Error(err))
		response.Message = mobilePaymentRegisterErrorMessage
		return response
	}

	item.OrderID = &orderID
	item.OrderName = req.OrderName
	item.UpdatedAt = time.Now()
	if err := tx.Save(&item).Error; err != nil {
		response.Message = mobilePaymentRegisterErrorMessage
		return response
	}

	response.Success = true
	if verdict == domains.Overpaid {
		response.Message = p.mobilePaymentGreaterTotalAmount(ctx, item, target.Name, currentOrderPrice, dni)
	} else {
		response.Message = "Pago registrado correctamente"
	}

	completed, err := p.finalizeCharge(ctx, target, nil)
	if err != nil && !errors.Is(err, ErrDraftChargedNotCompleted) {
		response.Message = mobilePaymentRegisterErrorMessage
		return response
	}
	if completed != nil {
		if realOrderID, err := strconv.Atoi(completed.LegacyOrderID); err == nil {
			item.OrderID = &realOrderID
			item.OrderName = completed.Name
			item.UpdatedAt = time.Now()
			if err := tx.Save(&item).Error; err != nil {
				p.logger.Error("failed to update mobile payment with completed order id", zap.Error(err), zap.Int("paymentId", item.ID))
			}
		} else {
			p.logger.Error("failed to parse completed order legacy id", zap.Error(err), zap.String("legacyOrderId", completed.LegacyOrderID))
		}
	}

	if !req.Automatic {
		go p.updateDebitDirectData(ctx, target.Customer.ID, models.DebitDirect{
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
	orderType := models.OrderTypeOrDefault(req.TypeOrder)

	// Get BCV Tasa
	BCVTasa, err := p.bcvClient.Get(ctx)
	if err != nil {
		return errors.New(_debitImmediateGenericError)
	}

	target, err := p.GetChargeableByID(ctx, req.OrderID, orderType)
	if err != nil {
		return errors.New(_debitImmediateGenericError)
	}

	var currentOrderPrice float64
	if value, err := strconv.ParseFloat(target.AmountUSD, 64); err == nil {
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
			OrderName:   target.Name,
			OrderID:     req.OrderID,
			OrderType:   string(orderType),
			Date:        time.Now().In(p.location),
			CreatedAt:   time.Now(),
		},
	)

	if domains.IsR4BreakCode(r4Resp.Code) {
		p.logger.Warn("debit direct is being processed", zap.Any("response", r4Resp), zap.Any("order", target.Name))
		return fmt.Errorf("EN_PROCESO")
	}

	go p.updateDebitDirectData(ctx, target.Customer.ID, models.DebitDirect{
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

	target, err := p.GetChargeableByID(ctx, req.OrderID, models.OrderTypeOrDefault(req.TypeOrder))
	if err != nil {
		return err
	}

	var currentOrderPrice float64
	if value, err := strconv.ParseFloat(target.AmountUSD, 64); err == nil {
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
	intents := 0
	for domains.IsR4BreakCode(log.Code) && intents < 10 {
		resp, err := p.r4Repo.GetOperationByID(context.Background(), operationID)
		if err != nil {
			p.logger.Error(err.Error())
			log.Code = "ERROR"
			break
		}

		log.Code = resp.Code
		log.Reference = resp.Reference
		log.Success = resp.Success
		if !domains.IsR4BreakCode(log.Code) {
			break
		}

		intents++
		time.Sleep(3 * time.Second)
	}

	if log.Code == domains.R4CodeApproved {
		orderType := models.OrderType(log.OrderType)
		if orderType == "" {
			orderType = models.OrderTypeComplete
		}
		target := &Chargeable{Type: orderType, GID: log.OrderID, Name: log.OrderName}
		completed, err := p.finalizeCharge(context.Background(), target, nil)
		if err != nil && !errors.Is(err, ErrDraftChargedNotCompleted) {
			p.logger.Error("failed to finalize debit direct completion", zap.Error(err), zap.Any("order_name", log.OrderName))
		}
		if completed != nil {
			log.OrderID = completed.LegacyOrderID
			log.OrderName = completed.Name
		}
	}

	p.logger.Info("debit direct operation completed", zap.Any("log", log), zap.Any("response_code", log.Code))

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

// Chargeable is a unified view over a chargeable Order or DraftOrder.
type Chargeable struct {
	Type      models.OrderType
	GID       string
	Name      string
	AmountUSD string
	Customer  shopify.Customer
	Tags      []string
	App       *shopify.App // only set for Complete; a draft has no App yet
}

// GetChargeableByID resolves an Order or a DraftOrder into one common shape.
func (p *paymentService) GetChargeableByID(
	ctx context.Context, id string, orderType models.OrderType,
) (*Chargeable, error) {
	switch orderType {
	case models.OrderTypeDraft:
		resp, err := p.shopifyRepo.GetDraftOrderByID(ctx, id)
		if err != nil {
			return nil, err
		}
		d := resp.DraftOrder
		return &Chargeable{
			Type:      models.OrderTypeDraft,
			GID:       d.ID,
			Name:      d.Name,
			AmountUSD: d.TotalPriceSet.ShopMoney.Amount,
			Customer:  d.Customer,
			Tags:      d.Tags,
		}, nil
	case models.OrderTypeCart:
		return nil, errors.New("cart order type is not supported")
	default:
		resp, err := p.shopifyRepo.GetOrderByID(ctx, id)
		if err != nil {
			return nil, err
		}
		o := resp.Order
		return &Chargeable{
			Type:      models.OrderTypeComplete,
			GID:       o.ID,
			Name:      o.Name,
			AmountUSD: o.CurrentTotalPriceSet.ShopMoney.Amount,
			Customer:  o.Customer,
			Tags:      o.Tags,
			App:       o.App,
		}, nil
	}
}

// ErrDraftChargedNotCompleted means the charge already succeeded but turning
// the draft into a real order failed — callers must not report failure to
// the buyer when they see this.
var ErrDraftChargedNotCompleted = errors.New("payment succeeded but order finalization failed")

// finalizeCharge marks target paid; for a draft, tags land before
// CompleteDraftOrder runs, since the draft locks once it becomes an order.
func (p *paymentService) finalizeCharge(
	ctx context.Context,
	target *Chargeable,
	tags []string,
) (*shopify.CompletedOrder, error) {
	if target.Type != models.OrderTypeDraft {
		return nil, p.markOrderAsPaid(ctx, target.GID)
	}

	if len(tags) > 0 {
		if err := p.shopifyRepo.AddDraftOrderTags(ctx, target.GID, tags); err != nil {
			p.logger.Error("failed to tag draft order", zap.Error(err), zap.String("draftId", target.GID))
		}
	}

	completed, err := p.shopifyRepo.CompleteDraftOrder(ctx, target.GID, false)
	if err != nil {
		p.logger.Error("failed to complete draft order after successful charge", zap.Error(err), zap.String("draftId", target.GID))
		p.alertDraftFinalizationFailed(ctx, target, "se cobró pero no se pudo completar el pedido", err)
		return nil, fmt.Errorf("%w: %v", ErrDraftChargedNotCompleted, err)
	}

	if completed.DisplayFinancialStatus != "PAID" {
		if err := p.shopifyRepo.MarkOrderAsPaid(ctx, completed.OrderGID); err != nil {
			p.logger.Error("failed to mark completed order as paid", zap.Error(err), zap.String("orderId", completed.OrderGID))
		} else {
			completed.DisplayFinancialStatus = "PAID"
		}
	}
	return completed, nil
}

func (p *paymentService) alertDraftFinalizationFailed(ctx context.Context, target *Chargeable, reason string, cause error) {
	if mailErr := p.mailgunRepo.SendSupportAlert(ctx, mailgun.SupportAlertRequest{
		OrderName: target.Name,
		Message:   fmt.Sprintf("%s: %v", reason, cause),
	}); mailErr != nil {
		p.logger.Error("failed to send support alert email", zap.Error(mailErr), zap.String("draftId", target.GID))
	}
}

func stripOrderGIDPrefix(gid string) string {
	gid = strings.ReplaceAll(gid, shopify.OrderKindID, "")
	gid = strings.ReplaceAll(gid, shopify.DraftOrderKindID, "")
	return gid
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

	go p.registerMobilePaymentReversal(item, orderName, currentOrderPrice, item.Amount, "LESS", nil)

	response.Message = "Debe realizar el pago por el monto exacto de la orden, se ha realizado la devolución del mismo, a los datos utilizados en su pago"
	return response, nil
}

// registerMobilePaymentReversal records a reversal result (success or error)
func (p *paymentService) registerMobilePaymentReversal(item dbModels.R4AppaMobilePayment, orderName string, orderAmount, reversalAmount float64, reason string, changePaidErr error) {
	record := dbModels.R4AppaMobilePaymentReversal{
		Reference:      item.Reference,
		OrderName:      orderName,
		OrderAmount:    orderAmount,
		ReversalAmount: reversalAmount,
		Reason:         reason,
		Success:        changePaidErr == nil,
	}
	if changePaidErr != nil {
		record.ErrorDetail = changePaidErr.Error()
	}
	if err := p.db.Create(&record).Error; err != nil {
		p.logger.Error("failed to register mobile payment reversal", zap.Error(err), zap.Any("record", record))
	}
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

	amount := item.Amount - currentOrderPrice
	err := p.r4Repo.ChangePaid(ctx, r4bank.ChangePaidRequest{
		Bank:    item.IssuingBank,
		Amount:  amount,
		Phone:   item.SenderPhone,
		DNI:     dni,
		Concept: fmt.Sprintf("DMT (%s)", orderName),
	})
	if err != nil {
		p.logger.Error("failed to return money to sender", zap.Error(err), zap.Any("payment", item))
		return "su pago fue registrado, pero hubo un error al devolver el excedente, contacte soporte"
	}

	go p.registerMobilePaymentReversal(item, orderName, currentOrderPrice, amount, "GREATER", nil)

	return fmt.Sprintf(
		"el monto del pago fue mayor al total del pedido, se ha realizado la devolución del excedente (Bs.S %.2f), a los datos utilizados en su pago",
		amount,
	)
}

// ValidateMobilePaymentManual validates a manual mobile payment
func (p *paymentService) ValidateMobilePaymentManual(
	ctx context.Context,
	req models.ValidateMobilePaymentManualRequest,
) error {
	var dbError error

	target, err := p.GetChargeableByID(ctx, req.OrderID, models.OrderTypeOrDefault(req.TypeOrder))
	if err != nil {
		return err // or custom error
	}

	tasaBCV, err := p.bcvClient.Get(ctx)
	if err != nil {
		return err
	}

	orderID, err := strconv.Atoi(req.OrderID)
	if err != nil {
		p.logger.Error(err.Error(), zap.Any("order", target))
		return errors.New("invalid order ID")
	}

	var amount float64
	if value, err := strconv.ParseFloat(target.AmountUSD, 64); err == nil {
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

// RequestDirectDebitAccountOTP generates a 6-digit OTP, stores it in the cache,
// and sends it to the customer's email address associated with the given order.
func (p *paymentService) RequestDirectDebitAccountOTP(ctx context.Context, orderID string, typeOrder *models.OrderType) error {
	target, err := p.GetChargeableByID(ctx, orderID, models.OrderTypeOrDefault(typeOrder))
	if err != nil {
		p.logger.Error("failed to get order for OTP request", zap.Error(err), zap.String("orderID", orderID))
		return errors.New(_debitImmediateGenericError)
	}

	code, err := generateOTPCode()
	if err != nil {
		p.logger.Error("failed to generate OTP code", zap.Error(err))
		return errors.New(_debitImmediateGenericError)
	}

	p.otpCache.Set(orderID, code)

	return p.mailgunRepo.SendOTPEmail(ctx, mailgun.OTPEmailRequest{
		To:                target.Customer.Email,
		OTPCode:           code,
		ExpirationMinutes: int(otpTTL.Minutes()),
		UserName:          target.Customer.DisplayName,
	})
}

// DirectDebitAccount processes a direct debit account charge using the provided account number.
func (p *paymentService) DirectDebitAccount(
	ctx context.Context,
	req models.DirectDebitAccountRequest,
) (*models.ProcessDirectDebitAccountResponse, error) {
	target, err := p.GetChargeableByID(ctx, req.OrderID, models.OrderTypeOrDefault(req.TypeOrder))
	if err != nil {
		p.logger.Error("failed to get order from Shopify", zap.Error(err), zap.String("orderID", req.OrderID))
		return nil, p.debitImmediateGenericError()
	}

	// Business rules: refuse if customer already affiliated.
	if target.Customer.DirectDebitAccount != nil {
		p.logger.Error("customer already has a direct debit account",
			zap.String("customerID", target.Customer.ID),
			zap.Any("DirectDebitAccount", target.Customer.DirectDebitAccount))
		return nil, errors.New(_debitImmediateGenericError)
	}

	amount, err := helpers.StringToFloat64(target.AmountUSD)
	if err != nil {
		p.logger.Error("failed to parse order total price", zap.Error(err),
			zap.String("order", target.Name),
			zap.String("price", target.AmountUSD))
		return nil, errors.New(_debitImmediateGenericError)
	}

	resp, record, err := p.processDirectDebitAccount(ctx, domains.DirectDebitAccountRequest{
		OrderID:     target.GID,
		Account:     req.Account,
		DNI:         req.DNI,
		DisplayName: target.Customer.DisplayName,
		CustomerID:  target.Customer.ID,
		OrderName:   target.Name,
		Amount:      amount,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return resp, nil
	}

	if err := p.shopifyRepo.SetCustomerDebitDirectAccount(ctx, target.Customer.ID, shopify.DebitDirectAccountJson{
		Account: req.Account,
		DNI:     req.DNI,
	}); err != nil {
		p.logger.Error("failed to update debit direct account data", zap.Error(err), zap.String("order", target.Name), zap.Any("customer_id", target.Customer.ID))
		return nil, errors.New(_debitImmediateGenericError)
	}

	completed, err := p.finalizeCharge(ctx, target, nil)
	if err != nil && !errors.Is(err, ErrDraftChargedNotCompleted) {
		p.logger.Error("failed to mark order as paid", zap.Error(err), zap.String("order", target.Name))
	}
	p.updateDirectDebitAccountRecordAfterCompletion(ctx, record, completed)

	return &models.ProcessDirectDebitAccountResponse{
		Success: true,
		Code:    domains.ResponseCodeOK,
	}, nil
}

// DirectDebitAccountWithOTP processes a direct debit account charge using an OTP for authentication.
func (p *paymentService) DirectDebitAccountWithOTP(
	ctx context.Context,
	req models.DirectDebitAccountWithOTPRequest,
) (*models.ProcessDirectDebitAccountResponse, error) {
	var isRecurrentAppOrder bool

	target, err := p.GetChargeableByID(ctx, req.OrderID, models.OrderTypeOrDefault(req.TypeOrder))
	if err != nil {
		p.logger.Error("failed to get order from Shopify", zap.Error(err), zap.String("orderID", req.OrderID))
		return nil, p.debitImmediateGenericError()
	}

	if target.Customer.DirectDebitAccount == nil || target.Customer.DirectDebitAccount.JsonValue == nil {
		p.logger.Warn("customer does not have a direct debit account", zap.String("customerID", target.Customer.ID))
		return &models.ProcessDirectDebitAccountResponse{
			Success: false,
			Code:    domains.ResponseCodeAffiliationExists,
		}, nil
	}

	isRecurrentAppOrder = target.App != nil && target.App.IsID(p.recurrentDirectDebitAppID)
	if !isRecurrentAppOrder && !p.otpCache.Validate(req.OrderID, req.OTP) {
		p.logger.Warn("invalid OTP", zap.String("orderID", req.OrderID))
		return &models.ProcessDirectDebitAccountResponse{Success: false, Code: domains.ResponseCodeInvalidOTP}, nil
	}

	var directDebit models.DirectDebitAccount
	if err := json.Unmarshal([]byte(target.Customer.DirectDebitAccount.JsonValue), &directDebit); err != nil {
		p.logger.Error("failed to unmarshal direct debit account", zap.Error(err), zap.Any("json", target.Customer.DirectDebitAccount.JsonValue))
		return nil, errors.New(_debitImmediateGenericError)
	}

	amount, err := helpers.StringToFloat64(target.AmountUSD)
	if err != nil {
		p.logger.Error("failed to parse order total price", zap.Error(err),
			zap.String("order", target.Name),
			zap.String("price", target.AmountUSD))
		return nil, errors.New(_debitImmediateGenericError)
	}

	resp, record, err := p.processDirectDebitAccount(ctx, domains.DirectDebitAccountRequest{
		Amount:      amount,
		Account:     directDebit.Account,
		DNI:         directDebit.DNI,
		DisplayName: target.Customer.DisplayName,
		CustomerID:  target.Customer.ID,
		OrderName:   target.Name,
		OrderID:     target.GID,
		IsRecurring: isRecurrentAppOrder,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		if domains.IsAffiliationPending(resp.Code) {
			if err := p.clearDirectDebitAccount(ctx, target.Customer.ID); err != nil {
				p.logger.Error("failed to clear direct debit account data", zap.Error(err), zap.String("customerID", target.Customer.ID))
			}
		}
		return resp, nil
	}

	completed, err := p.finalizeCharge(ctx, target, nil)
	if err != nil && !errors.Is(err, ErrDraftChargedNotCompleted) {
		p.logger.Error("failed to mark order as paid", zap.Error(err), zap.String("order", target.Name))
	}
	p.updateDirectDebitAccountRecordAfterCompletion(ctx, record, completed)

	return resp, nil
}

// processDirectDebitAccount processes a direct debit account charge against R4.
// Returns success (ACCP) or an internal error code (ERR0X).
func (p *paymentService) processDirectDebitAccount(
	ctx context.Context,
	req domains.DirectDebitAccountRequest,
) (*models.ProcessDirectDebitAccountResponse, *dbModels.R4DebitDirectAccount, error) {
	BCVTasa, err := p.bcvClient.Get(ctx)
	if err != nil {
		return nil, nil, p.debitImmediateGenericError()
	}
	req.Amount = BCVTasa * req.Amount

	r4Resp, err := p.r4Repo.DirectDebitAccount(ctx, r4bank.DirectDebitAccountRequest{
		Account: req.Account,
		DNI:     req.DNI,
		Name:    req.DisplayName,
		Amount:  req.Amount,
		Concept: "Prueba",
	})
	if err != nil {
		p.logger.Error("direct debit account call failed", zap.Error(err))
		return nil, nil, errors.New(_debitImmediateGenericError)
	}

	record, err := p.registerDirectDebitAccountResult(ctx, req, r4Resp)
	if err != nil {
		p.logger.Error("failed to register direct debit account result", zap.Error(err), zap.Any("order_name", req.OrderName), zap.String("r4_code", r4Resp.Code))
	}

	if r4Resp.Code == domains.R4CodeApproved {
		return &models.ProcessDirectDebitAccountResponse{
			Success:   true,
			Code:      domains.ResponseCodeOK,
			Reference: r4Resp.Reference,
			OrderName: req.OrderName,
		}, record, nil
	}

	if internalCode, ok := domains.DirectDebitAccountResponseCode(r4Resp.Code); ok {
		return &models.ProcessDirectDebitAccountResponse{
			Success:   false,
			Code:      internalCode,
			Reference: r4Resp.Reference,
			OrderName: req.OrderName,
		}, record, nil
	}

	return nil, record, errors.New(_debitImmediateGenericError)
}

// registerDirectDebitAccountResult stores the R4 charge result and returns
// the row so it can be updated once finalizeCharge completes a draft.
func (p *paymentService) registerDirectDebitAccountResult(ctx context.Context, req domains.DirectDebitAccountRequest, r4Resp *r4bank.DirectDebitAccountResponse) (*dbModels.R4DebitDirectAccount, error) {
	result := &dbModels.R4DebitDirectAccount{
		StoreClientID: strings.ReplaceAll(req.CustomerID, shopify.CustomerKindID, ""),
		Amount:        req.Amount,
		Account:       req.Account[len(req.Account)-4:],
		Code:          r4Resp.Code,
		Reference:     r4Resp.Reference,
		CreatedAt:     time.Now(),
		Success:       r4Resp.Code == domains.R4CodeApproved,
		OrderName:     req.OrderName,
		OrderID:       stripOrderGIDPrefix(req.OrderID),
		IsRecurring:   req.IsRecurring,
		DNI:           req.DNI,
		Date:          time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := p.db.WithContext(ctx).Create(result).Error; err != nil {
		return nil, err
	}

	return result, nil
}

func (p *paymentService) updateDirectDebitAccountRecordAfterCompletion(
	ctx context.Context, record *dbModels.R4DebitDirectAccount, completed *shopify.CompletedOrder,
) {
	if record == nil || completed == nil {
		return
	}
	record.OrderID = completed.LegacyOrderID
	record.OrderName = completed.Name
	record.UpdatedAt = time.Now()
	if err := p.db.WithContext(ctx).Save(record).Error; err != nil {
		p.logger.Error("failed to update direct debit account record with completed order id", zap.Error(err), zap.Int("recordId", record.ID))
	}
}

// clearDirectDebitAccount removes the direct debit account metafield for the given customer.
func (p *paymentService) clearDirectDebitAccount(ctx context.Context, customerID string) error {
	if err := p.shopifyRepo.DeleteCustomerDebitDirectAccount(ctx, customerID); err != nil {
		p.logger.Error("failed to clear direct debit account data", zap.Error(err), zap.Any("customer_id", customerID))
		return err
	}
	return nil
}

// HasSuccessfulRecurrentCharge reports whether there is a successful direct debit charge associated with the given order ID.
func (p *paymentService) HasSuccessfulRecurrentCharge(ctx context.Context, orderID string) (bool, error) {
	numericID := strings.ReplaceAll(orderID, shopify.OrderKindID, "")
	var count int64
	err := p.db.WithContext(ctx).
		Model(&dbModels.R4DebitDirectAccount{}).
		Where(&dbModels.R4DebitDirectAccount{OrderID: numericID, Success: true}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
