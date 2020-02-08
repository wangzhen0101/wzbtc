package peer

import (
	"github.com/wangzhen0101/wzbtc/wire"
	"time"
)

const (
	// MaxProtocolVersion is the max protocol version the peer supports.
	MaxProtocolVersion = wire.FeeFilterVersion

	// DefaultTrickleInterval is the min time between attempts to send an
	// inv message to a peer.
	DefaultTrickleInterval = 10 * time.Second

	// MinAcceptableProtocolVersion is the lowest protocol version that a
	// connected peer may support.
	MinAcceptableProtocolVersion = wire.MultipleAddressVersion

	// outputBufferSize is the number of elements the output channels use.
	outputBufferSize = 50

	// invTrickleSize is the maximum amount of inventory to send in a single
	// message when trickling inventory to remote peers.
	maxInvTrickleSize = 1000

	// maxKnownInventory is the maximum number of items to keep in the known
	// inventory cache.
	maxKnownInventory = 1000

	// pingInterval is the interval of time to wait in between sending ping
	// messages.
	pingInterval = 2 * time.Minute

	// negotiateTimeout is the duration of inactivity before we timeout a
	// peer that hasn't completed the initial version negotiation.
	negotiateTimeout = 30 * time.Second

	// idleTimeout is the duration of inactivity before we time out a peer.
	idleTimeout = 5 * time.Minute

	// stallTickInterval is the interval of time between each check for
	// stalled peers.
	stallTickInterval = 15 * time.Second

	// stallResponseTimeout is the base maximum amount of time messages that
	// expect a response will wait before disconnecting the peer for
	// stalling.  The deadlines are adjusted for callback running times and
	// only checked on each stall tick interval.
	stallResponseTimeout = 30 * time.Second
)
