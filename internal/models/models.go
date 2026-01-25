package models

import (
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
