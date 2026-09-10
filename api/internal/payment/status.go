package payment

type Status string

const (
	StatusRequiresPayment Status = "requires_payment"
	StatusProcessing      Status = "processing"
	StatusAuthorized      Status = "authorized"
	StatusCaptured        Status = "captured"
	StatusRefunded        Status = "refunded"
	StatusFailed          Status = "failed"
	StatusCancelled       Status = "cancelled"
)

var transitions = map[Status]map[Status]struct{}{
	StatusRequiresPayment: {StatusProcessing: {}},
	StatusProcessing:      {StatusAuthorized: {}, StatusFailed: {}},
	StatusAuthorized:      {StatusCaptured: {}, StatusCancelled: {}},
	StatusCaptured:        {StatusRefunded: {}},
}

func CanTransition(from, to Status) bool {
	next, ok := transitions[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}