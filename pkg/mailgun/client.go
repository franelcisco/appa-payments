package mailgun

import (
	"github.com/mailgun/mailgun-go/v5"
)

type MailGunClient struct {
	mg *mailgun.Client
}

func NewClient(apiKey string) *MailGunClient {
	mg := mailgun.NewMailgun(apiKey)

	return &MailGunClient{
		mg: mg,
	}
}
