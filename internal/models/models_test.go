package models

import (
	"math"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helper: build a valid TradeEvent for mutation in tests
// ---------------------------------------------------------------------------

func validTradeEvent() *TradeEvent {
	return &TradeEvent{
		EventType: "TRADE_DETECTED",
		Source:    "robinhood",
		Timestamp: time.Now(),
		Data: TradeData{
			OrderID:       "order-001",
			Symbol:        "AAPL",
			Side:          "buy",
			Quantity:      "10",
			AveragePrice:  "150.50",
			TotalNotional: "1505.00",
			Fees:          "0.00",
			State:         "filled",
			CreatedAt:     "2026-02-20T14:00:00Z",
		},
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — nil / empty top-level
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_NilEvent(t *testing.T) {
	if err := ValidateTradeEvent(nil); err == nil {
		t.Fatal("expected error for nil event")
	}
}

func TestValidateTradeEvent_EmptyEventType(t *testing.T) {
	e := validTradeEvent()
	e.EventType = ""
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for empty event_type")
	}
}

func TestValidateTradeEvent_WhitespaceEventType(t *testing.T) {
	e := validTradeEvent()
	e.EventType = "   "
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for whitespace event_type")
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — symbol
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_EmptySymbol(t *testing.T) {
	e := validTradeEvent()
	e.Data.Symbol = ""
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error for empty symbol")
	}
	if !strings.Contains(err.Error(), "symbol") {
		t.Fatalf("error should mention symbol: %v", err)
	}
}

func TestValidateTradeEvent_WhitespaceSymbol(t *testing.T) {
	e := validTradeEvent()
	e.Data.Symbol = "   "
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for whitespace symbol")
	}
}

func TestValidateTradeEvent_SymbolTooLong(t *testing.T) {
	e := validTradeEvent()
	e.Data.Symbol = "ABCDEFGHIJK" // 11 chars
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error for symbol exceeding max length")
	}
	if !strings.Contains(err.Error(), "max length") {
		t.Fatalf("error should mention max length: %v", err)
	}
}

func TestValidateTradeEvent_SymbolAtMaxLength(t *testing.T) {
	e := validTradeEvent()
	e.Data.Symbol = "ABCDEFGHIJ" // exactly 10 chars
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("symbol at max length should be valid: %v", err)
	}
}

func TestValidateTradeEvent_SymbolWithWhitespace(t *testing.T) {
	e := validTradeEvent()
	e.Data.Symbol = " AAPL "
	// Should pass — trimmed symbol is "AAPL" which is 4 chars
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("symbol with leading/trailing whitespace should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — side
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_InvalidSide(t *testing.T) {
	e := validTradeEvent()
	e.Data.Side = "hold"
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error for invalid side")
	}
	if !strings.Contains(err.Error(), "side") {
		t.Fatalf("error should mention side: %v", err)
	}
}

func TestValidateTradeEvent_EmptySide(t *testing.T) {
	e := validTradeEvent()
	e.Data.Side = ""
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for empty side")
	}
}

func TestValidateTradeEvent_SideBuyUppercase(t *testing.T) {
	e := validTradeEvent()
	e.Data.Side = "BUY"
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("uppercase BUY should be valid: %v", err)
	}
}

func TestValidateTradeEvent_SideSellMixedCase(t *testing.T) {
	e := validTradeEvent()
	e.Data.Side = "Sell"
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("mixed case Sell should be valid: %v", err)
	}
}

func TestValidateTradeEvent_SideWithWhitespace(t *testing.T) {
	e := validTradeEvent()
	e.Data.Side = " buy "
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("side with whitespace should be valid after trimming: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — quantity
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_EmptyQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = ""
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for empty quantity")
	}
}

func TestValidateTradeEvent_WhitespaceQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "   "
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for whitespace quantity")
	}
}

func TestValidateTradeEvent_NonNumericQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "abc"
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error for non-numeric quantity")
	}
	if !strings.Contains(err.Error(), "not a valid number") {
		t.Fatalf("error should mention invalid number: %v", err)
	}
}

func TestValidateTradeEvent_ZeroQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "0"
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error for zero quantity")
	}
	if !strings.Contains(err.Error(), "finite positive") {
		t.Fatalf("error should mention finite positive: %v", err)
	}
}

func TestValidateTradeEvent_NegativeQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "-5"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for negative quantity")
	}
}

func TestValidateTradeEvent_NaNQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "NaN"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for NaN quantity")
	}
}

func TestValidateTradeEvent_InfQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "Inf"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for Inf quantity")
	}
}

func TestValidateTradeEvent_NegInfQuantity(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "-Inf"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for -Inf quantity")
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — average_price
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_EmptyAveragePrice(t *testing.T) {
	e := validTradeEvent()
	e.Data.AveragePrice = ""
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for empty average_price")
	}
}

