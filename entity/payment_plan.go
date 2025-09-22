package entity

type PaymentPlan struct {
	PlanId       string `json:"plan_id" bson:"plan_id"`
	Description  string `json:"description" bson:"description"`
	IsDefault    bool   `json:"is_default" bson:"is_default"` // global default, for all users
	IsActive     bool   `json:"is_active" bson:"is_active"`
	PricePerKwh  int    `json:"price_per_kwh" bson:"price_per_kwh"`
	PricePerHour int    `json:"price_per_hour" bson:"price_per_hour"`
	StartTime    string `json:"start_time" bson:"start_time"`
	EndTime      string `json:"end_time" bson:"end_time"`
}
