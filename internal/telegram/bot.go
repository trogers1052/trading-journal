package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/trogers1052/trading-journal/internal/models"
)

// JournalCompleteCallback is called when a journal entry is complete
type JournalCompleteCallback func(entry *models.JournalEntry) error

// Bot handles Telegram interactions for journaling
type Bot struct {
	api                   *tgbotapi.BotAPI
	chatID                int64
	activePrompts         map[int64]*models.JournalPromptState // positionID -> state
	promptsMu             sync.RWMutex
	onJournalComplete     JournalCompleteCallback
	updates               tgbotapi.UpdatesChannel
	ctx                   context.Context
	cancel                context.CancelFunc
}

// NewBot creates a new Telegram bot
func NewBot(token string, chatID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	log.Printf("Authorized on Telegram account %s", api.Self.UserName)

	return &Bot{
		api:           api,
		chatID:        chatID,
		activePrompts: make(map[int64]*models.JournalPromptState),
	}, nil
}

// SetJournalCompleteCallback sets the callback for when a journal entry is complete
func (b *Bot) SetJournalCompleteCallback(cb JournalCompleteCallback) {
	b.onJournalComplete = cb
}

// Start begins listening for messages
func (b *Bot) Start(ctx context.Context) error {
	b.ctx, b.cancel = context.WithCancel(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	b.updates = b.api.GetUpdatesChan(u)

	go b.processUpdates()

	log.Println("Telegram bot started, listening for messages...")
	return nil
}

// Stop stops the bot
func (b *Bot) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.api.StopReceivingUpdates()
}

// processUpdates handles incoming messages
func (b *Bot) processUpdates() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case update := <-b.updates:
			if update.Message == nil {
				continue
			}

			// Only respond to messages from the configured chat
			if update.Message.Chat.ID != b.chatID {
				continue
			}

			b.handleMessage(update.Message)
		}
	}
}

// handleMessage processes an incoming message
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)

	// Handle commands
	if strings.HasPrefix(text, "/") {
		b.handleCommand(msg)
		return
	}

	// Check if we have an active prompt waiting for a response
	b.promptsMu.RLock()
	var activeState *models.JournalPromptState
	var activePositionID int64
	for posID, state := range b.activePrompts {
		if state.CurrentStep < models.StepComplete {
			activeState = state
			activePositionID = posID
			break
		}
	}
	b.promptsMu.RUnlock()

	if activeState == nil {
		b.sendMessage("No active journal prompt. Use /pending to see trades needing journal entries.")
		return
	}

	// Process the response
	b.processResponse(activePositionID, activeState, text)
}

// handleCommand handles bot commands
func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	cmd := msg.Command()

	switch cmd {
	case "start":
		b.sendMessage("Welcome to Trading Journal Bot!\n\n" +
			"I'll help you journal your trades for better learning and improvement.\n\n" +
			"Commands:\n" +
			"/pending - Show trades needing journal entries\n" +
			"/stats - Show journal statistics\n" +
			"/skip - Skip current journal prompt\n" +
			"/help - Show this help message")

	case "help":
		b.sendMessage("Trading Journal Bot Commands:\n\n" +
			"/pending - Show trades needing journal entries\n" +
			"/stats - Show journal statistics\n" +
			"/skip - Skip current journal prompt\n" +
			"/cancel - Cancel current journal entry")

	case "skip":
		b.skipCurrentPrompt()

	case "cancel":
		b.cancelCurrentPrompt()

	default:
		b.sendMessage("Unknown command. Use /help to see available commands.")
	}
}

// StartJournalPrompt begins a journal prompt sequence for a closed position
func (b *Bot) StartJournalPrompt(position *models.Position) {
	b.promptsMu.Lock()
	defer b.promptsMu.Unlock()

	// Check if already prompting for this position
	if _, exists := b.activePrompts[position.ID]; exists {
		return
	}

	state := &models.JournalPromptState{
		PositionID:   position.ID,
		Symbol:       position.Symbol,
		CurrentStep:  models.StepEntryReasoning,
		Responses:    make(map[string]string),
		StartedAt:    time.Now(),
		LastPromptAt: time.Now(),
	}

	b.activePrompts[position.ID] = state

	// Send the position summary first
	b.sendPositionSummary(position)

	// Then send the first question
	time.Sleep(500 * time.Millisecond) // Small delay for readability
	b.sendNextQuestion(state)
}

