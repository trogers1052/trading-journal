package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trogers1052/trading-journal/internal/config"
	"github.com/trogers1052/trading-journal/internal/database"
	"github.com/trogers1052/trading-journal/internal/kafka"
	"github.com/trogers1052/trading-journal/internal/service"
	"github.com/trogers1052/trading-journal/internal/telegram"
)

// shutdownTimeout is the maximum time allowed for graceful shutdown before
// the process exits forcefully. This prevents the service from hanging
// indefinitely if a resource (Kafka, DB, etc.) fails to close.
const shutdownTimeout = 15 * time.Second

func main() {
	log.Println("Starting trading-journal service...")

	// Health endpoint — Docker/systemd HEALTHCHECK target
	healthPort := os.Getenv("HEALTH_PORT")
	if healthPort == "" {
		healthPort = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	})
	healthServer := &http.Server{
		Addr:    ":" + healthPort,
		Handler: mux,
	}
	go func() {
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()
	log.Printf("Health endpoint: http://localhost:%s/health", healthPort)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Kafka brokers: %v", cfg.KafkaBrokers)
	log.Printf("  Orders topic: %s", cfg.KafkaOrdersTopic)
	log.Printf("  Database: %s@%s/%s", cfg.DBUser, cfg.DBHost, cfg.DBName)

	// Connect to database
	repo, err := database.NewRepository(cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Create Telegram bot
	bot, err := telegram.NewBot(cfg.TelegramBotToken, cfg.TelegramChatID)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// Create journal service
	journalService := service.NewJournalService(repo, bot)

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaConsumerGroup,
		cfg.KafkaOrdersTopic,
	)
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}

	// Set up trade handler
	consumer.SetTradeHandler(journalService.HandleTradeEvent)

	// Create context with cancellation — all long-running goroutines use this
	ctx, cancel := context.WithCancel(context.Background())

	// Start Telegram bot
	if err := bot.Start(ctx); err != nil {
		log.Fatalf("Failed to start Telegram bot: %v", err)
	}

	// Start Kafka consumer
	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start Kafka consumer: %v", err)
	}

	log.Println("Trading journal service running...")

	// Wait for Kafka to catch up with historical trades, then disable catchup mode
	// and start prompting for pending journal entries one at a time
	var pendingTimer *time.Timer
	catchupTimer := time.AfterFunc(10*time.Second, func() {
		log.Println("Kafka catchup period complete, switching to live mode...")
		journalService.SetCatchupMode(false)

		// Get stats and send startup notification
		stats, err := journalService.GetStats()
		if err != nil {
			log.Printf("Warning: failed to get stats: %v", err)
			stats = map[string]interface{}{
				"total_positions":     0,
				"closed_positions":    0,
				"journaled_positions": 0,
				"pending_journals":    0,
				"wins":                0,
				"losses":              0,
				"win_rate":            0.0,
				"total_pl":            0.0,
				"avg_rating":          0.0,
			}
		}
		bot.SendStats(stats)

		// Now check for pending journal entries and prompt one at a time
		pendingTimer = time.AfterFunc(2*time.Second, func() {
			log.Println("Checking for pending journal entries...")
			if err := journalService.CatchUpPendingJournals(); err != nil {
				log.Printf("Warning: failed to catch up pending journals: %v", err)
			}
		})
	})

	// ---- Wait for OS shutdown signal (SIGINT / SIGTERM) ----
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("Received signal %v — initiating graceful shutdown (timeout %s)...", sig, shutdownTimeout)

	// Stop accepting new work immediately
	catchupTimer.Stop()
	if pendingTimer != nil {
		pendingTimer.Stop()
	}

	// Cancel the root context so all goroutines (Kafka consumer, Telegram bot,
	// timers) begin draining.
	cancel()

	// Perform resource cleanup under a deadline so a hung resource cannot
	// block the process from exiting.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Channel to signal that cleanup finished before the deadline.
	done := make(chan struct{})
	go func() {
		defer close(done)

		// 1. Shut down the health HTTP server so readiness probes start failing.
		log.Println("Shutting down health server...")
		if err := healthServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Warning: health server shutdown error: %v", err)
		}

		// 2. Close Kafka consumer (waits for in-flight message processing).
		log.Println("Closing Kafka consumer...")
		if err := consumer.Close(); err != nil {
			log.Printf("Warning: Kafka consumer close error: %v", err)
		}

		// 3. Stop Telegram bot.
		log.Println("Stopping Telegram bot...")
		bot.Stop()

		// 4. Close database connection pool last (other components may still
		//    flush writes during their own shutdown).
		log.Println("Closing database connection...")
		if err := repo.Close(); err != nil {
			log.Printf("Warning: database close error: %v", err)
		}
	}()

	select {
	case <-done:
		log.Println("Trading journal service stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("WARNING: Graceful shutdown timed out — exiting forcefully")
	}
}