func TestValidateTradeEvent_NonNumericAveragePrice(t *testing.T) {
	e := validTradeEvent()
	e.Data.AveragePrice = "xyz"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for non-numeric average_price")
	}
}

func TestValidateTradeEvent_ZeroAveragePrice(t *testing.T) {
	e := validTradeEvent()
	e.Data.AveragePrice = "0"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for zero average_price")
	}
}

func TestValidateTradeEvent_NegativeAveragePrice(t *testing.T) {
	e := validTradeEvent()
	e.Data.AveragePrice = "-100.50"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for negative average_price")
	}
}

func TestValidateTradeEvent_NaNAveragePrice(t *testing.T) {
	e := validTradeEvent()
	e.Data.AveragePrice = "NaN"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for NaN average_price")
	}
}

func TestValidateTradeEvent_InfAveragePrice(t *testing.T) {
	e := validTradeEvent()
	e.Data.AveragePrice = "+Inf"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for +Inf average_price")
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — total_notional
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_EmptyTotalNotional(t *testing.T) {
	e := validTradeEvent()
	e.Data.TotalNotional = ""
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for empty total_notional")
	}
}

func TestValidateTradeEvent_NonNumericTotalNotional(t *testing.T) {
	e := validTradeEvent()
	e.Data.TotalNotional = "abc"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for non-numeric total_notional")
	}
}

func TestValidateTradeEvent_ZeroTotalNotional(t *testing.T) {
	e := validTradeEvent()
	e.Data.TotalNotional = "0"
	// Zero is allowed for total_notional (non-negative)
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("zero total_notional should be valid: %v", err)
	}
}

func TestValidateTradeEvent_NegativeTotalNotional(t *testing.T) {
	e := validTradeEvent()
	e.Data.TotalNotional = "-100"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for negative total_notional")
	}
}

func TestValidateTradeEvent_NaNTotalNotional(t *testing.T) {
	e := validTradeEvent()
	e.Data.TotalNotional = "NaN"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for NaN total_notional")
	}
}

func TestValidateTradeEvent_InfTotalNotional(t *testing.T) {
	e := validTradeEvent()
	e.Data.TotalNotional = "Inf"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for Inf total_notional")
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — fees
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_EmptyFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = ""
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for empty fees")
	}
}

func TestValidateTradeEvent_NonNumericFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = "free"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for non-numeric fees")
	}
}

func TestValidateTradeEvent_ZeroFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = "0"
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("zero fees should be valid: %v", err)
	}
}

func TestValidateTradeEvent_NegativeFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = "-1.50"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for negative fees")
	}
}

func TestValidateTradeEvent_NaNFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = "NaN"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for NaN fees")
	}
}

func TestValidateTradeEvent_InfFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = "+Inf"
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for +Inf fees")
	}
}

func TestValidateTradeEvent_PositiveFees(t *testing.T) {
	e := validTradeEvent()
	e.Data.Fees = "2.50"
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("positive fees should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — order_id
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_EmptyOrderID(t *testing.T) {
	e := validTradeEvent()
	e.Data.OrderID = ""
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error for empty order_id")
	}
	if !strings.Contains(err.Error(), "order_id") {
		t.Fatalf("error should mention order_id: %v", err)
	}
}

func TestValidateTradeEvent_WhitespaceOrderID(t *testing.T) {
	e := validTradeEvent()
	e.Data.OrderID = "   "
	if err := ValidateTradeEvent(e); err == nil {
		t.Fatal("expected error for whitespace order_id")
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — valid events
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_ValidBuyEvent(t *testing.T) {
	e := validTradeEvent()
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("valid buy event should pass: %v", err)
	}
}

func TestValidateTradeEvent_ValidSellEvent(t *testing.T) {
	e := validTradeEvent()
	e.Data.Side = "sell"
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("valid sell event should pass: %v", err)
	}
}

func TestValidateTradeEvent_ValidWithAllFields(t *testing.T) {
	execAt := "2026-02-20T14:30:00Z"
	e := &TradeEvent{
		EventType: "TRADE_DETECTED",
		Source:    "robinhood",
		Timestamp: time.Now(),
		Data: TradeData{
			OrderID:       "order-999",
			Symbol:        "GOOG",
			Side:          "sell",
			Quantity:      "5.5",
			AveragePrice:  "2800.99",
			TotalNotional: "15405.45",
			Fees:          "1.25",
			State:         "filled",
			ExecutedAt:    &execAt,
			CreatedAt:     "2026-02-20T14:00:00Z",
		},
	}
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("valid complete event should pass: %v", err)
	}
}

