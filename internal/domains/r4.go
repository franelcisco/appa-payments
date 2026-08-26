package domains

type R4Code string

const (
	R4CodeApproved              R4Code = "ACCP"
	R4CodeInProgress            R4Code = "AC00"
	R4CodeInPending             R4Code = "11"
	R4CodeInsufficientFunds     R4Code = "AM04"
	R4CodeAffiliationRequested  R4Code = "MD01"
	R4CodeAffiliationNotAcepted R4Code = "MD09"
	R4CodeInvalidAccountNumber  R4Code = "AC01"
	R4CodeUpstreamRejected      R4Code = "FAI01"
	R4InvalidAmount             R4Code = "MD15"
	R4InvalidOTP                R4Code = "TKCM"
	R4InvalidTime               R4Code = "VE01"
	R4InvalidClientData         R4Code = "BE01"
)

const R4CodeUnknownDescription = "Desconocido"

var _DebitInmediateSpecialResponse = map[R4Code]string{
	R4CodeInvalidAccountNumber:  "Número de cuenta incorrecto",
	R4CodeInsufficientFunds:     "Saldo insuficiente",
	R4CodeInProgress:            "En espera de respuesta del banco",
	R4CodeInPending:             "En espera de respuesta del banco",
	R4CodeApproved:              "Transacción Exitosa",
	R4CodeAffiliationRequested:  "Afiliación solicitada, debe aprobarla en su banco",
	R4CodeAffiliationNotAcepted: "No posee afiliación",
	R4CodeUpstreamRejected:      "Conexion con el banco fallida",
	R4InvalidAmount:             "Monto incorrecto",
	R4InvalidOTP:                "Codigo OTP inválido",
	R4InvalidTime:               "Fuera del horario permitido",
	R4InvalidClientData:         "Datos del cliente no corresponden a la cuenta",
}

// GetR4CodeDescription returns a human-readable description for a given R4 code.
func (c R4Code) GetR4CodeDescription() string {
	if desc, ok := _DebitInmediateSpecialResponse[c]; ok {
		return desc
	}
	return R4CodeUnknownDescription
}

// IsR4BreakCode returns true if the code is one that indicates the payment is still in progress.
func IsR4BreakCode(code R4Code) bool {
	return code == R4CodeInProgress || code == R4CodeInPending
}

// R4APIError represents an error response from the R4 API.
type R4APIError struct {
	Code        R4Code `json:"code"`
	OperationID string `json:"operation_id"`
	Error       string `json:"error"`
}
