Backend Flow for Station Uptime Analysis in Go

Overview

This document outlines a backend‑friendly workflow for calculating charging station uptime and current status using Go and MongoDB.  The system stores events indicating when a station connects or disconnects from a WebSocket.  By interpreting these events, the backend can derive how long each station has been online or offline during a given period and determine its current state.  The flow emphasises clear separation of inputs, processing steps, and outputs so that it can be implemented in a maintainable service layer.

Inputs and Data Model
•	Event schema: Each event document in the events collection has a charge_point_id, a timestamp (Mongo Date), and a text field that describes the action.  The backend interprets the text to derive a state value: registered implies ONLINE and unregistered implies OFFLINE.
•	Parameters: For the period report, the service accepts a start time (from) and end time (to) and an optional list of station identifiers.  Times should be normalised to UTC and validated so that from < to.
•	Utility function: Implement stateFromText(text string) State to map event texts to an enumeration (ONLINE, OFFLINE, UNKNOWN).  Log or ignore unknown states instead of failing.

Report A – Uptime and Downtime per Station

This report calculates how long each station was online or offline within a specified time range.
1.	Validate range and normalise times. Confirm that from is earlier than to and convert both to a consistent timezone (typically UTC).
2.	Find all relevant stations. Either use a preexisting registry or derive the list of charge_point_id values by querying distinct identifiers from events between from and to (optionally including a small buffer before from to capture ongoing sessions).
3.	Determine starting state per station. For each station, find the last event at or before the start of the period:
•	Query: findOne({charge_point_id: id, timestamp: { $lte: from }}).sort({timestamp: -1}).
•	If a record exists, use stateFromText(text) to set the initial state; otherwise default to OFFLINE.
•	Record the starting lastTs = from and current state for each station.
4.	Load events within the period. Fetch all events for the desired stations where timestamp > from and timestamp ≤ to.  Sort by charge_point_id and timestamp ascending.  Index on {charge_point_id, timestamp} improves efficiency.
5.	Walk the timeline. For each station, iterate through its events chronologically:
1.	Compute delta = e.timestamp - lastTs.  Add delta to the accumulator for the current state: if state == ONLINE then online += delta else offline += delta.
2.	Transition: set state = stateFromText(e.text) and lastTs = e.timestamp.
6.	Handle the tail interval. After processing all events for a station, add the difference between to and the last timestamp to the bucket corresponding to the current state.  This captures time spent online or offline after the final event.
7.	Return aggregated results. For each station, report total online and offline durations (e.g., in minutes or hours) and, optionally, the percentage of time online.  Including the final state at the end of the period can help debug.

Batch Mode for Performance

When analysing many stations or long time spans, fetching events per station may become expensive.  In this case:
1.	Preload starting states with aggregation. Run a single aggregation pipeline that matches all events with timestamp ≤ from, sorts by charge_point_id ascending and timestamp descending, and groups by charge_point_id taking the first document.  This yields the initial state for each station in one query.
2.	Stream period events once. Query for all events in the period sorted by (charge_point_id, timestamp).  Iterate through the cursor, updating a map keyed by charge_point_id that tracks the current state, last timestamp, and running totals.  When the station identifier changes, close the previous station’s tail interval.
3.	Complete any remaining tails. After the stream ends, finish any open intervals by adding to – lastTs to each station’s current state.

This streaming approach scales better because it reduces the number of database round trips and processes events in a single pass.

Report B – Current Status per Station

This report shows whether each station is currently connected and how long it has been in its present state.
1.	Choose the reference time. Use now := time.Now().UTC() (or another consistent timestamp) as the reference for duration calculation.
2.	Fetch the last event per station. Use an aggregation pipeline sorted by (charge_point_id ASC, timestamp DESC) and grouped by charge_point_id to retrieve the most recent event for each station.  This single query avoids N separate findOne calls.
3.	Interpret state and duration. For each station:
•	state = stateFromText(lastEvent.text); treat unknown or missing events as OFFLINE.
•	since = lastEvent.timestamp.
•	duration = now – since.
•	Optionally include the raw text and feature fields for context.
4.	Return the status list. Provide for each station its current state, the timestamp of the last change, and the duration in that state.  When there are no events, use since = nil and duration = 0 or define an appropriate sentinel.

Edge Cases and Considerations
•	Duplicate state events: If two consecutive events indicate the same state (e.g., registered followed by registered), treat the interval between them normally; the station remains online during that period.
•	Missing unregistered events: In Report A, a session that has a registered event but no corresponding unregistered event should be considered online until the end of the report range.  In Report B, the station is treated as online from the last registered until now.
•	Out‑of‑order events: Always sort events by timestamp ascending to ensure correct interval computation.
•	Time zones: Store and compute times in UTC internally.  Convert to the user’s time zone only when presenting results.
•	Indexes: Ensure an index on (charge_point_id, timestamp) to support queries for starting states and event streams efficiently.

Go Implementation Structure
•	Types:
•	type State string with values "ONLINE", "OFFLINE", "UNKNOWN".
•	type StationUptime struct { ID string; Online, Offline time.Duration } to hold report A results.
•	type StationStatus struct { ID string; State State; Since time.Time; Duration time.Duration } to hold report B results.
•	Service functions:
1.	func ComputeUptime(ctx context.Context, from, to time.Time, stationIDs []string) ([]StationUptime, error) implements the period analysis using either per‑station or batch streaming logic.
2.	func GetCurrentStatuses(ctx context.Context, stationIDs []string, now time.Time) ([]StationStatus, error) implements the current status report.
•	Supporting functions: A helper stateFromText(text string) State performs the regex or substring matching for “registered” and “unregistered”.  Additional helpers may fetch starting states or stream events with a MongoDB cursor.

By following this flow, a Go backend can compute reliable uptime statistics and current status reports for charging stations, even across large datasets and varied time windows.  The logic separates data retrieval from business rules, enabling testing and future enhancements (such as caching or preaggregated daily summaries).