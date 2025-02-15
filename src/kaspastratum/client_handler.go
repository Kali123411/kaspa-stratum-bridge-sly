package kaspastratum

import (
	"math/big"

	"github.com/slyvex-core/slyvexd/app/appmessage"
)

// HandleClientSubmission processes miner share submissions
func HandleClientSubmission(template *appmessage.RPCBlock) []byte {
	return SerializeSlyvexBlockHeader(new(big.Int).SetUint64(uint64(template.Header.Timestamp)))
}
