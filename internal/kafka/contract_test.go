package kafka_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testkit "github.com/trogers1052/trading-testkit"
	"github.com/trogers1052/trading-journal/internal/models"
)

func TestContract_TradeEvent_Consumer(t *testing.T) {
	raw := testkit.LoadContract("trade_event.json")

	var event models.TradeEvent
	err := json.Unmarshal(raw, &event)
	require.NoError(t, err, "canonical trade_event.json must unmarshal into models.TradeEvent")

	assert.Equal(t, "TRADE_DETECTED", event.EventType)
	assert.Equal(t, "robinhood", event.Source)
	assert.False(t, event.Timestamp.IsZero(), "Timestamp should not be zero")

	assert.Equal(t, "order-contract-001", event.Data.OrderID)
	assert.Equal(t, "PLTR", event.Data.Symbol)
	assert.Equal(t, "buy", event.Data.Side)
	assert.Equal(t, "25", event.Data.Quantity)
	assert.Equal(t, "42.50", event.Data.AveragePrice)
	assert.Equal(t, "1062.50", event.Data.TotalNotional)
	assert.Equal(t, "0.02", event.Data.Fees)
	assert.Equal(t, "filled", event.Data.State)
	require.NotNil(t, event.Data.ExecutedAt, "ExecutedAt should not be nil")
	assert.Equal(t, "2026-03-10T14:29:00Z", event.Data.CreatedAt)
}

func TestContract_TradeEvent_Roundtrip(t *testing.T) {
	raw := testkit.LoadContract("trade_event.json")

	var original models.TradeEvent
	err := json.Unmarshal(raw, &original)
	require.NoError(t, err, "initial unmarshal must succeed")

	marshaled, err := json.Marshal(original)
	require.NoError(t, err, "re-marshal must succeed")

	var roundtripped models.TradeEvent
	err = json.Unmarshal(marshaled, &roundtripped)
	require.NoError(t, err, "roundtrip unmarshal must succeed")

	assert.Equal(t, original.Data.OrderID, roundtripped.Data.OrderID)
	assert.Equal(t, original.Data.Symbol, roundtripped.Data.Symbol)
}
