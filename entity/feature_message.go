package entity

import "time"

const FeatureMessageType = "featureMessage"

type FeatureMessage struct {
	Time          string    `json:"time" bson:"time"`
	Feature       string    `json:"feature" bson:"feature"`
	ChargePointId string    `json:"id" bson:"charge_point_id"`
	Text          string    `json:"text" bson:"text"`
	Timestamp     time.Time `json:"timestamp" bson:"timestamp"`
	Importance    string    `json:"importance" bson:"importance"`
}
