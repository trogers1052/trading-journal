package database

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/lib/pq"
	"github.com/trogers1052/trading-journal/internal/models"
)

// Repository handles database operations for the trading journal
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new database repository
func NewRepository(dsn string) (*Repository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &Repository{db: db}

	// Initialize schema
	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Println("Database connection established")
	return repo, nil
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}

// initSchema creates the required tables if they don't exist
func (r *Repository) initSchema() error {
	schema := `
	-- Trades table
	CREATE TABLE IF NOT EXISTS journal_trades (
		id SERIAL PRIMARY KEY,
		order_id VARCHAR(255) UNIQUE NOT NULL,
		symbol VARCHAR(20) NOT NULL,
		side VARCHAR(10) NOT NULL,
		quantity DECIMAL(18, 8) NOT NULL,
		price DECIMAL(18, 8) NOT NULL,
		total_amount DECIMAL(18, 8) NOT NULL,
		fees DECIMAL(18, 8) DEFAULT 0,
		executed_at TIMESTAMP WITH TIME ZONE NOT NULL,
		position_id INTEGER,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	-- Positions table
	CREATE TABLE IF NOT EXISTS journal_positions (
		id SERIAL PRIMARY KEY,
		symbol VARCHAR(20) NOT NULL,
		entry_order_id VARCHAR(255) NOT NULL,
		exit_order_id VARCHAR(255),
		entry_price DECIMAL(18, 8) NOT NULL,
		exit_price DECIMAL(18, 8),
		quantity DECIMAL(18, 8) NOT NULL,
		entry_date TIMESTAMP WITH TIME ZONE NOT NULL,
		exit_date TIMESTAMP WITH TIME ZONE,
		realized_pl DECIMAL(18, 8),
		realized_pl_pct DECIMAL(10, 4),
		holding_days INTEGER,
		status VARCHAR(20) DEFAULT 'open',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		-- Analysis columns (populated by reporting-service)
		rule_compliance_score DECIMAL(10, 4),
		entry_signal_confidence DECIMAL(10, 4),
		entry_signal_type VARCHAR(20),
		position_size_deviation DECIMAL(10, 4),
		exit_type VARCHAR(50),
		risk_metrics_at_entry JSONB,
		analysis_notes TEXT,
		analyzed_at TIMESTAMP WITH TIME ZONE
	);

	-- Journal entries table
	CREATE TABLE IF NOT EXISTS journal_entries (
		id SERIAL PRIMARY KEY,
		position_id INTEGER REFERENCES journal_positions(id) UNIQUE,
		symbol VARCHAR(20) NOT NULL,
		entry_reasoning TEXT,
		exit_reasoning TEXT,
		what_worked TEXT,
		what_didnt_work TEXT,
		lessons_learned TEXT,
		emotional_state VARCHAR(100),
		would_repeat BOOLEAN,
		rating INTEGER CHECK (rating >= 1 AND rating <= 5),
		tags TEXT[],
		notes TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_journal_trades_symbol ON journal_trades(symbol);
	CREATE INDEX IF NOT EXISTS idx_journal_trades_executed_at ON journal_trades(executed_at);
	CREATE INDEX IF NOT EXISTS idx_journal_positions_symbol ON journal_positions(symbol);
	CREATE INDEX IF NOT EXISTS idx_journal_positions_status ON journal_positions(status);
	CREATE INDEX IF NOT EXISTS idx_journal_entries_position_id ON journal_entries(position_id);
	CREATE INDEX IF NOT EXISTS idx_journal_positions_compliance_score ON journal_positions(rule_compliance_score);
	CREATE INDEX IF NOT EXISTS idx_journal_positions_exit_type ON journal_positions(exit_type);
	CREATE INDEX IF NOT EXISTS idx_journal_positions_analyzed_at ON journal_positions(analyzed_at);
	CREATE INDEX IF NOT EXISTS idx_journal_positions_entry_signal_type ON journal_positions(entry_signal_type);
	`

	_, err := r.db.Exec(schema)
	if err != nil {
		return err
	}

	// Run migrations to add columns to existing tables
	return r.runMigrations()
}

