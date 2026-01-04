package entity

import "time"

const FeatureMessageType = "featureMessage"

type FeatureMessage struct {
	Time          string    `json:"time" bson:"time" validate:"omitempty"`
	Feature       string    `json:"feature" bson:"feature" validate:"required"`
	ChargePointId string    `json:"id" bson:"charge_point_id" validate:"required"`
	Text          string    `json:"text" bson:"text" validate:"omitempty"`
	Timestamp     time.Time `json:"timestamp" bson:"timestamp"`
	Importance    string    `json:"importance" bson:"importance" validate:"omitempty"`
}
