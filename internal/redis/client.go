package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/trogers1052/trading-journal/internal/metrics"
)

// Client reads market context and indicator data from Redis for risk metric snapshots.
type Client struct {
	rdb *goredis.Client
}

// NewClient creates a Redis client. Returns nil if addr is empty.
func NewClient(addr, password string, db int) *Client {
	if addr == "" {
		return nil
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		DialTimeout:  3 * time.Second,
	})
	return &Client{rdb: rdb}
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close releases the underlying connection pool.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// SnapshotRiskMetrics reads market context and per-symbol indicators from Redis
// and returns a combined JSON blob suitable for storing in risk_metrics_at_entry.
// Returns nil (no error) when data is unavailable — callers must not block
// trade recording on this.
func (c *Client) SnapshotRiskMetrics(ctx context.Context, symbol string) []byte {
	snapshot := make(map[string]any)

	// 1. Market context (regime, VIX, macro signals) — written by context-service
	if raw, err := c.rdb.Get(ctx, "market:context").Result(); err == nil {
		var mc map[string]any
		if json.Unmarshal([]byte(raw), &mc) == nil {
			if v, ok := mc["regime"]; ok {
				snapshot["regime"] = v
			}
			if v, ok := mc["regime_confidence"]; ok {
				snapshot["regime_confidence"] = v
			}
			// Macro signals (VIX, HY spread, etc.)
			if macro, ok := mc["macro_signals"].(map[string]any); ok {
				for k, v := range macro {
					snapshot[k] = v
				}
			}
		}
	} else {
		metrics.RedisSnapshotErrors.Inc()
		log.Printf("risk_metrics: market:context unavailable: %v", err)
	}

	// 2. Per-symbol indicators — written by analytics-service to indicators:{symbol}
	indicatorKey := fmt.Sprintf("indicators:%s", symbol)
	if raw, err := c.rdb.Get(ctx, indicatorKey).Result(); err == nil {
		var ind map[string]any
		if json.Unmarshal([]byte(raw), &ind) == nil {
			// Cherry-pick the most useful indicators for trade analysis
			for _, key := range []string{
				"RSI_14", "ATR_14", "SMA_200", "SMA_50", "SMA_20",
				"EMA_9", "EMA_21", "MACD", "MACD_SIGNAL",
				"BB_UPPER", "BB_LOWER", "BB_MIDDLE",
				"STOCH_K", "STOCH_D", "ADX_14",
			} {
				if v, ok := ind[key]; ok {
					snapshot[key] = v
				}
			}
		}
	} else {
		metrics.RedisSnapshotErrors.Inc()
		log.Printf("risk_metrics: %s unavailable: %v", indicatorKey, err)
	}

	if len(snapshot) == 0 {
		return nil
	}

	snapshot["snapshot_time"] = time.Now().UTC().Format(time.RFC3339)

	data, err := json.Marshal(snapshot)
	if err != nil {
		metrics.RedisSnapshotErrors.Inc()
		log.Printf("risk_metrics: failed to marshal snapshot: %v", err)
		return nil
	}
	return data
}
