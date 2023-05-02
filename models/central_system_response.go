package models

type ResponseStatus string

const (
	Success ResponseStatus = "success"
	Error   ResponseStatus = "error"
)

type CentralSystemResponse struct {
	Status ResponseStatus `json:"status" bson:"status"`
	Info   string         `json:"info" bson:"info"`
}

func NewCentralSystemResponse(status ResponseStatus, info string) *CentralSystemResponse {
	return &CentralSystemResponse{Status: status, Info: info}
}
