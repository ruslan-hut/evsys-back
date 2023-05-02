package models

type CentralSystemCommand struct {
	ChargePointId string `json:"charge_point_id" bson:"charge_point_id"`
	ConnectorId   int    `json:"connector_id" bson:"connector_id"`
	FeatureName   string `json:"feature_name" bson:"feature_name"`
	Payload       string `json:"payload" bson:"payload"`
}
