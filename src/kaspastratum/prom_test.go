package slyvexstratum

import (
	"testing"

	"github.com/slyvexnetwork/slyvexd/app/appmessage"
	"github.com/onemorebsmith/slyvexstratum/src/gostratum"
	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/Kali123411/kaspa-stratum-bridge-sly/src/gostratum"
)

func TestPromValid(t *testing.T) {
	// Mismatched Prometheus labels throw a panic, sanity check that everything
	// is valid to write to here.
	ctx := gostratum.StratumContext{}

	RecordShareFound(&ctx, 1000.1001)
	RecordStaleShare(&ctx)
	RecordDupeShare(&ctx)
	RecordInvalidShare(&ctx)
	RecordWeakShare(&ctx)
	RecordBlockFound(&ctx, 10000, 12345, "abcdefg")
	RecordDisconnect(&ctx)
	RecordNewJob(&ctx)
	RecordNetworkStats(1234, 5678, 910)
	RecordWorkerError("localhost", ErrDisconnected)
	RecordBalances(&appmessage.GetBalancesByAddressesResponseMessage{
		Entries: []*appmessage.BalancesByAddressesEntry{
			{
				Address: "localhost",
				Balance: 1234,
			},
		},
	})
}
