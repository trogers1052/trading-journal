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

func main() {
	log.Println("Starting trading-journal service...")

	// Health endpoint — Docker/systemd HEALTHCHECK target
	healthPort := os.Getenv("HEALTH_PORT")
	if healthPort == "" {
		healthPort = "8080"
	}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck
	})
	go func() {
		if err := http.ListenAndServe(":"+healthPort, nil); err != nil {
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
	defer repo.Close()

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
	defer consumer.Close()

	// Set up trade handler
	consumer.SetTradeHandler(journalService.HandleTradeEvent)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Telegram bot
	if err := bot.Start(ctx); err != nil {
		log.Fatalf("Failed to start Telegram bot: %v", err)
	}
	defer bot.Stop()

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

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down trading-journal service...")
	catchupTimer.Stop()
	if pendingTimer != nil {
		pendingTimer.Stop()
	}
	cancel()

	log.Println("Trading journal service stopped")
}
