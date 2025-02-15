package kaspastratum

import "math/big"

// SerializeSlyvexBlockHeader serializes the block header for submission
func SerializeSlyvexBlockHeader(header *big.Int) []byte {
	return header.Bytes()
}
