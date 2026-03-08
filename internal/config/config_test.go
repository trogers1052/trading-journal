package config

import (
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper: set required env vars for a valid Load()
// ---------------------------------------------------------------------------

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TELEGRAM_JOURNAL_BOT_TOKEN", "test-token-123")
	t.Setenv("TELEGRAM_JOURNAL_CHAT_ID", "999888777")
}

// ---------------------------------------------------------------------------
// Load — defaults
// ---------------------------------------------------------------------------

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Kafka defaults
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:19092" {
		t.Errorf("KafkaBrokers = %v, want [localhost:19092]", cfg.KafkaBrokers)
	}
	if cfg.KafkaConsumerGroup != "trading-journal" {
		t.Errorf("KafkaConsumerGroup = %q, want %q", cfg.KafkaConsumerGroup, "trading-journal")
	}
	if cfg.KafkaOrdersTopic != "trading.orders" {
		t.Errorf("KafkaOrdersTopic = %q, want %q", cfg.KafkaOrdersTopic, "trading.orders")
	}

	// DB defaults
	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost = %q, want %q", cfg.DBHost, "localhost")
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort = %d, want %d", cfg.DBPort, 5432)
	}
	if cfg.DBUser != "trader" {
		t.Errorf("DBUser = %q, want %q", cfg.DBUser, "trader")
	}
	if cfg.DBPassword != "REDACTED_PASSWORD" {
		t.Errorf("DBPassword = %q, want %q", cfg.DBPassword, "REDACTED_PASSWORD")
	}
	if cfg.DBName != "trading_platform" {
		t.Errorf("DBName = %q, want %q", cfg.DBName, "trading_platform")
	}

	// Redis defaults
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q, want %q", cfg.RedisAddr, "localhost:6379")
	}
	if cfg.RedisPassword != "" {
		t.Errorf("RedisPassword = %q, want empty", cfg.RedisPassword)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("RedisDB = %d, want 0", cfg.RedisDB)
	}

	// Journal defaults
	if cfg.JournalPromptTimeout != 60 {
		t.Errorf("JournalPromptTimeout = %d, want 60", cfg.JournalPromptTimeout)
	}
}

// ---------------------------------------------------------------------------
// Load — required field validation
// ---------------------------------------------------------------------------

func TestLoad_MissingTelegramBotToken(t *testing.T) {
	// Only set chat ID, not token
	t.Setenv("TELEGRAM_JOURNAL_CHAT_ID", "12345")
	// Ensure token is unset
	os.Unsetenv("TELEGRAM_JOURNAL_BOT_TOKEN")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_JOURNAL_BOT_TOKEN is missing")
	}
	if !strings.Contains(err.Error(), "TELEGRAM_JOURNAL_BOT_TOKEN") {
		t.Fatalf("error should mention TELEGRAM_JOURNAL_BOT_TOKEN: %v", err)
	}
}

func TestLoad_MissingTelegramChatID(t *testing.T) {
	t.Setenv("TELEGRAM_JOURNAL_BOT_TOKEN", "test-token")
	// Ensure chat ID is unset (default is 0 which triggers validation)
	os.Unsetenv("TELEGRAM_JOURNAL_CHAT_ID")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_JOURNAL_CHAT_ID is missing")
	}
	if !strings.Contains(err.Error(), "TELEGRAM_JOURNAL_CHAT_ID") {
		t.Fatalf("error should mention TELEGRAM_JOURNAL_CHAT_ID: %v", err)
	}
}

func TestLoad_ZeroChatID(t *testing.T) {
	t.Setenv("TELEGRAM_JOURNAL_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_JOURNAL_CHAT_ID", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TELEGRAM_JOURNAL_CHAT_ID is 0")
	}
}

// ---------------------------------------------------------------------------
// Load — custom env vars
// ---------------------------------------------------------------------------

func TestLoad_CustomKafkaBrokers(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("KAFKA_BROKERS", "broker1:9092,broker2:9092,broker3:9092")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.KafkaBrokers) != 3 {
		t.Fatalf("KafkaBrokers length = %d, want 3", len(cfg.KafkaBrokers))
	}
	if cfg.KafkaBrokers[0] != "broker1:9092" {
		t.Errorf("KafkaBrokers[0] = %q, want %q", cfg.KafkaBrokers[0], "broker1:9092")
	}
}

func TestLoad_CustomDBPort(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DB_PORT", "5433")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.DBPort != 5433 {
		t.Errorf("DBPort = %d, want 5433", cfg.DBPort)
	}
}

func TestLoad_CustomRedisDB(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("REDIS_DB", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.RedisDB != 3 {
		t.Errorf("RedisDB = %d, want 3", cfg.RedisDB)
	}
}

func TestLoad_CustomJournalTimeout(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JOURNAL_PROMPT_TIMEOUT", "120")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.JournalPromptTimeout != 120 {
		t.Errorf("JournalPromptTimeout = %d, want 120", cfg.JournalPromptTimeout)
	}
}

func TestLoad_TelegramValues(t *testing.T) {
	t.Setenv("TELEGRAM_JOURNAL_BOT_TOKEN", "my-secret-token")
	t.Setenv("TELEGRAM_JOURNAL_CHAT_ID", "123456789")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.TelegramBotToken != "my-secret-token" {
		t.Errorf("TelegramBotToken = %q, want %q", cfg.TelegramBotToken, "my-secret-token")
	}
	if cfg.TelegramChatID != 123456789 {
		t.Errorf("TelegramChatID = %d, want 123456789", cfg.TelegramChatID)
	}
}

// ---------------------------------------------------------------------------
// DSN
// ---------------------------------------------------------------------------

