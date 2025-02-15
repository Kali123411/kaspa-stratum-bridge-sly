package slyvexstratum

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"log"
	"math/big"

	"github.com/slyvex/slyvexd/app/appmessage"
	"golang.org/x/crypto/blake2b"
)

// Static value definitions to avoid overhead in diff translations
var (
	maxTarget = big.NewFloat(0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
	minHash   = new(big.Float).Quo(new(big.Float).SetMantExp(big.NewFloat(1), 256), maxTarget)
	bigGig    = big.NewFloat(1e9)
)

// Represents different ways of computing difficulty
// Updated for Slyvex

type slyvexDiff struct {
	hashValue   float64
	diffValue   float64
	targetValue *big.Int
}

func newSlyvexDiff() *slyvexDiff {
	return &slyvexDiff{}
}

func (s *slyvexDiff) setDiffValue(diff float64) {
	s.diffValue = diff
	s.targetValue = DiffToTarget(diff)
	s.hashValue = DiffToHash(diff)
}

func DiffToTarget(diff float64) *big.Int {
	target := new(big.Float).Quo(maxTarget, big.NewFloat(diff))
	t, _ := target.Int(nil)
	return t
}

func DiffToHash(diff float64) float64 {
	hashVal := new(big.Float).Mul(minHash, big.NewFloat(diff))
	hashVal.Quo(hashVal, bigGig)

	h, _ := hashVal.Float64()
	return h
}

func SerializeBlockHeader(template *appmessage.RPCBlock) ([]byte, error) {
	hasher, err := blake2b.New(32, []byte("BlockHash"))
	if err != nil {
		return nil, err
	}

	write16(hasher, uint16(template.Header.Version))
	write64(hasher, uint64(len(template.Header.Parents)))
	for _, v := range template.Header.Parents {
		write64(hasher, uint64(len(v.ParentHashes)))
		for _, hash := range v.ParentHashes {
			writeHexString(hasher, hash)
		}
	}

	writeHexString(hasher, template.Header.HashMerkleRoot)
	writeHexString(hasher, template.Header.AcceptedIDMerkleRoot)
	writeHexString(hasher, template.Header.UTXOCommitment)

	data := struct {
		TS        uint64
		Bits      uint32
		Nonce     uint64
		DAAScore  uint64
		BlueScore uint64
	}{
		TS:        uint64(0),
		Bits:      uint32(template.Header.Bits),
		Nonce:     uint64(0),
		DAAScore:  uint64(template.Header.DAAScore),
		BlueScore: uint64(template.Header.BlueScore),
	}
	detailsBuff := &bytes.Buffer{}
	if err := binary.Write(detailsBuff, binary.LittleEndian, data); err != nil {
		return nil, err
	}
	hasher.Write(detailsBuff.Bytes())

	bw := template.Header.BlueWork
	padding := len(bw) + (len(bw) % 2)
	for len(bw) < padding {
		bw = "0" + bw
	}
	hh, _ := hex.DecodeString(bw)
	write64(hasher, uint64(len(hh)))
	writeHexString(hasher, bw)
	writeHexString(hasher, template.Header.PruningPoint)

	final := hasher.Sum(nil)
	return final, nil
}

var bi = big.NewInt(16777215)

func CalculateTarget(bits uint64) big.Int {
	truncated := uint64(bits) >> 24
	mantissa := bits & bi.Uint64()
	exponent := uint64(0)
	if truncated <= 3 {
		mantissa = mantissa >> (8 * (3 - truncated))
	} else {
		exponent = 8 * ((bits >> 24) - 3)
	}
	diff := big.Int{}
	diff.SetUint64(mantissa)
	diff.Lsh(&diff, uint(exponent))

	return diff
}

func write16(hasher hash.Hash, val uint16) {
	intBuff := make([]byte, 2)
	binary.LittleEndian.PutUint16(intBuff, val)
	hasher.Write(intBuff)
}

func write64(hasher hash.Hash, val uint64) {
	intBuff := make([]byte, 8)
	binary.LittleEndian.PutUint64(intBuff, val)
	hasher.Write(intBuff)
}

func writeHexString(hasher hash.Hash, val string) {
	hexBw, _ := hex.DecodeString(val)
	hasher.Write([]byte(hexBw))
}
