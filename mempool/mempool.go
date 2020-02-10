package mempool

import "time"

const (
	// DefaultBlockPrioritySize is the default size in bytes for high-
	// priority / low-fee transactions.  It is used to help determine which
	// are allowed into the mempool and consequently affects their relay and
	// inclusion when generating block templates.
	DefaultBlockPrioritySize = 50000

	// orphanTTL is the maximum amount of time an orphan is allowed to
	// stay in the orphan pool before it expires and is evicted during the
	// next scan.
	orphanTTL = time.Minute * 15

	// orphanExpireScanInterval is the minimum amount of time in between
	// scans of the orphan pool to evict expired transactions.
	orphanExpireScanInterval = time.Minute * 5

	// MaxRBFSequence is the maximum sequence number an input can use to
	// signal that the transaction spending it can be replaced using the
	// Replace-By-Fee (RBF) policy.
	MaxRBFSequence = 0xfffffffd

	// MaxReplacementEvictions is the maximum number of transactions that
	// can be evicted from the mempool when accepting a transaction
	// replacement.
	MaxReplacementEvictions = 100
)
