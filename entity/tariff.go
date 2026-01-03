package entity

// Tariff represents pricing tariff (placeholder - expand when CSMS definition available)
type Tariff struct {
	TariffId    string `json:"tariff_id,omitempty" bson:"tariff_id,omitempty"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
}
