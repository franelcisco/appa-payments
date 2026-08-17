package domains

const ToleranceUSD = 0.1

func Tolerance(rate float64) float64 { return ToleranceUSD * rate }

type Verdict int

const (
	Exact Verdict = iota
	Underpaid
	Overpaid
)

// ClassifyCharge returns a verdict on whether a buyer's money is accepted,
// refunded, or partially refunded. Both amounts are in VES; rate scales the
// tolerance.
func ClassifyCharge(expectedVES, receivedVES, rate float64) Verdict {
	tol := Tolerance(rate)
	switch {
	case expectedVES > receivedVES+tol:
		return Underpaid
	case expectedVES < receivedVES-tol:
		return Overpaid
	default:
		return Exact
	}
}

const (
	CartMobilePaymentNotFound  = "not_found"
	CartMobilePaymentUnderpaid = "under"
	CartMobilePaymentOverpaid  = "over"
)
