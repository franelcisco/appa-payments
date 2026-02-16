package r4bank

// BCVTasaUSDResponse represent the response from the BCV API
type BCVTasaUSDResponse struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

type OTPRequest struct {
	Bank   string  `json:"bank"`
	Amount float64 `json:"amount"`
	Phone  string  `json:"phone"`
	DNI    string  `json:"dni"`
}

type ValidateOTPRequest struct {
	Bank    string  `json:"bank"`
	Amount  float64 `json:"amount"`
	Phone   string  `json:"phone"`
	DNI     string  `json:"dni"`
	Name    string  `json:"name"`
	OTP     string  `json:"otp"`
	Concept string  `json:"concept"`
}

type ChangePaidRequest struct {
	Bank    string  `json:"bank"`
	Amount  float64 `json:"amount"`
	Phone   string  `json:"phone"`
	DNI     string  `json:"dni"`
	Concept string  `json:"concept"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type GetOperationResponse struct {
	Code      string `json:"code"`
	Reference string `json:"reference"`
	Success   bool   `json:"success"`
}

type ValidateDebitInmediateResponse struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Reference string `json:"reference"`
	Message   string `json:"message"`
	Status    bool   `json:"status"`
}
