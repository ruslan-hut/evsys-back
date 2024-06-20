package entity

import (
	"encoding/json"
	"evsys-back/internal/lib/validate"
	"fmt"
	"net/http"
	"time"
)

type ChargePoint struct {
	Id              string       `json:"charge_point_id" bson:"charge_point_id" validate:"required"`
	IsEnabled       bool         `json:"is_enabled" bson:"is_enabled"`
	Title           string       `json:"title" bson:"title" validate:"omitempty"`
	Description     string       `json:"description" bson:"description" validate:"omitempty"`
	Model           string       `json:"model" bson:"model" validate:"omitempty"`
	SerialNumber    string       `json:"serial_number" bson:"serial_number" validate:"omitempty"`
	Vendor          string       `json:"vendor" bson:"vendor" validate:"omitempty"`
	FirmwareVersion string       `json:"firmware_version" bson:"firmware_version" validate:"omitempty"`
	Status          string       `json:"status" bson:"status" validate:"omitempty"`
	ErrorCode       string       `json:"error_code" bson:"error_code" validate:"omitempty"`
	Info            string       `json:"info" bson:"info" validate:"omitempty"`
	LastSeen        string       `json:"last_seen" bson:"last_seen" validate:"omitempty"` // deprecated
	EventTime       time.Time    `json:"event_time" bson:"event_time" validate:"omitempty"`
	IsOnline        bool         `json:"is_online" bson:"is_online"`
	StatusTime      time.Time    `json:"status_time" bson:"status_time" validate:"omitempty"`
	Address         string       `json:"address" bson:"address" validate:"omitempty"`
	AccessType      string       `json:"access_type" bson:"access_type" validate:"omitempty"`
	AccessLevel     int          `json:"access_level" bson:"access_level" validate:"omitempty"`
	Location        GeoLocation  `json:"location" bson:"location" validate:"omitempty"`
	Connectors      []*Connector `json:"connectors" bson:"connectors" validate:"omitempty,dive"`
}

func (cp *ChargePoint) GetConnector(id int) (*Connector, error) {
	for _, c := range cp.Connectors {
		if c.Id == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("connector %d not found", id)
}

func GetChargePointFromPayload(payload []byte) (*ChargePoint, error) {
	var cp ChargePoint
	err := json.Unmarshal(payload, &cp)
	if err != nil {
		return nil, err
	}
	return &cp, nil
}

func (cp *ChargePoint) Bind(_ *http.Request) error {
	return validate.Struct(cp)
}
