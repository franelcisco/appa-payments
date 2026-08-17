package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"appa_payments/internal/domains"
	"appa_payments/internal/models"
	"appa_payments/pkg/bcv"
	dbModels "appa_payments/pkg/db/models"
	"appa_payments/pkg/mailgun"
	"appa_payments/pkg/r4bank"
	"appa_payments/pkg/shopify"
)

type cartPaymentService struct {
	shopifyRepo shopify.Repository
	r4Repo      r4bank.R4Repository
	bcvClient   bcv.Client
	db          *gorm.DB
	logger      *zap.Logger
	location    *time.Location
	mailgunRepo mailgun.Repository
	otpCache    *otpCache
}

const (
	cartPaymentMethodDirectDebit        = "direct_debit"
	cartPaymentMethodMobilePayment      = "mobile_payment"
	cartPaymentMethodDirectDebitAccount = "direct_debit_account"
)

func NewCartPaymentService(
	shopifyRepo shopify.Repository,
	r4Repo r4bank.R4Repository,
	bcvClient bcv.Client,
	db *gorm.DB,
	location *time.Location,
	mailgunRepo mailgun.Repository,
	logger *zap.Logger,
) *cartPaymentService {
	return &cartPaymentService{
		shopifyRepo: shopifyRepo,
		r4Repo:      r4Repo,
		bcvClient:   bcvClient,
		db:          db,
		location:    location,
		mailgunRepo: mailgunRepo,
		logger:      logger,
		otpCache:    newOTPCache(),
	}
}

// parseCartIDAndKey splits a "gid://shopify/Cart/<id>?key=<key>" cart quote
// id into its plain id and key, for use in refund concepts/reversal records.
func (s *cartPaymentService) parseCartIDAndKey(cartQuoteID string) (string, string, error) {
	cartQuoteID = strings.TrimSpace(cartQuoteID)
	cartQuoteID = strings.ReplaceAll(cartQuoteID, "gid://shopify/Cart/", "")
	cartQuoteID = strings.ReplaceAll(cartQuoteID, "key=", "")
	parts := strings.Split(cartQuoteID, "?")
	if len(parts) != 2 {
		return "", "", errors.New("invalid cart quote format")
	}
	return parts[0], parts[1], nil
}

func (s *cartPaymentService) amountVES(ctx context.Context, quote models.CartQuote) (float64, error) {
	tasa, err := s.bcvClient.Get(ctx)
	if err != nil {
		return 0, err
	}
	return quote.Amount * tasa, nil
}

// GenerateOTP generates an OTP for a cart payment
func (s *cartPaymentService) GenerateOTP(
	ctx context.Context,
	quote models.CartQuote,
	req models.CartOTPRequest,
) error {
	amount, err := s.amountVES(ctx, quote)
	if err != nil {
		return err
	}

	return s.r4Repo.GenerateOTP(ctx, r4bank.OTPRequest{
		Bank:   req.Bank,
		Amount: amount,
		Phone:  req.Phone,
		DNI:    fmt.Sprintf("%s%s", req.DNIType, req.DNI),
	})
}

// awaitOperation polls R4 for the final status of a débito inmediato charge,
// which may be pending for several seconds after the initial ValidateImmediateDebit call returns.
func (s *cartPaymentService) awaitOperation(
	resp *r4bank.ValidateDebitInmediateResponse,
) (code, reference string, success bool) {
	code, reference, success = resp.Code, resp.Reference, resp.Status

	for intents := 0; domains.IsR4BreakCode(code) && intents < 15; intents++ {
		time.Sleep(3 * time.Second)

		op, err := s.r4Repo.GetOperationByID(context.Background(), resp.ID)
		if err != nil {
			s.logger.Error(err.Error())
			return "ERROR", reference, false
		}
		code, reference, success = op.Code, op.Reference, op.Success
	}

	return code, reference, success
}

func (s *cartPaymentService) registerDebitDirectPayment(ctx context.Context, req dbModels.R4AppaDebitDirect) {
	if err := s.db.WithContext(ctx).Create(&req).Error; err != nil {
		s.logger.Error("failed to register cart debit direct payment", zap.Error(err))
	}
}

