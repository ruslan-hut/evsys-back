package database

import (
	"context"
	"evsys-back/entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

// Integration tests for the queries behind the MCP server: the log filters,
// the error summary, the all-users active sessions and the OAuth storage.
// Skipped unless MONGO_TEST_URI is set, see concurrency_test.go.

func feature(cp, feat, importance, text string, min int) bson.M {
	return bson.M{"charge_point_id": cp, "feature": feat, "importance": importance, "text": text, "timestamp": mm(min)}
}

func TestReadLogFilters(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	seed(t, m, collectionSysLog,
		feature("CP1", "StatusNotification", "error", "connector 1: Faulted (GroundFailure)", 0),
		feature("CP1", "StatusNotification", "info", "connector 1: Available", 10),
		feature("CP2", "BootNotification", "info", "boot: firmware 1.2.3", 20),
		feature("CP2", "StatusNotification", "info", "connector 2: faulted", 30),
	)

	read := func(f *entity.LogFilter) []entity.FeatureMessage {
		t.Helper()
		out, err := m.ReadLog(ctx, "sys", f)
		require.NoError(t, err)
		return out.([]entity.FeatureMessage)
	}

	all := read(nil)
	require.Len(t, all, 4)
	assert.Equal(t, "connector 2: faulted", all[0].Text, "newest first")

	assert.Len(t, read(&entity.LogFilter{Search: "FAULTED"}), 2, "search ignores case")
	assert.Len(t, read(&entity.LogFilter{Search: "1.2.3"}), 1)
	assert.Len(t, read(&entity.LogFilter{Search: "1.2.4"}), 0)
	assert.Len(t, read(&entity.LogFilter{Search: "1x2"}), 0, "search is literal, not a pattern")
	assert.Len(t, read(&entity.LogFilter{Search: "("}), 1, "regex metacharacters are escaped")
	assert.Len(t, read(&entity.LogFilter{Category: "StatusNotification"}), 3)
	assert.Len(t, read(&entity.LogFilter{Level: "error"}), 1)
	assert.Len(t, read(&entity.LogFilter{ChargePointId: "CP2", Category: "StatusNotification"}), 1)
	from, to := mm(5), mm(25)
	assert.Len(t, read(&entity.LogFilter{From: &from, To: &to}), 2)
	assert.Len(t, read(&entity.LogFilter{Limit: 3}), 3)
}

func TestReadErrorLog(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	seed(t, m, collectionErrorLog,
		bson.M{"charge_point_id": "CP1", "connector_id": 1, "error_code": "GroundFailure", "info": "RCD tripped", "status": "Faulted", "timestamp": mm(0)},
		bson.M{"charge_point_id": "CP1", "connector_id": 1, "error_code": "GroundFailure", "info": "RCD tripped again", "status": "Faulted", "timestamp": mm(60)},
		bson.M{"charge_point_id": "CP1", "connector_id": 2, "error_code": "OtherError", "vendor_error_code": "E42", "status": "Faulted", "timestamp": mm(30)},
		bson.M{"charge_point_id": "CP2", "connector_id": 1, "error_code": "OverVoltage", "status": "Faulted", "timestamp": mm(90)},
		bson.M{"charge_point_id": "CP2", "connector_id": 1, "error_code": "OverVoltage", "status": "Faulted", "timestamp": mm(-600)},
	)

	out, err := m.ReadLog(ctx, "errors", &entity.LogFilter{ChargePointId: "CP1"})
	require.NoError(t, err)
	records := out.([]entity.ErrorRecord)
	require.Len(t, records, 3)
	assert.Equal(t, "RCD tripped again", records[0].Info)

	out, err = m.ReadLog(ctx, "errors", &entity.LogFilter{Search: "e42"})
	require.NoError(t, err)
	assert.Len(t, out.([]entity.ErrorRecord), 1, "search covers the vendor error code")
	out, err = m.ReadLog(ctx, "errors", &entity.LogFilter{Category: "OverVoltage"})
	require.NoError(t, err)
	assert.Len(t, out.([]entity.ErrorRecord), 2, "category is the error code")

	summary, err := m.ErrorSummary(ctx, mm(-1), mm(120), "")
	require.NoError(t, err)
	require.Len(t, summary, 3, "the record outside the period is left out")
	top := summary[0]
	assert.Equal(t, "CP1", top.ChargePointId)
	assert.Equal(t, 1, top.ConnectorId)
	assert.Equal(t, "GroundFailure", top.ErrorCode)
	assert.Equal(t, 2, top.Count)
	assert.True(t, top.First.Equal(mm(0)))
	assert.True(t, top.Last.Equal(mm(60)))
	assert.Equal(t, "RCD tripped again", top.LastInfo)

	summary, err = m.ErrorSummary(ctx, mm(-1), mm(120), "CP2")
	require.NoError(t, err)
	require.Len(t, summary, 1)
	assert.Equal(t, 1, summary[0].Count)
}

func TestAllActiveTransactions(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	seed(t, m, collectionChargePoints,
		bson.M{"charge_point_id": "CP1", "title": "Gate", "access_level": 0},
		bson.M{"charge_point_id": "VIP", "title": "Private", "access_level": 10},
	)
	seed(t, m, collectionConnectors,
		bson.M{"charge_point_id": "CP1", "connector_id": 1, "status": "Charging"},
		bson.M{"charge_point_id": "VIP", "connector_id": 1, "status": "Charging"},
	)
	seed(t, m, collectionTransactions,
		bson.M{"transaction_id": 1, "charge_point_id": "CP1", "connector_id": 1, "id_tag": "A", "is_finished": false, "meter_start": 100, "time_start": mm(0)},
		bson.M{"transaction_id": 2, "charge_point_id": "CP1", "connector_id": 1, "id_tag": "B", "is_finished": true, "meter_start": 0, "meter_stop": 10, "time_start": mm(-100)},
		bson.M{"transaction_id": 3, "charge_point_id": "VIP", "connector_id": 1, "id_tag": "C", "is_finished": false, "time_start": mm(5)},
		bson.M{"transaction_id": 4, "charge_point_id": "GONE", "connector_id": 1, "id_tag": "D", "is_finished": false, "time_start": mm(6)},
	)
	seed(t, m, collectionMeterValues,
		bson.M{"transaction_id": 1, "measurand": "Energy.Active.Import.Register", "value": 1600, "power_rate": 7000, "price": 50, "time": mm(10)},
		bson.M{"transaction_id": 1, "measurand": "Energy.Active.Import.Register", "value": 3100, "power_rate": 7200, "price": 90, "time": mm(20)},
	)

	states, err := m.GetAllActiveTransactions(ctx, 5)
	require.NoError(t, err)
	require.Len(t, states, 1, "finished, invisible and orphaned transactions are left out")
	s := states[0]
	assert.Equal(t, 1, s.TransactionId)
	assert.Equal(t, "Gate", s.ChargePointTitle)
	assert.Equal(t, 3000, s.Consumed)
	assert.Equal(t, 7200, s.PowerRate)
	assert.Len(t, s.MeterValues, 2)

	states, err = m.GetAllActiveTransactions(ctx, 10)
	require.NoError(t, err)
	require.Len(t, states, 2)
	assert.Equal(t, 3, states[0].TransactionId, "newest first")
}

func TestFilteredTransactionsLimitAndRange(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		seed(t, m, collectionTransactions, bson.M{
			"transaction_id": i, "charge_point_id": "CP1", "id_tag": "A", "is_finished": true,
			"time_start": mm(i * 100), "time_stop": mm(i*100 + 50),
			"meter_values": bson.A{bson.M{"value": i, "time": mm(i * 100)}},
		})
	}

	from, to := mm(200), mm(451)
	txs, err := m.GetFilteredTransactions(ctx, &entity.TransactionFilter{From: &from, To: &to})
	require.NoError(t, err)
	ids := func(txs []*entity.Transaction) []int {
		var out []int
		for _, tx := range txs {
			out = append(out, tx.TransactionId)
		}
		return out
	}
	assert.Equal(t, []int{4, 3, 2}, ids(txs), "both bounds apply")
	assert.Len(t, txs[0].MeterValues, 1)

	txs, err = m.GetFilteredTransactions(ctx, &entity.TransactionFilter{Limit: 2, SkipMeterValues: true})
	require.NoError(t, err)
	assert.Equal(t, []int{5, 4}, ids(txs))
	assert.Empty(t, txs[0].MeterValues)
}

