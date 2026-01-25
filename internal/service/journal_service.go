package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/trogers1052/trading-journal/internal/database"
	"github.com/trogers1052/trading-journal/internal/models"
	"github.com/trogers1052/trading-journal/internal/telegram"
)

// JournalService orchestrates the trading journal workflow
type JournalService struct {
	repo            *database.Repository
	bot             *telegram.Bot
	pendingPositions []*models.Position // Queue of positions needing journal entries
}

// NewJournalService creates a new journal service
func NewJournalService(repo *database.Repository, bot *telegram.Bot) *JournalService {
	svc := &JournalService{
		repo: repo,
		bot:  bot,
	}

	// Set up the callback for when journal entries are complete
	bot.SetJournalCompleteCallback(svc.onJournalComplete)

	return svc
}

// HandleTradeEvent processes a trade event from Kafka
func (s *JournalService) HandleTradeEvent(ctx context.Context, event *models.TradeEvent) error {
	data := event.Data

	log.Printf("Processing trade: %s %s %s @ %s",
		data.Side, data.Quantity, data.Symbol, data.AveragePrice)

	// Parse trade data
	quantity, _ := strconv.ParseFloat(data.Quantity, 64)
	price, _ := strconv.ParseFloat(data.AveragePrice, 64)
	totalAmount, _ := strconv.ParseFloat(data.TotalNotional, 64)
	fees, _ := strconv.ParseFloat(data.Fees, 64)

	var executedAt time.Time
	if data.ExecutedAt != nil {
		executedAt, _ = time.Parse(time.RFC3339, *data.ExecutedAt)
	} else {
		executedAt = event.Timestamp
	}

	trade := &models.Trade{
		OrderID:     data.OrderID,
		Symbol:      data.Symbol,
		Side:        data.Side,
		Quantity:    quantity,
		Price:       price,
		TotalAmount: totalAmount,
		Fees:        fees,
		ExecutedAt:  executedAt,
	}

	// Check if trade already exists
	existingTrade, err := s.repo.GetTradeByOrderID(trade.OrderID)
	if err != nil {
		return fmt.Errorf("failed to check existing trade: %w", err)
	}
	if existingTrade != nil {
		log.Printf("Trade %s already processed, skipping", trade.OrderID)
		return nil
	}

	// Insert the trade
	if err := s.repo.InsertTrade(trade); err != nil {
		return fmt.Errorf("failed to insert trade: %w", err)
	}

	// Handle based on side
	if data.Side == "buy" {
		return s.handleBuyTrade(trade)
	} else if data.Side == "sell" {
		return s.handleSellTrade(trade)
	}

	return nil
}

// handleBuyTrade processes a buy trade - opens a new position
func (s *JournalService) handleBuyTrade(trade *models.Trade) error {
	// Check if there's already an open position for this symbol
	existingPosition, err := s.repo.GetOpenPosition(trade.Symbol)
	if err != nil {
		return fmt.Errorf("failed to check existing position: %w", err)
	}

	if existingPosition != nil {
		// Adding to existing position - for simplicity, we'll track each buy separately
		// In a more complex system, we'd average the entry price
		log.Printf("Adding to existing position for %s", trade.Symbol)
	}

	// Create a new position
	position, err := s.repo.CreatePosition(trade)
	if err != nil {
		return fmt.Errorf("failed to create position: %w", err)
	}

	// Link the trade to the position
	if err := s.repo.UpdateTradePositionID(trade.OrderID, position.ID); err != nil {
		log.Printf("Warning: failed to link trade to position: %v", err)
	}

	log.Printf("Created new position %d for %s", position.ID, trade.Symbol)
	return nil
}

// handleSellTrade processes a sell trade - closes a position
func (s *JournalService) handleSellTrade(trade *models.Trade) error {
	// Find the open position for this symbol
	position, err := s.repo.GetOpenPosition(trade.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get open position: %w", err)
	}

	if position == nil {
		// No open position found - this could be a short or an untracked position
		log.Printf("No open position found for %s, creating closed position record", trade.Symbol)
		return nil
	}

	// Close the position
	if err := s.repo.ClosePosition(position.ID, trade); err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	// Link the trade to the position
	if err := s.repo.UpdateTradePositionID(trade.OrderID, position.ID); err != nil {
		log.Printf("Warning: failed to link trade to position: %v", err)
	}

	// Refresh position data with P&L calculated
	closedPosition, err := s.repo.GetPositionByID(position.ID)
	if err != nil {
		return fmt.Errorf("failed to get closed position: %w", err)
	}

	log.Printf("Closed position %d for %s (P&L: $%.2f)",
		position.ID, trade.Symbol, *closedPosition.RealizedPL)

	// Notify and start journal prompt
	s.bot.SendPositionClosedNotification(closedPosition)

	// Start the journal prompt after a short delay
	time.AfterFunc(2*time.Second, func() {
		s.bot.StartJournalPrompt(closedPosition)
	})

	return nil
}

// onJournalComplete is called when a journal entry is completed via Telegram
func (s *JournalService) onJournalComplete(entry *models.JournalEntry) error {
	if err := s.repo.InsertJournalEntry(entry); err != nil {
		return fmt.Errorf("failed to save journal entry: %w", err)
	}

	log.Printf("Saved journal entry for position %d (%s)", entry.PositionID, entry.Symbol)

	// Check if there are more pending positions
	s.promptNextPendingPosition()

	return nil
}

// CatchUpPendingJournals finds all closed positions without journal entries and prompts for them
func (s *JournalService) CatchUpPendingJournals() error {
	positions, err := s.repo.GetClosedPositionsWithoutJournal()
	if err != nil {
		return fmt.Errorf("failed to get pending positions: %w", err)
	}

	if len(positions) == 0 {
		log.Println("All positions have journal entries")
		return nil
	}

	log.Printf("Found %d positions without journal entries", len(positions))

	// Store pending positions
	s.pendingPositions = positions

	// Send summary
	s.bot.SendPendingJournalsSummary(positions)

	// Start with the first one after a delay
	time.AfterFunc(3*time.Second, func() {
		s.promptNextPendingPosition()
	})

	return nil
}

// promptNextPendingPosition prompts for the next pending position
func (s *JournalService) promptNextPendingPosition() {
	if len(s.pendingPositions) == 0 {
		return
	}

	// Don't start a new prompt if one is already active
	if s.bot.HasActivePrompt() {
		return
	}

	// Get the next position
	position := s.pendingPositions[0]
	s.pendingPositions = s.pendingPositions[1:]

	// Start the journal prompt
	s.bot.StartJournalPrompt(position)
}

// GetStats returns journal statistics
func (s *JournalService) GetStats() (map[string]interface{}, error) {
	return s.repo.GetJournalStats()
}
