// Package integrity defines deterministic identities for immutable run intent.
package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"

	"github.com/srikarjy/RunBridge/internal/runs"
)

// SpecificationDigest hashes exact persisted bytes. Length-prefixed fields
// prevent concatenation ambiguity while keeping the format easy to reproduce
// in another service.
func SpecificationDigest(specification runs.Specification) [32]byte {
	var digest hash.Hash = sha256.New()
	writeField(digest, specification.Workflow().Name())
	writeField(digest, specification.Workflow().Revision())
	writeField(digest, specification.Configuration().SchemaVersion())
	document := specification.Configuration().Document()
	writeFieldBytes(digest, document)
	var result [32]byte
	copy(result[:], digest.Sum(nil))
	return result
}

func DigestHex(specification runs.Specification) string {
	digest := SpecificationDigest(specification)
	return hex.EncodeToString(digest[:])
}

func writeField(writer hash.Hash, value string) { writeFieldBytes(writer, []byte(value)) }
func writeFieldBytes(writer hash.Hash, value []byte) {
	var length [8]byte
	for index := uint(0); index < 8; index++ {
		length[7-index] = byte(len(value) >> (index * 8))
	}
	writer.Write(length[:])
	writer.Write(value)
}