func TestOAuthStorage(t *testing.T) {
	m := testMongo(t)
	ctx := context.Background()
	require.NoError(t, m.EnsureOAuthIndexes(ctx))
	require.NoError(t, m.EnsureOAuthIndexes(ctx), "creating the indexes again is a no-op")

	client := &entity.OAuthClient{ClientId: "mcp_1", ClientName: "Claude", RedirectUris: []string{"https://claude.ai/cb"}, CreatedAt: day}
	require.NoError(t, m.SaveOAuthClient(ctx, client))
	assert.Error(t, m.SaveOAuthClient(ctx, client), "client ids are unique")
	got, err := m.GetOAuthClient(ctx, "mcp_1")
	require.NoError(t, err)
	assert.Equal(t, []string{"https://claude.ai/cb"}, got.RedirectUris)
	missing, err := m.GetOAuthClient(ctx, "mcp_2")
	require.NoError(t, err)
	assert.Nil(t, missing)

	for _, tok := range []*entity.OAuthToken{
		{Hash: "h1", Kind: entity.OAuthAccessToken, GrantId: "g1", ExpiresAt: time.Now().Add(time.Hour)},
		{Hash: "h2", Kind: entity.OAuthRefreshToken, GrantId: "g1", ExpiresAt: time.Now().Add(time.Hour)},
		{Hash: "h3", Kind: entity.OAuthAccessToken, GrantId: "g2", ExpiresAt: time.Now().Add(time.Hour)},
	} {
		require.NoError(t, m.SaveOAuthToken(ctx, tok))
	}
	assert.Error(t, m.SaveOAuthToken(ctx, &entity.OAuthToken{Hash: "h1"}), "token hashes are unique")

	tok, err := m.GetOAuthToken(ctx, "h2")
	require.NoError(t, err)
	assert.Equal(t, entity.OAuthRefreshToken, tok.Kind)

	claimed, err := m.DeleteOAuthToken(ctx, "h2")
	require.NoError(t, err)
	assert.True(t, claimed)
	claimed, err = m.DeleteOAuthToken(ctx, "h2")
	require.NoError(t, err)
	assert.False(t, claimed, "only the first delete claims a token")

	require.NoError(t, m.DeleteOAuthGrant(ctx, "g1"))
	tok, err = m.GetOAuthToken(ctx, "h1")
	require.NoError(t, err)
	assert.Nil(t, tok)
	tok, err = m.GetOAuthToken(ctx, "h3")
	require.NoError(t, err)
	assert.NotNil(t, tok, "other grants are untouched")
}

