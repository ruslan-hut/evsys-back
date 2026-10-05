package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func (t *tools) register(s *mcp.Server) {
	// current state
	addTool(t, s, "system_overview", "System overview",
		"Current state of the whole network in one call: charge points online/offline, connectors by status, a list of problems (offline charge points, faulted, erroring or unavailable connectors) and the sessions charging right now with their power. Start here.",
		t.systemOverview)
	addTool(t, s, "list_charge_points", "List charge points",
		"Charge points with their connectors: online state, last reported status and error code, model, vendor, firmware, smart charging, the current limit installed by the load balancer and the charger's answer to it. Connector status is the last one reported, so it may be stale while the charge point is offline.",
		t.listChargePoints)
	addTool(t, s, "get_charge_point", "Get charge point",
		"One charge point in full, with connectors and its current problems.",
		t.getChargePoint)
	addTool(t, s, "list_locations", "List locations",
		"Sites with their power settings and the charge points installed there. Locations are the unit of load balancing and of the site_concurrency report.",
		t.listLocations)

	// transactions
	addTool(t, s, "list_active_transactions", "Active sessions",
		"Charging sessions running now, of all users: charge point, user, duration, energy so far, current power, the amperage the load balancer assigned, and the latest meter reading (voltage, current drawn, current offered by the charger, battery level).",
		t.listActiveTransactions)
	addTool(t, s, "search_transactions", "Search transactions",
		"Finished charging sessions in a period, newest first, filtered by charge point, user, RFID tag or payment failure. Each row has duration, energy, average power, assigned amperage, stop reason and payment. Includes totals of the returned rows. For aggregates over long periods use energy_report or power_report instead.",
		t.searchTransactions)
	addTool(t, s, "get_transaction", "Get transaction",
		"One session in full: user, tariff, payment and the meter value series (energy, power reported and derived, voltage, current drawn, current offered, battery level, connector status). Use it to see why a session charged slowly: compare current_import_a with current_offered_a and power_limit_a.",
		t.getTransaction)

	// logs
	addTool(t, s, "read_log", "Read log",
		"Records of one log, newest first, filtered by time, charge point, text, category or level. sys holds the OCPP traffic from charge points (BootNotification, StatusNotification, MeterValues, connect/disconnect events); errors holds connector errors; back and pay hold backend and payment events.",
		t.readLog)
	addTool(t, s, "error_summary", "Error summary",
		"Connector errors over a period counted per charge point, connector and error code, with first and last occurrence. The quickest way to find the troublesome hardware.",
		t.errorSummary)

	// workload reports, as in the web UI
	addTool(t, s, "energy_report", "Energy report",
		"Energy delivered and number of sessions per month, user, charge point or hour, for the users of one group - the figures of the web statistics page.",
		t.energyReport)
	addTool(t, s, "power_report", "Power report",
		"Power statistics per charge point, per session, or as a timeline per hour or day: energy, average and peak power. The hour and day timelines give the concurrent load of the fleet.",
		t.powerReport)
	addTool(t, s, "station_uptime", "Station uptime",
		"Share of time each enabled charge point was connected to the central system over a period, from its connect and disconnect events.",
		t.stationUptime)
	addTool(t, s, "station_status", "Station status",
		"Whether each enabled charge point is connected now, and since when.",
		t.stationStatus)
	addTool(t, s, "site_concurrency", "Site concurrency",
		"Per location: when sessions overlapped, the most charging at once, the peak amperage the load balancer assigned and the peak power the site actually drew, with the timeline of overlapping segments.",
		t.siteConcurrency)

	// administration
	addTool(t, s, "list_users", "List users",
		"User accounts with role, access level, group and last activity. Admins only.",
		t.listUsers)
	addTool(t, s, "list_user_tags", "List RFID tags",
		"RFID tags (id_tag) with the user they belong to, note and last use.",
		t.listUserTags)
	addTool(t, s, "payment_retry_queue", "Payment retries",
		"Failed payments scheduled for automatic retry, with the last error.",
		t.paymentRetryQueue)
	addTool(t, s, "webhook_status", "Webhook status",
		"Delivery counters of each webhook subscriber and the recent failed deliveries.",
		t.webhookStatus)
}
