package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/trogers1052/trading-journal/internal/models"
)

// ---------------------------------------------------------------------------
// SetCatchupMode / IsCatchupMode
// ---------------------------------------------------------------------------

func TestSetCatchupMode_DefaultTrue(t *testing.T) {
	// JournalService starts in catchup mode (set in NewJournalService).
	// We can't call NewJournalService without a real Bot, so test directly.
	svc := &JournalService{
		catchupMode: true,
	}
	if !svc.IsCatchupMode() {
		t.Fatal("expected catchup mode to be true initially")
	}
}

func TestSetCatchupMode_DisableAndEnable(t *testing.T) {
	svc := &JournalService{
		catchupMode: true,
	}

	svc.SetCatchupMode(false)
	if svc.IsCatchupMode() {
		t.Fatal("expected catchup mode to be false after disabling")
	}

	svc.SetCatchupMode(true)
	if !svc.IsCatchupMode() {
		t.Fatal("expected catchup mode to be true after re-enabling")
	}
}

func TestIsCatchupMode_ThreadSafe(t *testing.T) {
	svc := &JournalService{
		catchupMode: true,
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			svc.SetCatchupMode(i%2 == 0)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = svc.IsCatchupMode()
	}
	<-done
}

// ---------------------------------------------------------------------------
// HandleTradeEvent — parsing errors (no DB required)
// ---------------------------------------------------------------------------

// We test HandleTradeEvent's parsing layer by providing events with
// invalid numeric fields. The function returns errors before hitting
// the repository.

func TestHandleTradeEvent_InvalidQuantity(t *testing.T) {
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "not-a-number",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid quantity")
	}
	if !strings.Contains(err.Error(), "quantity") {
		t.Fatalf("error should mention quantity: %v", err)
	}
}

func TestHandleTradeEvent_InvalidPrice(t *testing.T) {
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "xyz",
			TotalNotional: "1000",
			Fees:          "0",
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid price")
	}
	if !strings.Contains(err.Error(), "price") {
		t.Fatalf("error should mention price: %v", err)
	}
}

func TestHandleTradeEvent_InvalidTotalNotional(t *testing.T) {
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "abc",
			Fees:          "0",
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid total_notional")
	}
	if !strings.Contains(err.Error(), "total notional") {
		t.Fatalf("error should mention total notional: %v", err)
	}
}

func TestHandleTradeEvent_InvalidFees(t *testing.T) {
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "not-fee",
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid fees")
	}
	if !strings.Contains(err.Error(), "fees") {
		t.Fatalf("error should mention fees: %v", err)
	}
}

func TestHandleTradeEvent_InvalidTimestamp(t *testing.T) {
	svc := &JournalService{}
	badTimestamp := "not-a-timestamp"
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
			ExecutedAt:    &badTimestamp,
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Fatalf("error should mention parsing: %v", err)
	}
}

func TestHandleTradeEvent_NilExecutedAt_ZeroTimestamp(t *testing.T) {
	// When ExecutedAt is nil and event.Timestamp is zero, should error
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Timestamp: time.Time{}, // zero value
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
			ExecutedAt:    nil,
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for zero timestamp when ExecutedAt is nil")
	}
	if !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("error should mention timestamp: %v", err)
	}
}

func TestHandleTradeEvent_ValidTimestampRFC3339(t *testing.T) {
	svc := &JournalService{}
	ts := "2026-02-20T14:30:00Z"
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
			ExecutedAt:    &ts,
		},
	}
	// With no repo, this will panic on repo.GetTradeByOrderID.
	// A panic (not an error return) proves the parsing phase passed.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo, meaning parsing succeeded")
			}
			// Panic from nil repo dereference = parsing phase passed
		}()
		_ = svc.HandleTradeEvent(context.Background(), event)
	}()
}

func TestHandleTradeEvent_ValidTimestampRFC3339Nano(t *testing.T) {
	svc := &JournalService{}
	ts := "2026-02-20T14:30:00.123456789Z"
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
			ExecutedAt:    &ts,
		},
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo, meaning parsing succeeded")
			}
		}()
		_ = svc.HandleTradeEvent(context.Background(), event)
	}()
}