// A missing transaction used to reach getTransactionState as nil and panic,
// which on the MCP path killed the process.
func TestTransactionStateOfUnknownId(t *testing.T) {
	m := testMongo(t)
	state, err := m.GetTransactionState(context.Background(), "", 10, 999999)
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestAllLocations(t *testing.T) {
	m := testMongo(t)
	seed(t, m, collectionLocations,
		bson.M{"id": "ELP", "roaming": true, "name": "El Pedernoso"},
		bson.M{"id": "ALP", "roaming": false, "name": "Alpicat"},
		bson.M{"id": "EMPTY", "roaming": false},
	)
	seed(t, m, collectionChargePoints,
		bson.M{"charge_point_id": "PE00001", "location_id": "ELP"},
		bson.M{"charge_point_id": "Wallbox1", "location_id": "ALP"},
		bson.M{"charge_point_id": "Wallbox2", "location_id": "ALP"},
	)
	locations, err := m.GetAllLocations(context.Background())
	require.NoError(t, err)
	require.Len(t, locations, 3, "non-roaming locations and those without charge points are included")
	assert.Equal(t, "ALP", locations[0].Id)
	assert.Len(t, locations[0].ChargePoints, 2)
	assert.Equal(t, "EMPTY", locations[2].Id)
	assert.Empty(t, locations[2].ChargePoints)

	roaming, err := m.GetLocations(context.Background())
	require.NoError(t, err)
	assert.Len(t, roaming, 1, "the REST query is unchanged")
}

func TestUserOutstandingTransactions(t *testing.T) {
	m := testMongo(t)
	seed(t, m, collectionUserTags,
		bson.M{"user_id": "u1", "username": "alice", "id_tag": "TAGA", "is_enabled": false},
		bson.M{"user_id": "u2", "username": "bob", "id_tag": "TAGB", "is_enabled": true},
	)
	paid := func(id int, tag string, amount, billed int, perr string, stop int, finished bool) bson.M {
		doc := bson.M{"transaction_id": id, "id_tag": tag, "is_finished": finished,
			"payment_amount": amount, "payment_billed": billed, "time_stop": mm(stop)}
		if perr != "" {
			doc["payment_error"] = perr
		}
		return doc
	}
	seed(t, m, collectionTransactions,
		paid(1, "TAGA", 1000, 1000, "", 10, true),      // paid
		paid(2, "TAGA", 1000, 0, "", 30, true),         // not charged yet
		paid(3, "TAGA", 500, 500, "SIS0334", 20, true), // declined
		paid(4, "TAGA", 0, 0, "", 40, true),            // free
		paid(5, "TAGA", 800, 0, "", 50, false),         // still running
		paid(6, "TAGB", 900, 0, "", 60, true),          // someone else's
		paid(7, "TAGA", 300, 300, "", 70, true),        // paid, empty error
	)
	txs, err := m.GetUserOutstandingTransactions(context.Background(), "u1")
	require.NoError(t, err)
	var ids []int
	for _, tx := range txs {
		ids = append(ids, tx.TransactionId)
	}
	assert.Equal(t, []int{3, 2}, ids, "unpaid ones of this user, oldest first, on a disabled tag too")

	none, err := m.GetUserOutstandingTransactions(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Empty(t, none)
}
