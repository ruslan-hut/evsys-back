package database

type ReportLineMonth struct {
	Year    int   `json:"year" bson:"year"`
	Month   int   `json:"month" bson:"month"`
	Total   int64 `json:"total" bson:"totalConsumed"`
	Count   int64 `json:"count" bson:"count"`
	Average int64 `json:"average" bson:"avgWatts"`
}
