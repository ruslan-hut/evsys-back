package database

type ReportLine struct {
	Year    int     `json:"year,omitempty" bson:"year"`
	Month   int     `json:"month,omitempty" bson:"month"`
	User    string  `json:"user,omitempty" bson:"user"`
	Total   int64   `json:"total" bson:"totalConsumed"`
	Count   int64   `json:"count" bson:"count"`
	Average float64 `json:"average" bson:"avgWatts"`
}
