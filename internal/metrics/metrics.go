package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// TradesConsumed counts trade events consumed from Kafka, labeled by side (buy/sell).
var TradesConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "journal_trades_consumed_total",
	Help: "Trade events consumed from Kafka by side.",
}, []string{"side"})

// PositionsClosed counts positions that have been matched and closed.
var PositionsClosed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "journal_positions_closed_total",
	Help: "Positions matched and closed.",
})

// EntriesCreated counts journal entries written to the database.
var EntriesCreated = promauto.NewCounter(prometheus.CounterOpts{
	Name: "journal_entries_created_total",
	Help: "Journal entries written to DB.",
})

// KafkaConsumed counts total Kafka messages consumed (regardless of event type).
var KafkaConsumed = promauto.NewCounter(prometheus.CounterOpts{
	Name: "journal_kafka_consumed_total",
	Help: "Total Kafka messages consumed.",
})

// DBWriteDuration observes PostgreSQL write latency in seconds.
var DBWriteDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "journal_db_write_duration_seconds",
	Help:    "PostgreSQL write latency.",
	Buckets: prometheus.DefBuckets,
})

// DBWriteErrors counts database write failures.
var DBWriteErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: "journal_db_write_errors_total",
	Help: "DB write failures.",
})

// RedisSnapshotErrors counts risk metrics snapshot failures from Redis.
var RedisSnapshotErrors = promauto.NewCounter(prometheus.CounterOpts{
	Name: "journal_redis_snapshot_errors_total",
	Help: "Risk metrics snapshot failures.",
})

// TelegramPrompts counts Telegram journal prompts sent, labeled by status (success/failed).
var TelegramPrompts = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "journal_telegram_prompts_sent_total",
	Help: "Telegram journal prompts sent by status.",
}, []string{"status"})

// PendingEntries tracks the number of entries waiting for user journal response.
var PendingEntries = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "journal_pending_entries",
	Help: "Entries waiting for user journal response.",
})

// CatchupMode indicates whether the service is in Kafka catchup mode (1) or live (0).
var CatchupMode = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "journal_catchup_mode",
	Help: "1 if in catchup mode, 0 if live.",
})
