package writer

import (
	"time"
)

// WriterConfig contains configuration for batch writers.
type WriterConfig struct {
	// BatchSize is the number of rows to accumulate before flushing.
	BatchSize int

	// FlushInterval is the maximum time between flushes.
	FlushInterval time.Duration
}

// DefaultWriterConfig returns sensible defaults.
func DefaultWriterConfig() WriterConfig {
	return WriterConfig{
		BatchSize:     1000,
		FlushInterval: 5 * time.Second,
	}
}

// tradeRow represents a row to be inserted into the trades table.
type tradeRow struct {
	TradeID    string // UUID
	ExchangeTs int64  // Microseconds
	ReceivedAt int64  // Microseconds
	Ticker     string
	Price      int // Hundred-thousandths (0-100,000)
	Size       int
	TakerSide  bool // TRUE = yes, FALSE = no
	SID        int64
}

// orderbookDeltaRow represents a row for the orderbook_deltas table.
type orderbookDeltaRow struct {
	ExchangeTs int64
	ReceivedAt int64
	Seq        int64
	Ticker     string
	Side       bool // TRUE = yes, FALSE = no
	Price      int  // Hundred-thousandths
	SizeDelta  int  // Positive = add, negative = remove
	SID        int64
}

// orderbookSnapshotRow represents a row for the orderbook_snapshots table.
type orderbookSnapshotRow struct {
	SnapshotTs int64
	ExchangeTs int64 // 0 for WS/REST snapshots
	Ticker     string
	Source     string // "ws" or "rest"
	YesBids    []byte // JSONB: [{price: int, size: int}, ...]
	YesAsks    []byte // JSONB: derived from NO bids
	NoBids     []byte // JSONB
	NoAsks     []byte // JSONB: derived from YES bids
	BestYesBid int
	BestYesAsk int
	Spread     int
	SID        int64
}

// tickerRow represents a row for the tickers table.
type tickerRow struct {
	ExchangeTs         int64
	ReceivedAt         int64
	Ticker             string
	YesBid             int // Hundred-thousandths
	YesAsk             int
	LastPrice          int
	Volume             int64
	OpenInterest       int64
	DollarVolume       int64
	DollarOpenInterest int64
	SID                int64
}

// WriterMetrics holds metrics for a writer.
type WriterMetrics struct {
	Inserts   int64
	Conflicts int64
	Errors    int64
	Flushes   int64
	SeqGaps   int64
}