// ValidateDirectDebit validates the débito inmediato charge for a cart.
func (s *cartPaymentService) ValidateDirectDebit(
	ctx context.Context,
	quote models.CartQuote,
	req models.CartValidateOTPRequest,
) (*models.CartDirectDebitResult, error) {
	amount, err := s.amountVES(ctx, quote)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, errors.New(_debitImmediateGenericError)
	}

	r4Resp, err := s.r4Repo.ValidateImmediateDebit(ctx, r4bank.ValidateOTPRequest{
		Bank:    req.Bank,
		Amount:  amount,
		Phone:   req.Phone,
		DNI:     fmt.Sprintf("%s%s", req.DNIType, req.DNI),
		Name:    req.Name,
		OTP:     req.OTP,
		Concept: req.Concept,
	})
	if err != nil {
		s.logger.Error(err.Error())
		return nil, errors.New(_debitImmediateGenericError)
	}

	code, reference, success := s.awaitOperation(r4Resp)

	s.registerDebitDirectPayment(context.Background(), dbModels.R4AppaDebitDirect{
		SenderPhone: req.Phone,
		IssuingBank: req.Bank,
		Amount:      amount,
		Reference:   reference,
		DNI:         fmt.Sprintf("%s-%s", req.DNIType, req.DNI),
		Code:        code,
		Success:     success,
		CartID:      quote.CartID,
		OrderType:   string(models.OrderTypeCart),
		Date:        time.Now().In(s.location),
		CreatedAt:   time.Now(),
	})

	return &models.CartDirectDebitResult{
		Success:   code == domains.R4CodeApproved,
		Code:      code,
		Reference: reference,
		Message:   r4Resp.Message,
	}, nil
}

// AttachOrder backfills the Shopify order id/name a cart-keyed charge became,
// once create-order-from-cart has minted it. Never called by the browser.
func (s *cartPaymentService) AttachOrder(ctx context.Context, cartID string, req models.CartAttachOrderRequest) error {
	query := s.db.WithContext(ctx)

	values := map[string]any{
		"order_id":   req.OrderID,
		"order_name": req.OrderName,
	}

	switch req.PaymentMethod {
	case cartPaymentMethodDirectDebit:
		query = query.Model(&dbModels.R4AppaDebitDirect{})
	case cartPaymentMethodMobilePayment:
		query = query.Model(&dbModels.R4AppaMobilePayment{})
	case cartPaymentMethodDirectDebitAccount:
		query = query.Model(&dbModels.R4DebitDirectAccount{})
		values["store_client_id"] = strings.ReplaceAll(req.ClientID, shopify.CustomerKindID, "")
	default:
		return fmt.Errorf("unsupported payment method: %s", req.PaymentMethod)
	}

	result := query.Where("cart_id = ? AND reference = ?", cartID, req.Reference).Updates(values)
	if result.Error != nil {
		s.logger.Error("failed to attach order to cart payment", zap.Error(result.Error))
		return errors.New("failed to attach order to payment")
	}
	if result.RowsAffected == 0 {
		s.logger.Warn("attach-order matched no row", zap.String("cartId", cartID), zap.String("reference", req.Reference))
		return errors.New("failed to attach order to payment")
	}

	return nil
}

func (s *cartPaymentService) getCartMobilePaymentFilters(
	query *gorm.DB,
	req models.CartValidateMobilePaymentRequest,
) *gorm.DB {
	query = query.Where("order_id IS NULL AND cart_id IS NULL")

	if req.Bank != "" {
		query = query.Where("issuing_bank = ?", req.Bank)
	}
	if req.Phone != "" {
		query = query.Where("sender_phone = ?", req.Phone)
	}
	if req.Reference != "" {
		query = query.Where("reference LIKE ?", fmt.Sprintf("%%%s", req.Reference))
	}
	if req.Automatic {
		query = query.Where("date = ?", time.Now().In(s.location).Format("2006-01-02"))
	} else if req.Date != "" {
		query = query.Where("date = ?", req.Date)
	}

	return query
}

