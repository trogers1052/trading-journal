package models

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// TradeEvent represents a trade event from Kafka (trading.orders topic)
type TradeEvent struct {
	EventType string    `json:"event_type"` // TRADE_DETECTED
	Source    string    `json:"source"`     // robinhood
	Timestamp time.Time `json:"timestamp"`
	Data      TradeData `json:"data"`
}

// TradeData contains the actual trade information
type TradeData struct {
	OrderID       string  `json:"order_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"` // buy, sell
	Quantity      string  `json:"quantity"`
	AveragePrice  string  `json:"average_price"`
	TotalNotional string  `json:"total_notional"`
	Fees          string  `json:"fees"`
	State         string  `json:"state"`
	ExecutedAt    *string `json:"executed_at"`
	CreatedAt     string  `json:"created_at"`
}

// Trade represents a trade stored in the database
type Trade struct {
	ID           int64      `json:"id"`
	OrderID      string     `json:"order_id"`
	Symbol       string     `json:"symbol"`
	Side         string     `json:"side"`
	Quantity     float64    `json:"quantity"`
	Price        float64    `json:"price"`
	TotalAmount  float64    `json:"total_amount"`
	Fees         float64    `json:"fees"`
	ExecutedAt   time.Time  `json:"executed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	PositionID   *int64     `json:"position_id,omitempty"` // Links to closed position if part of one
}

// Position represents a trading position (from entry to exit)
type Position struct {
	ID             int64      `json:"id"`
	Symbol         string     `json:"symbol"`
	EntryOrderID   string     `json:"entry_order_id"`
	ExitOrderID    *string    `json:"exit_order_id,omitempty"`
	EntryPrice     float64    `json:"entry_price"`
	ExitPrice      *float64   `json:"exit_price,omitempty"`
	Quantity       float64    `json:"quantity"`
	EntryDate      time.Time  `json:"entry_date"`
	ExitDate       *time.Time `json:"exit_date,omitempty"`
	RealizedPL     *float64   `json:"realized_pl,omitempty"`      // Profit/Loss in dollars
	RealizedPLPct  *float64   `json:"realized_pl_pct,omitempty"`  // Profit/Loss percentage
	Status         string     `json:"status"`                     // open, closed
	HoldingDays    *int       `json:"holding_days,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// JournalEntry represents a user's journal entry for a closed position
type JournalEntry struct {
	ID              int64      `json:"id"`
	PositionID      int64      `json:"position_id"`
	Symbol          string     `json:"symbol"`
	EntryReasoning  string     `json:"entry_reasoning"`   // Why did you enter?
	ExitReasoning   string     `json:"exit_reasoning"`    // Why did you exit?
	WhatWorked      string     `json:"what_worked"`       // What worked well?
	WhatDidntWork   string     `json:"what_didnt_work"`   // What didn't work?
	LessonsLearned  string     `json:"lessons_learned"`   // Key takeaways
	EmotionalState  string     `json:"emotional_state"`   // How were you feeling?
	WouldRepeat     bool       `json:"would_repeat"`      // Would you take this trade again?
	Rating          int        `json:"rating"`            // 1-5 self-assessment
	Tags            []string   `json:"tags"`              // Custom tags
	Notes           string     `json:"notes"`             // Additional notes
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// JournalPromptState tracks the state of an ongoing journal prompt conversation
type JournalPromptState struct {
	PositionID     int64     `json:"position_id"`
	Symbol         string    `json:"symbol"`
	CurrentStep    int       `json:"current_step"`    // Which question we're on
	Responses      map[string]string `json:"responses"` // Collected responses
	StartedAt      time.Time `json:"started_at"`
	LastPromptAt   time.Time `json:"last_prompt_at"`
}

// ClosedPositionSummary provides a summary for journal prompts
type ClosedPositionSummary struct {
	Position      *Position
	EntryTrade    *Trade
	ExitTrade     *Trade
	HasJournal    bool
}

// Journal prompt steps
const (
	StepEntryReasoning = iota
	StepExitReasoning
	StepWhatWorked
	StepWhatDidntWork
	StepLessonsLearned
	StepEmotionalState
	StepWouldRepeat
	StepRating
	StepNotes
	StepComplete
)

// JournalQuestions maps step numbers to questions
var JournalQuestions = map[int]string{
	StepEntryReasoning:  "Why did you enter this trade? What was your thesis?",
	StepExitReasoning:   "Why did you exit? (profit target, stop loss, changed thesis, etc.)",
	StepWhatWorked:      "What worked well in this trade?",
	StepWhatDidntWork:   "What didn't work or could have been better?",
	StepLessonsLearned:  "What's the key lesson or takeaway from this trade?",
	StepEmotionalState:  "How were you feeling during this trade? (calm, anxious, FOMO, etc.)",
	StepWouldRepeat:     "Would you take this same trade again? (yes/no)",
	StepRating:          "Rate this trade 1-5 (1=poor decision, 5=excellent execution)",
	StepNotes:           "Any additional notes? (or type 'skip' to finish)",
}

// MaxSymbolLength is the maximum allowed length for a stock symbol.
const MaxSymbolLength = 10

// ValidateTradeEvent checks that a TradeEvent from Kafka has all required
// fields present and sane before allowing it to propagate through the system.
// Returns a descriptive error if validation fails, nil otherwise.
func ValidateTradeEvent(event *TradeEvent) error {
	if event == nil {
		return fmt.Errorf("trade event is nil")
	}

	if strings.TrimSpace(event.EventType) == "" {
		return fmt.Errorf("missing event_type")
	}

	d := event.Data

	// --- Symbol ---
	symbol := strings.TrimSpace(d.Symbol)
	if symbol == "" {
		return fmt.Errorf("missing or empty symbol")
	}
	if len(symbol) > MaxSymbolLength {
		return fmt.Errorf("symbol %q exceeds max length of %d", symbol, MaxSymbolLength)
	}

	// --- Side ---
	side := strings.ToLower(strings.TrimSpace(d.Side))
	if side != "buy" && side != "sell" {
		return fmt.Errorf("invalid side %q: must be 'buy' or 'sell'", d.Side)
	}

	// --- Quantity ---
	if strings.TrimSpace(d.Quantity) == "" {
		return fmt.Errorf("missing quantity")
	}
	qty, err := strconv.ParseFloat(d.Quantity, 64)
	if err != nil {
		return fmt.Errorf("quantity %q is not a valid number: %w", d.Quantity, err)
	}
	if !isFinitePositive(qty) {
		return fmt.Errorf("quantity must be a finite positive number, got %v", qty)
	}

	// --- Average Price ---
	if strings.TrimSpace(d.AveragePrice) == "" {
		return fmt.Errorf("missing average_price")
	}
	price, err := strconv.ParseFloat(d.AveragePrice, 64)
	if err != nil {
		return fmt.Errorf("average_price %q is not a valid number: %w", d.AveragePrice, err)
	}
	if !isFinitePositive(price) {
		return fmt.Errorf("average_price must be a finite positive number, got %v", price)
	}

	// --- Total Notional ---
	if strings.TrimSpace(d.TotalNotional) == "" {
		return fmt.Errorf("missing total_notional")
	}
	notional, err := strconv.ParseFloat(d.TotalNotional, 64)
	if err != nil {
		return fmt.Errorf("total_notional %q is not a valid number: %w", d.TotalNotional, err)
	}
	if !isFiniteNonNegative(notional) {
		return fmt.Errorf("total_notional must be a finite non-negative number, got %v", notional)
	}

	// --- Fees (allowed to be zero) ---
	if strings.TrimSpace(d.Fees) == "" {
		return fmt.Errorf("missing fees")
	}
	fees, err := strconv.ParseFloat(d.Fees, 64)
	if err != nil {
		return fmt.Errorf("fees %q is not a valid number: %w", d.Fees, err)
	}
	if !isFiniteNonNegative(fees) {
		return fmt.Errorf("fees must be a finite non-negative number, got %v", fees)
	}

	// --- Order ID ---
	if strings.TrimSpace(d.OrderID) == "" {
		return fmt.Errorf("missing order_id")
	}

	return nil
}

// isFinitePositive returns true if v is a normal positive finite number (> 0).
func isFinitePositive(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0
}

// isFiniteNonNegative returns true if v is a normal non-negative finite number (>= 0).
func isFiniteNonNegative(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}