// sendPositionSummary sends a summary of the closed position
func (b *Bot) sendPositionSummary(position *models.Position) {
	var emoji string
	var plStr string

	if position.RealizedPL != nil {
		if *position.RealizedPL >= 0 {
			emoji = "🟢"
			plStr = fmt.Sprintf("+$%.2f (+%.2f%%)", *position.RealizedPL, *position.RealizedPLPct)
		} else {
			emoji = "🔴"
			plStr = fmt.Sprintf("-$%.2f (%.2f%%)", -*position.RealizedPL, *position.RealizedPLPct)
		}
	}

	holdingDaysStr := ""
	if position.HoldingDays != nil {
		holdingDaysStr = fmt.Sprintf("%d days", *position.HoldingDays)
	}

	msg := fmt.Sprintf("%s <b>Position Closed: %s</b>\n\n"+
		"📊 <b>Trade Summary:</b>\n"+
		"• Entry: $%.2f\n"+
		"• Exit: $%.2f\n"+
		"• Quantity: %.4f\n"+
		"• P&L: %s\n"+
		"• Held: %s\n\n"+
		"📝 <b>Time to journal this trade!</b>\n"+
		"Answer the following questions to capture your learning.",
		emoji, position.Symbol,
		position.EntryPrice,
		*position.ExitPrice,
		position.Quantity,
		plStr,
		holdingDaysStr,
	)

	b.sendHTMLMessage(msg)
}

// sendNextQuestion sends the next question in the journal prompt sequence
func (b *Bot) sendNextQuestion(state *models.JournalPromptState) {
	question, ok := models.JournalQuestions[state.CurrentStep]
	if !ok {
		return
	}

	stepNum := state.CurrentStep + 1
	totalSteps := len(models.JournalQuestions)

	msg := fmt.Sprintf("📝 <b>Question %d/%d</b>\n\n%s", stepNum, totalSteps, question)
	b.sendHTMLMessage(msg)

	state.LastPromptAt = time.Now()
}

// processResponse processes a user's response to a journal prompt
func (b *Bot) processResponse(positionID int64, state *models.JournalPromptState, response string) {
	b.promptsMu.Lock()
	defer b.promptsMu.Unlock()

	// Store the response based on current step
	switch state.CurrentStep {
	case models.StepEntryReasoning:
		state.Responses["entry_reasoning"] = response
	case models.StepExitReasoning:
		state.Responses["exit_reasoning"] = response
	case models.StepWhatWorked:
		state.Responses["what_worked"] = response
	case models.StepWhatDidntWork:
		state.Responses["what_didnt_work"] = response
	case models.StepLessonsLearned:
		state.Responses["lessons_learned"] = response
	case models.StepEmotionalState:
		state.Responses["emotional_state"] = response
	case models.StepWouldRepeat:
		wouldRepeat := strings.ToLower(response)
		if strings.Contains(wouldRepeat, "yes") || strings.Contains(wouldRepeat, "y") {
			state.Responses["would_repeat"] = "true"
		} else {
			state.Responses["would_repeat"] = "false"
		}
	case models.StepRating:
		// Validate rating
		rating, err := strconv.Atoi(strings.TrimSpace(response))
		if err != nil || rating < 1 || rating > 5 {
			b.sendMessage("Please enter a number between 1 and 5.")
			return
		}
		state.Responses["rating"] = response
	case models.StepNotes:
		if strings.ToLower(response) != "skip" {
			state.Responses["notes"] = response
		}
	}

	// Move to next step
	state.CurrentStep++

	// Check if complete
	if state.CurrentStep >= models.StepComplete {
		b.completeJournal(positionID, state)
		return
	}

	// Send next question
	b.sendNextQuestion(state)
}