func (s *cartPaymentService) registerMobilePaymentReversal(
	cartID string, amount, reversalAmount float64, reason string, refundErr error,
) {
	record := dbModels.R4AppaMobilePaymentReversal{
		OrderName:      cartID,
		OrderAmount:    amount,
		ReversalAmount: reversalAmount,
		Reason:         reason,
		Success:        refundErr == nil,
	}
	if refundErr != nil {
		record.ErrorDetail = refundErr.Error()
	}
	if err := s.db.Create(&record).Error; err != nil {
		s.logger.Error("failed to register cart mobile payment reversal", zap.Error(err), zap.Any("record", record))
	}
}

// ValidateMobilePayment matches an already-received R4 pago móvil payment
// against a cart quote's amount; it does not itself initiate a charge.
func (s *cartPaymentService) ValidateMobilePayment(
	ctx context.Context,
	quote models.CartQuote,
	req models.CartValidateMobilePaymentRequest,
) (*models.CartMobilePaymentResult, error) {
	BCVTasa, err := s.bcvClient.Get(ctx)
	if err != nil {
		return nil, errors.New(mobilePaymentGenericErrorMessage)
	}
	expectedVES := quote.Amount * BCVTasa

	var item dbModels.R4AppaMobilePayment
	query := s.db.WithContext(ctx).Model(&dbModels.R4AppaMobilePayment{}).Select("r4_appa_mobile_payments.*")
	query = s.getCartMobilePaymentFilters(query, req)

	const maxIntents = 3
	for range maxIntents {
		if err := query.Last(&item).Error; err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	if item.ID == 0 {
		s.logger.Error("no mobile payment found for cart", zap.String("cartId", quote.CartID), zap.Any("filters", req))
		return &models.CartMobilePaymentResult{
			Success: false,
			Code:    domains.CartMobilePaymentNotFound,
			Message: "no se encontro ningun pago movil que coincida con los datos proporcionados",
		}, nil
	}

	dni := fmt.Sprintf("%s%s", req.DNIType, req.DNI)
	cartID, _, err := s.parseCartIDAndKey(quote.CartID)
	if err != nil {
		s.logger.Error("failed to parse cart id from quote", zap.Error(err), zap.String("cartQuote", quote.CartID))
		return nil, errors.New(mobilePaymentGenericErrorMessage)
	}

	switch domains.ClassifyCharge(expectedVES, item.Amount, BCVTasa) {
	case domains.Underpaid:
		if err := s.db.WithContext(ctx).Delete(&dbModels.R4AppaMobilePayment{}, item.ID).Error; err != nil {
			s.logger.Error("failed to delete underpaid cart mobile payment", zap.Error(err), zap.Int("paymentId", item.ID))
		}
		refundErr := s.r4Repo.ChangePaid(ctx, r4bank.ChangePaidRequest{
			Bank:    item.IssuingBank,
			Amount:  item.Amount,
			Phone:   item.SenderPhone,
			DNI:     dni,
			Concept: fmt.Sprintf("DMT (%s)", cartID),
		})
		go s.registerMobilePaymentReversal(cartID, expectedVES, item.Amount, "LESS", refundErr)
		if refundErr != nil {
			s.logger.Error("failed to return money to sender", zap.Error(refundErr), zap.Any("payment", item))
			return &models.CartMobilePaymentResult{
				Success: false,
				Code:    domains.CartMobilePaymentUnderpaid,
				Message: "Debe realizar el pago por el monto exacto de la orden, si no ve reflejado el reembolso contacte soporte",
			}, nil
		}
		return &models.CartMobilePaymentResult{
			Success:   false,
			Code:      domains.CartMobilePaymentUnderpaid,
			Reference: item.Reference,
			Message:   "Debe realizar el pago por el monto exacto de la orden, se ha realizado la devolución del mismo, a los datos utilizados en su pago",
		}, nil

	case domains.Overpaid:
		item.CartID = quote.CartID
		item.UpdatedAt = time.Now()
		if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
			return nil, errors.New(mobilePaymentGenericErrorMessage)
		}
		excess := item.Amount - expectedVES
		refundErr := s.r4Repo.ChangePaid(ctx, r4bank.ChangePaidRequest{
			Bank:    item.IssuingBank,
			Amount:  excess,
			Phone:   item.SenderPhone,
			DNI:     dni,
			Concept: fmt.Sprintf("DMT (%s)", cartID),
		})
		go s.registerMobilePaymentReversal(cartID, expectedVES, excess, "GREATER", refundErr)
		message := fmt.Sprintf(
			"El monto del pago fue mayor al total del pedido, se ha realizado la devolución del excedente (Bs.S %.2f), a los datos utilizados en su pago",
			excess,
		)
		if refundErr != nil {
			s.logger.Error("failed to return excess to sender", zap.Error(refundErr), zap.Any("payment", item))
			message = fmt.Sprintf("Su pago fue registrado, si no ve reflejado el reembolso del excedente (Bs.S %.2f) contacte soporte", excess)
		}
		return &models.CartMobilePaymentResult{
			Success:   true,
			Code:      domains.CartMobilePaymentOverpaid,
			Reference: item.Reference,
			Message:   message,
		}, nil

	default:
		item.CartID = quote.CartID
		item.UpdatedAt = time.Now()
		if err := s.db.WithContext(ctx).Save(&item).Error; err != nil {
			return nil, errors.New(mobilePaymentGenericErrorMessage)
		}
		return &models.CartMobilePaymentResult{
			Success:   true,
			Reference: item.Reference,
			Message:   "Pago registrado correctamente",
		}, nil
	}
}

