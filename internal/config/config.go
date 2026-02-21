package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the trading journal service
type Config struct {
	// Kafka
	KafkaBrokers       []string
	KafkaConsumerGroup string
	KafkaOrdersTopic   string // trading.orders from robinhood-sync

	// Database (PostgreSQL)
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Telegram Journal Bot (separate bot from alerts)
	TelegramBotToken string
	TelegramChatID   int64

	// Journal settings
	JournalPromptTimeout int // Minutes to wait for journal entry before reminder
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Kafka
		KafkaBrokers:       strings.Split(getEnv("KAFKA_BROKERS", "localhost:19092"), ","),
		KafkaConsumerGroup: getEnv("KAFKA_CONSUMER_GROUP", "trading-journal"),
		KafkaOrdersTopic:   getEnv("KAFKA_ORDERS_TOPIC", "trading.orders"),

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "trader"),
		DBPassword: getEnv("DB_PASSWORD", "REDACTED_PASSWORD"),
		DBName:     getEnv("DB_NAME", "trading_platform"),

		// Telegram
		TelegramBotToken: getEnv("TELEGRAM_JOURNAL_BOT_TOKEN", ""),
		TelegramChatID:   getEnvInt64("TELEGRAM_JOURNAL_CHAT_ID", 0),

		// Journal settings
		JournalPromptTimeout: getEnvInt("JOURNAL_PROMPT_TIMEOUT", 60),
	}

	// Validate required fields
	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_JOURNAL_BOT_TOKEN is required")
	}
	if cfg.TelegramChatID == 0 {
		return nil, fmt.Errorf("TELEGRAM_JOURNAL_CHAT_ID is required")
	}

	return cfg, nil
}

// DSN returns the PostgreSQL connection string
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=prefer",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}
