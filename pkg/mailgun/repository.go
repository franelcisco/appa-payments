package mailgun

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"time"

	"github.com/mailgun/mailgun-go/v5"
	"go.uber.org/zap"
)

//go:embed templates/otp_email.html
var rawOTPEmailTemplate string

var otpEmailTmpl = template.Must(template.New("otp_email").Parse(rawOTPEmailTemplate))

type Repository interface {
	SendEmail(ctx context.Context, req SendEmailRequest) error
	SendOTPEmail(ctx context.Context, req OTPEmailRequest) error
	SendSupportAlert(ctx context.Context, req SupportAlertRequest) error
}

type repository struct {
	client       *MailGunClient
	domain       string
	sender       string
	supportEmail string
	logger       *zap.Logger
}

func NewRepository(client *MailGunClient, domain string, sender string, supportEmail string, logger *zap.Logger) Repository {
	return &repository{
		client:       client,
		domain:       domain,
		sender:       sender,
		supportEmail: supportEmail,
		logger:       logger,
	}
}

func (r *repository) SendEmail(ctx context.Context, req SendEmailRequest) error {
	message := mailgun.NewMessage(r.domain, r.sender, req.Subject, req.Body, req.To)

	if req.Template != "" {
		message.SetTemplate(req.Template)
	}

	if len(req.Vars) > 0 {
		if err := r.setEmailVariables(message, req.Vars); err != nil {
			r.logger.Error(err.Error(), zap.String("to", req.To), zap.String("template", req.Template))
			return err
		}
	}

	if req.Template == "" {
		message.SetHTML(req.Body)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	_, err := r.client.mg.Send(ctx, message)
	if err != nil {
		r.logger.Error(err.Error(), zap.String("to", req.To), zap.String("template", req.Template))
		return err
	}

	return nil
}

// SendOTPEmail renders the OTP email template and sends it to the recipient.
func (r *repository) SendOTPEmail(ctx context.Context, req OTPEmailRequest) error {
	var buf bytes.Buffer
	if err := otpEmailTmpl.Execute(&buf, req); err != nil {
		r.logger.Error("failed to render OTP email template", zap.Error(err), zap.String("to", req.To))
		return err
	}

	return r.SendEmail(ctx, SendEmailRequest{
		To:      req.To,
		Subject: "Tu código de verificación — Appa",
		Body:    buf.String(),
	})
}

// SendSupportAlert sends a plain-text alert email to the support address.
func (r *repository) SendSupportAlert(ctx context.Context, req SupportAlertRequest) error {
	body := fmt.Sprintf(
		"Alerta: la orden %s no pudo recibir el descuento de domiciliación bancaria.\nSe requiere revisión manual para marcarla como pagada.\n\nError: %s",
		req.OrderName,
		req.Message,
	)
	return r.SendEmail(ctx, SendEmailRequest{
		To:      r.supportEmail,
		Subject: fmt.Sprintf("Alerta: error al aplicar descuento en orden %s", req.OrderName),
		Body:    body,
	})
}

func (r *repository) setEmailVariables(message *mailgun.PlainMessage, vars map[string]any) error {
	for key, value := range vars {
		message.AddVariable(key, value)
	}
	return nil
}