func TestValidateTradeEvent_SmallPositiveValues(t *testing.T) {
	e := validTradeEvent()
	e.Data.Quantity = "0.00000001"
	e.Data.AveragePrice = "0.01"
	e.Data.TotalNotional = "0.00"
	e.Data.Fees = "0.00"
	if err := ValidateTradeEvent(e); err != nil {
		t.Fatalf("small positive values should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateTradeEvent — field ordering (first error wins)
// ---------------------------------------------------------------------------

func TestValidateTradeEvent_MultipleErrors_FirstFieldWins(t *testing.T) {
	// event_type is checked before symbol, which is checked before side, etc.
	e := &TradeEvent{
		EventType: "",
		Data: TradeData{
			Symbol: "",
			Side:   "invalid",
		},
	}
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "event_type") {
		t.Fatalf("first validation error should be event_type, got: %v", err)
	}
}

func TestValidateTradeEvent_SymbolCheckedBeforeSide(t *testing.T) {
	e := validTradeEvent()
	e.Data.Symbol = ""
	e.Data.Side = "invalid"
	err := ValidateTradeEvent(e)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "symbol") {
		t.Fatalf("symbol should be checked before side, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isFinitePositive
// ---------------------------------------------------------------------------

func TestIsFinitePositive(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want bool
	}{
		{"positive", 1.0, true},
		{"large positive", 1e18, true},
		{"small positive", 0.000001, true},
		{"zero", 0, false},
		{"negative", -1.0, false},
		{"large negative", -1e18, false},
		{"NaN", math.NaN(), false},
		{"positive infinity", math.Inf(1), false},
		{"negative infinity", math.Inf(-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFinitePositive(tt.v)
			if got != tt.want {
				t.Errorf("isFinitePositive(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isFiniteNonNegative
// ---------------------------------------------------------------------------

func TestIsFiniteNonNegative(t *testing.T) {
	tests := []struct {
		name string
		v    float64
		want bool
	}{
		{"positive", 1.0, true},
		{"large positive", 1e18, true},
		{"small positive", 0.000001, true},
		{"zero", 0, true},
		{"negative", -1.0, false},
		{"small negative", -0.000001, false},
		{"NaN", math.NaN(), false},
		{"positive infinity", math.Inf(1), false},
		{"negative infinity", math.Inf(-1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFiniteNonNegative(tt.v)
			if got != tt.want {
				t.Errorf("isFiniteNonNegative(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Constants & JournalQuestions
// ---------------------------------------------------------------------------

func TestMaxSymbolLength(t *testing.T) {
	if MaxSymbolLength != 10 {
		t.Fatalf("MaxSymbolLength = %d, want 10", MaxSymbolLength)
	}
}

func TestStepConstants_Ordering(t *testing.T) {
	// iota starts at 0 and increments
	if StepEntryReasoning != 0 {
		t.Fatalf("StepEntryReasoning = %d, want 0", StepEntryReasoning)
	}
	if StepExitReasoning != 1 {
		t.Fatalf("StepExitReasoning = %d, want 1", StepExitReasoning)
	}
	if StepComplete != 9 {
		t.Fatalf("StepComplete = %d, want 9", StepComplete)
	}
	// StepComplete should be one more than the last question step
	if StepComplete != StepNotes+1 {
		t.Fatalf("StepComplete should be StepNotes+1, got %d vs %d", StepComplete, StepNotes+1)
	}
}

func TestJournalQuestions_AllStepsCovered(t *testing.T) {
	// Every step from 0 to StepComplete-1 should have a question
	for step := StepEntryReasoning; step < StepComplete; step++ {
		q, ok := JournalQuestions[step]
		if !ok {
			t.Errorf("missing JournalQuestions entry for step %d", step)
			continue
		}
		if q == "" {
			t.Errorf("empty question for step %d", step)
		}
	}
}

func TestJournalQuestions_NoExtraSteps(t *testing.T) {
	// Should have exactly StepComplete questions (steps 0 through StepComplete-1)
	expected := StepComplete
	if len(JournalQuestions) != expected {
		t.Fatalf("JournalQuestions has %d entries, want %d", len(JournalQuestions), expected)
	}
}

func TestJournalQuestions_StepCompleteHasNoQuestion(t *testing.T) {
	if _, ok := JournalQuestions[StepComplete]; ok {
		t.Fatal("StepComplete should NOT have a question entry")
	}
}

// ---------------------------------------------------------------------------
// Struct zero values
// ---------------------------------------------------------------------------

func TestTradeEvent_ZeroValue(t *testing.T) {
	var e TradeEvent
	if e.EventType != "" {
		t.Fatal("zero TradeEvent should have empty EventType")
	}
	if e.Data.OrderID != "" {
		t.Fatal("zero TradeEvent.Data should have empty OrderID")
	}
}

func TestJournalPromptState_Responses(t *testing.T) {
	state := JournalPromptState{
		PositionID:  1,
		Symbol:      "AAPL",
		CurrentStep: StepEntryReasoning,
		Responses:   make(map[string]string),
	}
	state.Responses["entry_reasoning"] = "Bullish breakout"
	if state.Responses["entry_reasoning"] != "Bullish breakout" {
		t.Fatal("response storage failed")
	}
}
