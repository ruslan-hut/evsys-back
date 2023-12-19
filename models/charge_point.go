package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type ChargePoint struct {
	Id              string      `json:"charge_point_id" bson:"charge_point_id"`
	IsEnabled       bool        `json:"is_enabled" bson:"is_enabled"`
	Title           string      `json:"title" bson:"title"`
	Description     string      `json:"description" bson:"description"`
	Model           string      `json:"model" bson:"model"`
	SerialNumber    string      `json:"serial_number" bson:"serial_number"`
	Vendor          string      `json:"vendor" bson:"vendor"`
	FirmwareVersion string      `json:"firmware_version" bson:"firmware_version"`
	Status          string      `json:"status" bson:"status"`
	ErrorCode       string      `json:"error_code" bson:"error_code"`
	Info            string      `json:"info" bson:"info"`
	LastSeen        string      `json:"last_seen" bson:"last_seen"` // deprecated
	EventTime       time.Time   `json:"event_time" bson:"event_time"`
	IsOnline        bool        `json:"is_online" bson:"is_online"`
	StatusTime      time.Time   `json:"status_time" bson:"status_time"`
	Address         string      `json:"address" bson:"address"`
	Location        Location    `json:"location" bson:"location"`
	Connectors      []Connector `json:"connectors" bson:"connectors"`
}

func (cp *ChargePoint) GetConnector(id int) (*Connector, error) {
	for _, c := range cp.Connectors {
		if c.Id == id {
			return &c, nil
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
