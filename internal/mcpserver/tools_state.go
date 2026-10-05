package mcpserver

import (
	"context"
	"evsys-back/entity"
	"sort"
	"strings"
)

type noInput struct{}

type problemView struct {
	ChargePointId string `json:"charge_point_id"`
	ConnectorId   *int   `json:"connector_id,omitempty"`
	Problem       string `json:"problem"`
	Status        string `json:"status,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	Info          string `json:"info,omitempty"`
	Since         string `json:"since,omitempty"`
}

// problemsOf lists what is wrong with an enabled charge point: being offline,
// or connectors reporting a fault, an error code or unavailability.
func problemsOf(cp *entity.ChargePoint) []problemView {
	if !cp.IsEnabled {
		return nil
	}
	var problems []problemView
	if !cp.IsOnline {
		problems = append(problems, problemView{
			ChargePointId: cp.Id,
			Problem:       "offline",
			Since:         timestamp(cp.EventTime),
		})
	}
	if hasError(cp.ErrorCode) || cp.Status == "Faulted" {
		problems = append(problems, problemView{
			ChargePointId: cp.Id,
			Problem:       "charge point error",
			Status:        cp.Status,
			ErrorCode:     cp.ErrorCode,
			Info:          cp.Info,
			Since:         timestamp(cp.StatusTime),
		})
	}
	for _, c := range cp.Connectors {
		if c == nil {
			continue
		}
		problem := ""
		switch {
		case c.Status == "Faulted" || hasError(c.ErrorCode):
			problem = "connector error"
		case c.Status == "Unavailable":
			problem = "connector unavailable"
		default:
			continue
		}
		id := c.Id
		problems = append(problems, problemView{
			ChargePointId: cp.Id,
			ConnectorId:   &id,
			Problem:       problem,
			Status:        c.Status,
			ErrorCode:     c.ErrorCode,
			Info:          c.Info,
			Since:         timestamp(c.StatusTime),
		})
	}
	return problems
}

func (t *tools) systemOverview(ctx context.Context, user *entity.User, _ noInput) (any, error) {
	chargePoints, err := t.core.InspectChargePoints(ctx, user, "")
	if err != nil {
		return nil, err
	}
	sessions, err := t.core.InspectActiveTransactions(ctx, user)
	if err != nil {
		return nil, err
	}
	now := t.now()

	type cpSummary struct {
		Total    int `json:"total"`
		Enabled  int `json:"enabled"`
		Online   int `json:"online"`
		Offline  int `json:"offline_enabled"`
		Disabled int `json:"disabled"`
	}
	type connSummary struct {
		Total    int            `json:"total"`
		ByStatus map[string]int `json:"by_status"`
	}
	var cps cpSummary
	conns := connSummary{ByStatus: map[string]int{}}
	problems := make([]problemView, 0)
	for _, cp := range chargePoints {
		cps.Total++
		if !cp.IsEnabled {
			cps.Disabled++
			continue
		}
		cps.Enabled++
		if cp.IsOnline {
			cps.Online++
		} else {
			cps.Offline++
		}
		for _, c := range cp.Connectors {
			if c == nil {
				continue
			}
			conns.Total++
			status := c.Status
			if status == "" {
				status = "unknown"
			}
			conns.ByStatus[status]++
		}
		problems = append(problems, problemsOf(cp)...)
	}

	totalPower := 0
	active := make([]activeSessionView, 0, len(sessions))
	for _, s := range sessions {
		view := activeSessionFrom(s, now)
		view.LastMeter = nil
		totalPower += view.PowerW
		active = append(active, view)
	}

	return map[string]any{
		"now":           timestamp(now),
		"charge_points": cps,
		"connectors":    conns,
		"problems":      problems,
		"active_sessions": map[string]any{
			"count":         len(active),
			"total_power_w": totalPower,
			"sessions":      active,
		},
		"note": "connector counts and problems cover enabled charge points only",
	}, nil
}

type listChargePointsInput struct {
	Search       string `json:"search,omitempty" jsonschema:"case-insensitive text matched against charge point id, title, description and address"`
	ProblemsOnly bool   `json:"problems_only,omitempty" jsonschema:"only enabled charge points that are offline or have a faulted, erroring or unavailable connector"`
}

func (t *tools) listChargePoints(ctx context.Context, user *entity.User, in listChargePointsInput) (any, error) {
	chargePoints, err := t.core.InspectChargePoints(ctx, user, in.Search)
	if err != nil {
		return nil, err
	}
	views := make([]chargePointView, 0, len(chargePoints))
	for _, cp := range chargePoints {
		if in.ProblemsOnly && len(problemsOf(cp)) == 0 {
			continue
		}
		views = append(views, chargePointFrom(cp))
	}
	return map[string]any{"count": len(views), "charge_points": views}, nil
}

type chargePointInput struct {
	ChargePointId string `json:"charge_point_id" jsonschema:"the charge point identifier, as in list_charge_points"`
}

func (t *tools) getChargePoint(ctx context.Context, user *entity.User, in chargePointInput) (any, error) {
	cp, err := t.core.InspectChargePoint(ctx, user, strings.TrimSpace(in.ChargePointId))
	if err != nil {
		return nil, err
	}
	view := chargePointFrom(cp)
	return map[string]any{
		"charge_point": view,
		"problems":     problemsOf(cp),
		"geo_location": cp.Location,
		"description":  cp.Description,
	}, nil
}

func (t *tools) listLocations(ctx context.Context, user *entity.User, _ noInput) (any, error) {
	locations, err := t.core.InspectLocations(ctx, user)
	if err != nil {
		return nil, err
	}
	type cpBrief struct {
		Id        string `json:"charge_point_id"`
		Title     string `json:"title,omitempty"`
		IsEnabled bool   `json:"is_enabled"`
		IsOnline  bool   `json:"is_online"`
		Status    string `json:"status,omitempty"`
	}
	type locationView struct {
		Id                string    `json:"id"`
		Name              string    `json:"name,omitempty"`
		Address           string    `json:"address,omitempty"`
		City              string    `json:"city,omitempty"`
		Country           string    `json:"country,omitempty"`
		PowerLimit        int       `json:"power_limit"`
		DefaultPowerLimit int       `json:"default_power_limit"`
		ChargePoints      []cpBrief `json:"charge_points"`
	}
	views := make([]locationView, 0, len(locations))
	for _, l := range locations {
		view := locationView{
			Id:                l.Id,
			Name:              l.Name,
			Address:           l.Address,
			City:              l.City,
			Country:           l.Country,
			PowerLimit:        l.PowerLimit,
			DefaultPowerLimit: l.DefaultPowerLimit,
			ChargePoints:      make([]cpBrief, 0, len(l.ChargePoints)),
		}
		for _, cp := range l.ChargePoints {
			if cp == nil {
				continue
			}
			view.ChargePoints = append(view.ChargePoints, cpBrief{
				Id: cp.Id, Title: cp.Title, IsEnabled: cp.IsEnabled, IsOnline: cp.IsOnline, Status: cp.Status,
			})
		}
		sort.Slice(view.ChargePoints, func(i, j int) bool { return view.ChargePoints[i].Id < view.ChargePoints[j].Id })
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Id < views[j].Id })
	return map[string]any{
		"count":     len(views),
		"locations": views,
		"note":      "power_limit is the site's rated capacity, recorded for reference only; default_power_limit is the amperage of the default charging profile installed on its charge points at boot (0 = none). Only roaming-enabled locations are listed.",
	}, nil
}
