package models

type ResponseStatus string

const (
	Success ResponseStatus = "success"
	Error   ResponseStatus = "error"
	Waiting ResponseStatus = "waiting"
	Ping    ResponseStatus = "ping"
	Value   ResponseStatus = "value"
)
