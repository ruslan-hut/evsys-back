package models

type ResponseStatus string
type ResponseStage string

const (
	Success          ResponseStatus = "success"
	Error            ResponseStatus = "error"
	Waiting          ResponseStatus = "waiting"
	Ping             ResponseStatus = "ping"
	Value            ResponseStatus = "value"
	Event            ResponseStatus = "event"
	Start            ResponseStage  = "start"
	Stop             ResponseStage  = "stop"
	Info             ResponseStage  = "info"
	LogEvent         ResponseStage  = "log-event"
	ChargePointEvent ResponseStage  = "charge-point-event"
)
