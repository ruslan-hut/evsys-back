package entity

import (
	"evsys-back/internal/lib/validate"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	statusUnavailable = "Unavailable"
	statusFaulted     = "Faulted"
)

type ChargePoint struct {
	Id              string       `json:"charge_point_id" bson:"charge_point_id" validate:"required"`
	IsEnabled       bool         `json:"is_enabled" bson:"is_enabled"`
	Title           string       `json:"title" bson:"title" validate:"omitempty"`
	Description     string       `json:"description,omitempty" bson:"description" validate:"omitempty"`
	Model           string       `json:"model" bson:"model" validate:"omitempty"`
	SerialNumber    string       `json:"serial_number,omitempty" bson:"serial_number" validate:"omitempty"`
	Vendor          string       `json:"vendor,omitempty" bson:"vendor" validate:"omitempty"`
	FirmwareVersion string       `json:"firmware_version,omitempty" bson:"firmware_version" validate:"omitempty"`
	Status          string       `json:"status" bson:"status" validate:"omitempty,connector_status"`
	ErrorCode       string       `json:"error_code" bson:"error_code" validate:"omitempty"`
	Info            string       `json:"info,omitempty" bson:"info" validate:"omitempty"`
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

func (cp *ChargePoint) Bind(_ *http.Request) error {
	return validate.Struct(cp)
}

// IsAvailable returns true if the charge point is online and operational.
func (cp *ChargePoint) IsAvailable() bool {
	if !cp.IsOnline {
		return false
	}
	return cp.Status != statusUnavailable && cp.Status != statusFaulted
}

func (cp *ChargePoint) CheckConnectorsStatus() {
	if cp.Connectors == nil {
		return
	}
	if !cp.IsAvailable() {
		cp.Status = statusUnavailable
		for _, conn := range cp.Connectors {
			conn.Status = statusUnavailable
			conn.State = strings.ToLower(statusUnavailable)
		}
	}
}

func (cp *ChargePoint) HideSensitiveData() {
	cp.SerialNumber = ""
	cp.Vendor = ""
	cp.FirmwareVersion = ""
}
