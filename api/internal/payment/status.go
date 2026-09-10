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