// runMigrations adds new columns to existing tables
func (r *Repository) runMigrations() error {
	// Add analysis columns to journal_positions if they don't exist
	// These are added to the CREATE TABLE now, but this handles existing tables
	migrations := []string{
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS rule_compliance_score DECIMAL(10, 4)`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS entry_signal_confidence DECIMAL(10, 4)`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS entry_signal_type VARCHAR(20)`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS position_size_deviation DECIMAL(10, 4)`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS exit_type VARCHAR(50)`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS risk_metrics_at_entry JSONB`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS analysis_notes TEXT`,
		`ALTER TABLE journal_positions ADD COLUMN IF NOT EXISTS analyzed_at TIMESTAMP WITH TIME ZONE`,
	}

	for _, migration := range migrations {
		if _, err := r.db.Exec(migration); err != nil {
			// Log but don't fail - column might already exist in older PostgreSQL versions
			log.Printf("Migration note: %v", err)
		}
	}

	return nil
}

// InsertTrade inserts a new trade record
func (r *Repository) InsertTrade(trade *models.Trade) error {
	query := `
		INSERT INTO journal_trades (order_id, symbol, side, quantity, price, total_amount, fees, executed_at, position_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (order_id) DO NOTHING
		RETURNING id
	`

	err := r.db.QueryRow(
		query,
		trade.OrderID,
		trade.Symbol,
		trade.Side,
		trade.Quantity,
		trade.Price,
		trade.TotalAmount,
		trade.Fees,
		trade.ExecutedAt,
		trade.PositionID,
	).Scan(&trade.ID)

	if err == sql.ErrNoRows {
		// Trade already exists
		return nil
	}
	return err
}

// GetTradeByOrderID retrieves a trade by order ID
func (r *Repository) GetTradeByOrderID(orderID string) (*models.Trade, error) {
	query := `
		SELECT id, order_id, symbol, side, quantity, price, total_amount, fees, executed_at, position_id, created_at
		FROM journal_trades
		WHERE order_id = $1
	`

	trade := &models.Trade{}
	err := r.db.QueryRow(query, orderID).Scan(
		&trade.ID,
		&trade.OrderID,
		&trade.Symbol,
		&trade.Side,
		&trade.Quantity,
		&trade.Price,
		&trade.TotalAmount,
		&trade.Fees,
		&trade.ExecutedAt,
		&trade.PositionID,
		&trade.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return trade, err
}

// CreatePosition creates a new open position from a buy trade
func (r *Repository) CreatePosition(trade *models.Trade) (*models.Position, error) {
	query := `
		INSERT INTO journal_positions (symbol, entry_order_id, entry_price, quantity, entry_date, status)
		VALUES ($1, $2, $3, $4, $5, 'open')
		RETURNING id, created_at
	`

	position := &models.Position{
		Symbol:       trade.Symbol,
		EntryOrderID: trade.OrderID,
		EntryPrice:   trade.Price,
		Quantity:     trade.Quantity,
		EntryDate:    trade.ExecutedAt,
		Status:       "open",
	}

	err := r.db.QueryRow(
		query,
		position.Symbol,
		position.EntryOrderID,
		position.EntryPrice,
		position.Quantity,
		position.EntryDate,
	).Scan(&position.ID, &position.CreatedAt)

	return position, err
}

// GetOpenPosition retrieves an open position for a symbol
func (r *Repository) GetOpenPosition(symbol string) (*models.Position, error) {
	query := `
		SELECT id, symbol, entry_order_id, exit_order_id, entry_price, exit_price,
		       quantity, entry_date, exit_date, realized_pl, realized_pl_pct, holding_days, status, created_at
		FROM journal_positions
		WHERE symbol = $1 AND status = 'open'
		ORDER BY entry_date DESC
		LIMIT 1
	`

	position := &models.Position{}
	err := r.db.QueryRow(query, symbol).Scan(
		&position.ID,
		&position.Symbol,
		&position.EntryOrderID,
		&position.ExitOrderID,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryDate,
		&position.ExitDate,
		&position.RealizedPL,
		&position.RealizedPLPct,
		&position.HoldingDays,
		&position.Status,
		&position.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return position, err
}

// ClosePosition closes a position with sell trade data
func (r *Repository) ClosePosition(positionID int64, exitTrade *models.Trade) error {
	// Get the position first for P&L calculation
	position, err := r.GetPositionByID(positionID)
	if err != nil {
		return err
	}

	// Calculate P&L
	realizedPL := (exitTrade.Price - position.EntryPrice) * exitTrade.Quantity
	realizedPLPct := ((exitTrade.Price - position.EntryPrice) / position.EntryPrice) * 100
	holdingDays := int(exitTrade.ExecutedAt.Sub(position.EntryDate).Hours() / 24)

	query := `
		UPDATE journal_positions
		SET exit_order_id = $1, exit_price = $2, exit_date = $3,
		    realized_pl = $4, realized_pl_pct = $5, holding_days = $6, status = 'closed'
		WHERE id = $7
	`

	_, err = r.db.Exec(
		query,
		exitTrade.OrderID,
		exitTrade.Price,
		exitTrade.ExecutedAt,
		realizedPL,
		realizedPLPct,
		holdingDays,
		positionID,
	)

	return err
}

// GetPositionByID retrieves a position by ID
func (r *Repository) GetPositionByID(id int64) (*models.Position, error) {
	query := `
		SELECT id, symbol, entry_order_id, exit_order_id, entry_price, exit_price,
		       quantity, entry_date, exit_date, realized_pl, realized_pl_pct, holding_days, status, created_at
		FROM journal_positions
		WHERE id = $1
	`

	position := &models.Position{}
	err := r.db.QueryRow(query, id).Scan(
		&position.ID,
		&position.Symbol,
		&position.EntryOrderID,
		&position.ExitOrderID,
		&position.EntryPrice,
		&position.ExitPrice,
		&position.Quantity,
		&position.EntryDate,
		&position.ExitDate,
		&position.RealizedPL,
		&position.RealizedPLPct,
		&position.HoldingDays,
		&position.Status,
		&position.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return position, err
}

// GetClosedPositionsWithoutJournal returns closed positions that don't have journal entries
func (r *Repository) GetClosedPositionsWithoutJournal() ([]*models.Position, error) {
	query := `
		SELECT p.id, p.symbol, p.entry_order_id, p.exit_order_id, p.entry_price, p.exit_price,
		       p.quantity, p.entry_date, p.exit_date, p.realized_pl, p.realized_pl_pct, p.holding_days, p.status, p.created_at
		FROM journal_positions p
		LEFT JOIN journal_entries j ON p.id = j.position_id
		WHERE p.status = 'closed' AND j.id IS NULL
		ORDER BY p.exit_date ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*models.Position
	for rows.Next() {
		position := &models.Position{}
		err := rows.Scan(
			&position.ID,
			&position.Symbol,
			&position.EntryOrderID,
			&position.ExitOrderID,
			&position.EntryPrice,
			&position.ExitPrice,
			&position.Quantity,
			&position.EntryDate,
			&position.ExitDate,
			&position.RealizedPL,
			&position.RealizedPLPct,
			&position.HoldingDays,
			&position.Status,
			&position.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}

	return positions, rows.Err()
}

// InsertJournalEntry inserts a new journal entry
func (r *Repository) InsertJournalEntry(entry *models.JournalEntry) error {
	query := `
		INSERT INTO journal_entries (
			position_id, symbol, entry_reasoning, exit_reasoning, what_worked,
			what_didnt_work, lessons_learned, emotional_state, would_repeat,
			rating, tags, notes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`

	return r.db.QueryRow(
		query,
		entry.PositionID,
		entry.Symbol,
		entry.EntryReasoning,
		entry.ExitReasoning,
		entry.WhatWorked,
		entry.WhatDidntWork,
		entry.LessonsLearned,
		entry.EmotionalState,
		entry.WouldRepeat,
		entry.Rating,
		pq.Array(entry.Tags),
		entry.Notes,
	).Scan(&entry.ID, &entry.CreatedAt, &entry.UpdatedAt)
}

// GetJournalEntryByPositionID retrieves a journal entry by position ID
func (r *Repository) GetJournalEntryByPositionID(positionID int64) (*models.JournalEntry, error) {
	query := `
		SELECT id, position_id, symbol, entry_reasoning, exit_reasoning, what_worked,
		       what_didnt_work, lessons_learned, emotional_state, would_repeat,
		       rating, tags, notes, created_at, updated_at
		FROM journal_entries
		WHERE position_id = $1
	`

	entry := &models.JournalEntry{}
	var tags pq.StringArray
	err := r.db.QueryRow(query, positionID).Scan(
		&entry.ID,
		&entry.PositionID,
		&entry.Symbol,
		&entry.EntryReasoning,
		&entry.ExitReasoning,
		&entry.WhatWorked,
		&entry.WhatDidntWork,
		&entry.LessonsLearned,
		&entry.EmotionalState,
		&entry.WouldRepeat,
		&entry.Rating,
		&tags,
		&entry.Notes,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	entry.Tags = []string(tags)
	return entry, err
}

// HasJournalEntry checks if a position has a journal entry
func (r *Repository) HasJournalEntry(positionID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM journal_entries WHERE position_id = $1)`
	var exists bool
	err := r.db.QueryRow(query, positionID).Scan(&exists)
	return exists, err
}

// GetRecentClosedPositions returns recently closed positions for summary
func (r *Repository) GetRecentClosedPositions(limit int) ([]*models.Position, error) {
	query := `
		SELECT id, symbol, entry_order_id, exit_order_id, entry_price, exit_price,
		       quantity, entry_date, exit_date, realized_pl, realized_pl_pct, holding_days, status, created_at
		FROM journal_positions
		WHERE status = 'closed'
		ORDER BY exit_date DESC
		LIMIT $1
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*models.Position
	for rows.Next() {
		position := &models.Position{}
		err := rows.Scan(
			&position.ID,
			&position.Symbol,
			&position.EntryOrderID,
			&position.ExitOrderID,
			&position.EntryPrice,
			&position.ExitPrice,
			&position.Quantity,
			&position.EntryDate,
			&position.ExitDate,
			&position.RealizedPL,
			&position.RealizedPLPct,
			&position.HoldingDays,
			&position.Status,
			&position.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}

	return positions, rows.Err()
}

// UpdateTradePositionID updates the position_id for a trade
func (r *Repository) UpdateTradePositionID(orderID string, positionID int64) error {
	query := `UPDATE journal_trades SET position_id = $1 WHERE order_id = $2`
	_, err := r.db.Exec(query, positionID, orderID)
	return err
}

// GetJournalStats returns statistics about journal entries
func (r *Repository) GetJournalStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total positions
	var totalPositions, closedPositions, journaledPositions int
	r.db.QueryRow(`SELECT COUNT(*) FROM journal_positions`).Scan(&totalPositions)
	r.db.QueryRow(`SELECT COUNT(*) FROM journal_positions WHERE status = 'closed'`).Scan(&closedPositions)
	r.db.QueryRow(`SELECT COUNT(*) FROM journal_entries`).Scan(&journaledPositions)

	// Win rate from journaled trades
	var wins, losses int
	r.db.QueryRow(`SELECT COUNT(*) FROM journal_positions WHERE status = 'closed' AND realized_pl > 0`).Scan(&wins)
	r.db.QueryRow(`SELECT COUNT(*) FROM journal_positions WHERE status = 'closed' AND realized_pl <= 0`).Scan(&losses)

	// Average rating
	var avgRating float64
	r.db.QueryRow(`SELECT COALESCE(AVG(rating), 0) FROM journal_entries`).Scan(&avgRating)

	// Total P&L
	var totalPL float64
	r.db.QueryRow(`SELECT COALESCE(SUM(realized_pl), 0) FROM journal_positions WHERE status = 'closed'`).Scan(&totalPL)

	stats["total_positions"] = totalPositions
	stats["closed_positions"] = closedPositions
	stats["journaled_positions"] = journaledPositions
	stats["pending_journals"] = closedPositions - journaledPositions
	stats["wins"] = wins
	stats["losses"] = losses
	stats["win_rate"] = 0.0
	if closedPositions > 0 {
		stats["win_rate"] = float64(wins) / float64(closedPositions) * 100
	}
	stats["avg_rating"] = avgRating
	stats["total_pl"] = totalPL

	return stats, nil
}