func (s *cartPaymentService) registerDirectDebitAccountResult(ctx context.Context, req dbModels.R4DebitDirectAccount) {
	if err := s.db.WithContext(ctx).Create(&req).Error; err != nil {
		s.logger.Error("failed to register cart direct debit account result", zap.Error(err))
	}
}

// DirectDebitAccount charges a test amount to affiliate a new account for a
// cart. On success, create-order-from-cart writes it onto the customer's
// direct_debit_account metafield.
func (s *cartPaymentService) DirectDebitAccount(
	ctx context.Context,
	quote models.CartQuote,
	req models.CartDirectDebitAccountRequest,
) (*models.CartDirectDebitAccountResult, error) {
	amount, err := s.amountVES(ctx, quote)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, errors.New(_debitImmediateGenericError)
	}

	r4Resp, err := s.r4Repo.DirectDebitAccount(ctx, r4bank.DirectDebitAccountRequest{
		Account: req.Account,
		DNI:     req.DNI,
		Name:    req.Name,
		Amount:  amount,
		Concept: "Prueba",
	})
	if err != nil {
		s.logger.Error("direct debit account call failed", zap.Error(err))
		return nil, errors.New(_debitImmediateGenericError)
	}

	s.registerDirectDebitAccountResult(context.Background(), dbModels.R4DebitDirectAccount{
		Account:     req.Account,
		DNI:         req.DNI,
		Amount:      amount,
		Reference:   r4Resp.Reference,
		Code:        r4Resp.Code,
		Success:     r4Resp.Code == domains.R4CodeApproved,
		CartID:      quote.CartID,
		IsRecurring: true,
		Date:        time.Now().In(s.location),
		CreatedAt:   time.Now(),
	})

	return s.directDebitAccountResultFromR4(r4Resp)
}

// directDebitAccountResultFromR4 maps an R4 direct-debit-account response to
// the cart-facing result, shared by the affiliation charge and the recurring
// OTP charge below.
func (s *cartPaymentService) directDebitAccountResultFromR4(
	r4Resp *r4bank.DirectDebitAccountResponse,
) (*models.CartDirectDebitAccountResult, error) {
	if r4Resp.Code == domains.R4CodeApproved {
		return &models.CartDirectDebitAccountResult{
			Success:   true,
			Code:      domains.ResponseCodeOK,
			Reference: r4Resp.Reference,
		}, nil
	}

	if internalCode, ok := domains.DirectDebitAccountResponseCode(r4Resp.Code); ok {
		return &models.CartDirectDebitAccountResult{
			Success:   false,
			Code:      internalCode,
			Reference: r4Resp.Reference,
		}, nil
	}

	s.logger.Error("unexpected direct debit account code", zap.String("code", r4Resp.Code), zap.String("message", r4Resp.Message))
	return nil, errors.New(_debitImmediateGenericError)
}