func TestDSN_DefaultValues(t *testing.T) {
	cfg := &Config{
		DBHost:     "localhost",
		DBPort:     5432,
		DBUser:     "trader",
		DBPassword: "REDACTED_PASSWORD",
		DBName:     "trading_platform",
	}

	dsn := cfg.DSN()
	expected := "host=localhost port=5432 user=trader password=REDACTED_PASSWORD dbname=trading_platform sslmode=disable"
	if dsn != expected {
		t.Errorf("DSN() = %q, want %q", dsn, expected)
	}
}

func TestDSN_CustomValues(t *testing.T) {
	cfg := &Config{
		DBHost:     "db.example.com",
		DBPort:     5433,
		DBUser:     "admin",
		DBPassword: "s3cret!",
		DBName:     "mydb",
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "host=db.example.com") {
		t.Errorf("DSN should contain custom host: %q", dsn)
	}
	if !strings.Contains(dsn, "port=5433") {
		t.Errorf("DSN should contain custom port: %q", dsn)
	}
	if !strings.Contains(dsn, "user=admin") {
		t.Errorf("DSN should contain custom user: %q", dsn)
	}
	if !strings.Contains(dsn, "password=s3cret!") {
		t.Errorf("DSN should contain custom password: %q", dsn)
	}
	if !strings.Contains(dsn, "dbname=mydb") {
		t.Errorf("DSN should contain custom dbname: %q", dsn)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("DSN should contain sslmode=disable: %q", dsn)
	}
}

// ---------------------------------------------------------------------------
// getEnv
// ---------------------------------------------------------------------------

func TestGetEnv_WithValue(t *testing.T) {
	t.Setenv("TEST_GET_ENV_KEY", "my_value")
	result := getEnv("TEST_GET_ENV_KEY", "default")
	if result != "my_value" {
		t.Errorf("getEnv with set var = %q, want %q", result, "my_value")
	}
}

func TestGetEnv_WithDefault(t *testing.T) {
	os.Unsetenv("TEST_GET_ENV_UNSET_KEY")
	result := getEnv("TEST_GET_ENV_UNSET_KEY", "fallback")
	if result != "fallback" {
		t.Errorf("getEnv with unset var = %q, want %q", result, "fallback")
	}
}

func TestGetEnv_EmptyStringUsesDefault(t *testing.T) {
	t.Setenv("TEST_GET_ENV_EMPTY", "")
	result := getEnv("TEST_GET_ENV_EMPTY", "default")
	// Empty string means os.Getenv returns "", so default is used
	if result != "default" {
		t.Errorf("getEnv with empty var = %q, want %q", result, "default")
	}
}

// ---------------------------------------------------------------------------
// getEnvInt
// ---------------------------------------------------------------------------

func TestGetEnvInt_WithValue(t *testing.T) {
	t.Setenv("TEST_INT_KEY", "42")
	result := getEnvInt("TEST_INT_KEY", 0)
	if result != 42 {
		t.Errorf("getEnvInt with set var = %d, want 42", result)
	}
}

func TestGetEnvInt_WithDefault(t *testing.T) {
	os.Unsetenv("TEST_INT_UNSET")
	result := getEnvInt("TEST_INT_UNSET", 99)
	if result != 99 {
		t.Errorf("getEnvInt with unset var = %d, want 99", result)
	}
}

func TestGetEnvInt_InvalidValue(t *testing.T) {
	t.Setenv("TEST_INT_INVALID", "not_a_number")
	result := getEnvInt("TEST_INT_INVALID", 50)
	if result != 50 {
		t.Errorf("getEnvInt with invalid var = %d, want default 50", result)
	}
}

func TestGetEnvInt_NegativeValue(t *testing.T) {
	t.Setenv("TEST_INT_NEG", "-5")
	result := getEnvInt("TEST_INT_NEG", 0)
	if result != -5 {
		t.Errorf("getEnvInt with negative = %d, want -5", result)
	}
}

// ---------------------------------------------------------------------------
// getEnvInt64
// ---------------------------------------------------------------------------

func TestGetEnvInt64_WithValue(t *testing.T) {
	t.Setenv("TEST_INT64_KEY", "9999999999")
	result := getEnvInt64("TEST_INT64_KEY", 0)
	if result != 9999999999 {
		t.Errorf("getEnvInt64 = %d, want 9999999999", result)
	}
}

func TestGetEnvInt64_WithDefault(t *testing.T) {
	os.Unsetenv("TEST_INT64_UNSET")
	result := getEnvInt64("TEST_INT64_UNSET", 12345)
	if result != 12345 {
		t.Errorf("getEnvInt64 with unset var = %d, want 12345", result)
	}
}

func TestGetEnvInt64_InvalidValue(t *testing.T) {
	t.Setenv("TEST_INT64_INVALID", "not_int64")
	result := getEnvInt64("TEST_INT64_INVALID", 77)
	if result != 77 {
		t.Errorf("getEnvInt64 with invalid var = %d, want default 77", result)
	}
}

func TestGetEnvInt64_NegativeValue(t *testing.T) {
	t.Setenv("TEST_INT64_NEG", "-123456789")
	result := getEnvInt64("TEST_INT64_NEG", 0)
	if result != -123456789 {
		t.Errorf("getEnvInt64 with negative = %d, want -123456789", result)
	}
}

func TestGetEnvInt64_FloatFallsBackToDefault(t *testing.T) {
	t.Setenv("TEST_INT64_FLOAT", "3.14")
	result := getEnvInt64("TEST_INT64_FLOAT", 100)
	// ParseInt will fail on float strings
	if result != 100 {
		t.Errorf("getEnvInt64 with float = %d, want default 100", result)
	}
}
