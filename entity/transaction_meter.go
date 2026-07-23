package entity

import "time"

type TransactionMeter struct {
	Id        int `json:"transaction_id" bson:"transaction_id" validate:"required,min=1"`
	Value     int `json:"value" bson:"value" validate:"min=0"`
	PowerRate int `json:"power_rate" bson:"power_rate" validate:"min=0"`
	// PowerRateWh is PowerRate expressed in kW. PowerActive is the value the
	// charger reported itself, where PowerRate may instead be derived from the
	// difference between consecutive meter readings.
	PowerRateWh float64 `json:"power_rate_wh" bson:"power_rate_wh" validate:"min=0"`
	PowerActive int     `json:"power_active" bson:"power_active" validate:"min=0"`
	// Voltage, CurrentImport and CurrentOffered are what separate a session
	// limited by the load balancer from one limited by the hardware or by the
	// car; a power figure alone cannot say which was binding. CurrentOffered is
	// the charge point restating the limit it advertises to the vehicle, so it
	// can be compared against the amperage the balancer asked for. All three are
	// zero on readings from charge points that do not report them, and on every
	// reading written before evsys started recording them.
	Voltage         float64   `json:"voltage" bson:"voltage" validate:"min=0"`
	CurrentImport   float64   `json:"current_import" bson:"current_import" validate:"min=0"`
	CurrentOffered  float64   `json:"current_offered" bson:"current_offered" validate:"min=0"`
	BatteryLevel    int       `json:"battery_level" bson:"battery_level" validate:"min=0,max=100"`
	ConsumedEnergy  int       `json:"consumed_energy" bson:"consumed_energy" validate:"min=0"`
	Price           int       `json:"price" bson:"price" validate:"min=0"`
	Time            time.Time `json:"time" bson:"time"`
	Timestamp       int64     `json:"timestamp" bson:"timestamp"` // value not present in database, but used for clients
	Minute          int64     `json:"minute" bson:"minute" validate:"min=0"`
	Unit            string    `json:"unit" bson:"unit" validate:"omitempty"`
	Measurand       string    `json:"measurand" bson:"measurand" validate:"omitempty"`
	ConnectorId     int       `json:"connector_id" bson:"connector_id" validate:"min=0"`
	ConnectorStatus string    `json:"connector_status" bson:"connector_status" validate:"omitempty,connector_status"`
}