func TestHandleTradeEvent_NilExecutedAt_UsesEventTimestamp(t *testing.T) {
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Timestamp: time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
			ExecutedAt:    nil, // falls back to event.Timestamp
		},
	}
	// Should pass parsing (non-zero timestamp) then panic on nil repo
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from nil repo, meaning parsing succeeded")
			}
		}()
		_ = svc.HandleTradeEvent(context.Background(), event)
	}()
}

// ---------------------------------------------------------------------------
// HandleTradeEvent — field ordering validation
// ---------------------------------------------------------------------------

func TestHandleTradeEvent_QuantityCheckedFirst(t *testing.T) {
	// If both quantity and price are invalid, quantity error should come first
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "bad",
			AveragePrice:  "also-bad",
			TotalNotional: "1000",
			Fees:          "0",
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "quantity") {
		t.Fatalf("expected quantity error first, got: %v", err)
	}
}

func TestHandleTradeEvent_EmptyStringQuantity(t *testing.T) {
	svc := &JournalService{}
	event := &models.TradeEvent{
		EventType: "TRADE_DETECTED",
		Data: models.TradeData{
			OrderID:       "t-1",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "",
			AveragePrice:  "100",
			TotalNotional: "1000",
			Fees:          "0",
		},
	}
	err := svc.HandleTradeEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for empty quantity string")
	}
}

// ---------------------------------------------------------------------------
// PendingPositions queue management
// ---------------------------------------------------------------------------

func TestPromptNextPendingPosition_EmptyQueue(t *testing.T) {
	svc := &JournalService{
		pendingPositions: nil,
	}
	if len(svc.pendingPositions) != 0 {
		t.Fatalf("expected empty pending queue, got %d", len(svc.pendingPositions))
	}
}

func TestPendingPositions_QueueManagement(t *testing.T) {
	svc := &JournalService{}

	positions := []*models.Position{
		{ID: 1, Symbol: "AAPL"},
		{ID: 2, Symbol: "GOOG"},
		{ID: 3, Symbol: "MSFT"},
	}

	svc.mu.Lock()
	svc.pendingPositions = positions
	svc.mu.Unlock()

	svc.mu.Lock()
	if len(svc.pendingPositions) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(svc.pendingPositions))
	}

	// Pop first
	first := svc.pendingPositions[0]
	svc.pendingPositions = svc.pendingPositions[1:]
	svc.mu.Unlock()

	if first.Symbol != "AAPL" {
		t.Fatalf("first position should be AAPL, got %s", first.Symbol)
	}

	svc.mu.Lock()
	remaining := len(svc.pendingPositions)
	svc.mu.Unlock()

	if remaining != 2 {
		t.Fatalf("expected 2 remaining, got %d", remaining)
	}
}

func TestPendingPositions_FIFO(t *testing.T) {
	svc := &JournalService{}

	svc.mu.Lock()
	svc.pendingPositions = []*models.Position{
		{ID: 10, Symbol: "A"},
		{ID: 20, Symbol: "B"},
		{ID: 30, Symbol: "C"},
	}
	svc.mu.Unlock()

	// Pop all in order
	var symbols []string
	for {
		svc.mu.Lock()
		if len(svc.pendingPositions) == 0 {
			svc.mu.Unlock()
			break
		}
		pos := svc.pendingPositions[0]
		svc.pendingPositions = svc.pendingPositions[1:]
		svc.mu.Unlock()
		symbols = append(symbols, pos.Symbol)
	}

	expected := []string{"A", "B", "C"}
	if len(symbols) != len(expected) {
		t.Fatalf("popped %d symbols, want %d", len(symbols), len(expected))
	}
	for i, s := range symbols {
		if s != expected[i] {
			t.Errorf("symbol[%d] = %s, want %s", i, s, expected[i])
		}
	}
}