// RequestDirectDebitAccountOTP mails a 6-digit code to the email Shopify has
// on file for ClientID — never one the request supplies, so a stolen
// clientId can't redirect a charge's OTP to an attacker's inbox. Cached
// under the cart id, mirroring how the order/draft flow caches under orderID.
func (s *cartPaymentService) RequestDirectDebitAccountOTP(
	ctx context.Context,
	quote models.CartQuote,
	req models.CartDirectDebitAccountOTPRequest,
) error {
	customer, err := s.shopifyRepo.GetCustomerByID(ctx, req.ClientID)
	if err != nil {
		s.logger.Error("failed to fetch customer for direct debit account OTP", zap.Error(err), zap.String("clientId", req.ClientID))
		return errors.New(_debitImmediateGenericError)
	}
	if customer == nil || customer.DirectDebitAccount == nil {
		s.logger.Error("customer has no direct debit account on file", zap.String("clientId", req.ClientID))
		return errors.New(_debitImmediateGenericError)
	}

	code, err := generateOTPCode()
	if err != nil {
		s.logger.Error("failed to generate OTP code", zap.Error(err))
		return errors.New(_debitImmediateGenericError)
	}

	s.otpCache.Set(quote.CartID, code)

	return s.mailgunRepo.SendOTPEmail(ctx, mailgun.OTPEmailRequest{
		To:                customer.Email,
		OTPCode:           code,
		ExpirationMinutes: int(otpTTL.Minutes()),
		UserName:          customer.DisplayName,
	})
}

// ValidateDirectDebitAccountOTP confirms the OTP and charges the account
// already on file for ClientID — the recurring-charge counterpart to
// DirectDebitAccount above, which only runs on first affiliation.
func (s *cartPaymentService) ValidateDirectDebitAccountOTP(
	ctx context.Context,
	quote models.CartQuote,
	req models.CartValidateDirectDebitAccountOTPRequest,
) (*models.CartDirectDebitAccountResult, error) {
	customer, err := s.shopifyRepo.GetCustomerByID(ctx, req.ClientID)
	if err != nil {
		s.logger.Error("failed to fetch customer for direct debit account OTP", zap.Error(err), zap.String("clientId", req.ClientID))
		return nil, errors.New(_debitImmediateGenericError)
	}
	if customer == nil || customer.DirectDebitAccount == nil {
		return &models.CartDirectDebitAccountResult{Success: false, Code: domains.ResponseCodeAffiliationExists}, nil
	}

	if !s.otpCache.Validate(quote.CartID, req.OTP) {
		return &models.CartDirectDebitAccountResult{Success: false, Code: domains.ResponseCodeInvalidOTP}, nil
	}

	var directDebit shopify.DebitDirectAccountJson
	if err := json.Unmarshal(customer.DirectDebitAccount.JsonValue, &directDebit); err != nil {
		s.logger.Error(err.Error(), zap.Any("json", customer.DirectDebitAccount.JsonValue))
		return nil, errors.New(_debitImmediateGenericError)
	}

	amount, err := s.amountVES(ctx, quote)
	if err != nil {
		s.logger.Error(err.Error())
		return nil, errors.New(_debitImmediateGenericError)
	}

	r4Resp, err := s.r4Repo.DirectDebitAccount(ctx, r4bank.DirectDebitAccountRequest{
		Account: directDebit.Account,
		DNI:     directDebit.DNI,
		Name:    customer.DisplayName,
		Amount:  amount,
		Concept: "Prueba",
	})
	if err != nil {
		s.logger.Error("direct debit account call failed", zap.Error(err))
		return nil, errors.New(_debitImmediateGenericError)
	}

	s.registerDirectDebitAccountResult(context.Background(), dbModels.R4DebitDirectAccount{
		Account:     directDebit.Account,
		DNI:         directDebit.DNI,
		Amount:      amount,
		Reference:   r4Resp.Reference,
		Code:        r4Resp.Code,
		Success:     r4Resp.Code == domains.R4CodeApproved,
		CartID:      quote.CartID,
		IsRecurring: true,
		Date:        time.Now().In(s.location),
		CreatedAt:   time.Now(),
	})

	return s.directDebitAccountResultFromR4(r4Resp)
}
