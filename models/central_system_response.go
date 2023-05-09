package models

type CentralSystemResponse struct {
	Status ResponseStatus `json:"status" bson:"status"`
	Info   string         `json:"info" bson:"info"`
}

func NewCentralSystemResponse(status ResponseStatus, info string) *CentralSystemResponse {
	return &CentralSystemResponse{Status: status, Info: info}
}
