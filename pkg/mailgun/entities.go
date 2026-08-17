package mailgun

type SendEmailRequest struct {
	To       string
	Subject  string
	Body     string
	Template string
	Vars     map[string]any
}

type OTPEmailRequest struct {
	To                string
	OTPCode           string
	ExpirationMinutes int
	UserName          string // optional, used for greeting
}

type SupportAlertRequest struct {
	OrderName string
	Message   string
}