// completeJournal finalizes the journal entry
func (b *Bot) completeJournal(positionID int64, state *models.JournalPromptState) {
	// Build the journal entry
	wouldRepeat := state.Responses["would_repeat"] == "true"
	rating, _ := strconv.Atoi(state.Responses["rating"])

	entry := &models.JournalEntry{
		PositionID:     positionID,
		Symbol:         state.Symbol,
		EntryReasoning: state.Responses["entry_reasoning"],
		ExitReasoning:  state.Responses["exit_reasoning"],
		WhatWorked:     state.Responses["what_worked"],
		WhatDidntWork:  state.Responses["what_didnt_work"],
		LessonsLearned: state.Responses["lessons_learned"],
		EmotionalState: state.Responses["emotional_state"],
		WouldRepeat:    wouldRepeat,
		Rating:         rating,
		Notes:          state.Responses["notes"],
		Tags:           []string{}, // TODO: Extract tags from responses
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Remove from active prompts
	delete(b.activePrompts, positionID)

	// Call the completion callback
	if b.onJournalComplete != nil {
		if err := b.onJournalComplete(entry); err != nil {
			b.sendMessage(fmt.Sprintf("Error saving journal entry: %v", err))
			return
		}
	}

	// Send confirmation
	b.sendHTMLMessage(fmt.Sprintf(
		"✅ <b>Journal Entry Saved!</b>\n\n"+
			"Symbol: %s\n"+
			"Rating: %d/5\n"+
			"Would repeat: %v\n\n"+
			"Great job reflecting on this trade! 📈",
		state.Symbol, rating, wouldRepeat,
	))
}

// skipCurrentPrompt skips to the next pending position
func (b *Bot) skipCurrentPrompt() {
	b.promptsMu.Lock()
	defer b.promptsMu.Unlock()

	// Find and remove the current active prompt
	for posID := range b.activePrompts {
		delete(b.activePrompts, posID)
		b.sendMessage("Skipped current journal prompt. Use /pending to see remaining trades.")
		return
	}

	b.sendMessage("No active journal prompt to skip.")
}

// cancelCurrentPrompt cancels the current journal entry
func (b *Bot) cancelCurrentPrompt() {
	b.skipCurrentPrompt()
}

// HasActivePrompt checks if there's an active prompt
func (b *Bot) HasActivePrompt() bool {
	b.promptsMu.RLock()
	defer b.promptsMu.RUnlock()
	return len(b.activePrompts) > 0
}

// sendMessage sends a plain text message (internal)
func (b *Bot) sendMessage(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

// SendMessage sends a plain text message (public)
func (b *Bot) SendMessage(text string) {
	b.sendMessage(text)
}

// sendHTMLMessage sends an HTML-formatted message
func (b *Bot) sendHTMLMessage(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Failed to send HTML message: %v", err)
	}
}

// SendPositionClosedNotification sends a notification that a position was closed
func (b *Bot) SendPositionClosedNotification(position *models.Position) {
	var emoji string
	var plStr string

	if position.RealizedPL != nil {
		if *position.RealizedPL >= 0 {
			emoji = "🟢"
			plStr = fmt.Sprintf("+$%.2f (+%.2f%%)", *position.RealizedPL, *position.RealizedPLPct)
		} else {
			emoji = "🔴"
			plStr = fmt.Sprintf("-$%.2f (%.2f%%)", -*position.RealizedPL, *position.RealizedPLPct)
		}
	}

	msg := fmt.Sprintf("%s <b>Position Closed: %s</b>\n\nP&L: %s\n\nStarting journal entry...",
		emoji, position.Symbol, plStr)

	b.sendHTMLMessage(msg)
}

// SendPendingJournalsSummary sends a summary of positions needing journal entries
func (b *Bot) SendPendingJournalsSummary(positions []*models.Position) {
	if len(positions) == 0 {
		b.sendMessage("All trades have been journaled!")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>%d Trades Pending Journal</b>\n\n", len(positions)))

	for i, pos := range positions {
		var plStr string
		if pos.RealizedPL != nil {
			if *pos.RealizedPL >= 0 {
				plStr = fmt.Sprintf("+$%.2f", *pos.RealizedPL)
			} else {
				plStr = fmt.Sprintf("-$%.2f", -*pos.RealizedPL)
			}
		}

		exitDate := ""
		if pos.ExitDate != nil {
			exitDate = pos.ExitDate.Format("Jan 2")
		}

		sb.WriteString(fmt.Sprintf("%d. %s - %s (%s)\n", i+1, pos.Symbol, plStr, exitDate))
	}

	sb.WriteString("\nReady to start journaling? The next trade will be prompted shortly.")

	b.sendHTMLMessage(sb.String())
}

// SendStats sends journal statistics
func (b *Bot) SendStats(stats map[string]interface{}) {
	msg := fmt.Sprintf(
		"📊 <b>Journal Statistics</b>\n\n"+
			"Total Positions: %v\n"+
			"Closed Positions: %v\n"+
			"Journaled: %v\n"+
			"Pending: %v\n\n"+
			"Win Rate: %.1f%%\n"+
			"Wins: %v / Losses: %v\n"+
			"Total P&L: $%.2f\n"+
			"Avg Rating: %.1f/5",
		stats["total_positions"],
		stats["closed_positions"],
		stats["journaled_positions"],
		stats["pending_journals"],
		stats["win_rate"],
		stats["wins"],
		stats["losses"],
		stats["total_pl"],
		stats["avg_rating"],
	)

	b.sendHTMLMessage(msg)
}
