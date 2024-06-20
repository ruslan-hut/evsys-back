package entity

import (
	"evsys-back/internal/lib/validate"
	"net/http"
)

type CentralSystemCommand struct {
	ChargePointId string `json:"charge_point_id" bson:"charge_point_id" validate:"omitempty"`
	ConnectorId   int    `json:"connector_id" bson:"connector_id" validate:"omitempty"`
	FeatureName   string `json:"feature_name" bson:"feature_name" validate:"required"`
	Payload       string `json:"payload" bson:"payload" validate:"omitempty"`
}

func (c *CentralSystemCommand) Bind(_ *http.Request) error {
	return validate.Struct(c)
}
