package authentication

// File implements metadata serialization.
// Metadata can include shared state as well as metadata sent explicitly over the wire. In order to
// authenticate metadata cryptographically, it must be encoded into []byte. The conversion must be
// injective: no two sets of metadata can result in the same []byte.

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
)

var (
	// errOutOfOrderMetadata indicates a programming error (as opposed to a run-time error) and
	// therefore is not exported.
	errOutOfOrderMetadata = errors.New("metadata items need to be added in increasing tag order")

	// ErrMetadataFieldTooLong indicates an authenticated field (such as a verifier name) is too
	// long to be compatible with the serialization format.
	ErrMetadataFieldTooLong = errors.New("metadata fields can't be more than 255 bytes long")
)

type metadata struct {
	Context hash.Hash
	fields  map[signatures.Tag]bool
	last    signatures.Tag
}

// Add a (tag, value) pair to the list of metadata values.
func (m *metadata) Add(tag signatures.Tag, value []byte) error {
	if tag < m.last {
		return errOutOfOrderMetadata
	}
	if value == nil {
		return nil
	}
	if len(value) > 255 {
		return ErrMetadataFieldTooLong
	}
	m.last = tag
	m.Context.Write([]byte{byte(tag)})
	m.Context.Write([]byte{byte(len(value))})
	m.Context.Write(value)
	m.fields[tag] = true
	return nil
}

func (m *metadata) AddUint32(tag signatures.Tag, value uint32) error {
	var buffer [4]byte
	binary.BigEndian.PutUint32(buffer[:], value)
	return m.Add(tag, buffer[:])
}

func newMetadata() *metadata {
	return newMetadataHash(sha256.New())
}

func newMetadataHash(context hash.Hash) *metadata {
	meta := metadata{
		Context: context,
		fields:  make(map[signatures.Tag]bool),
	}
	return &meta
}

// Contains returns true if every tag on the provided list has been added.
func (m *metadata) Contains(tags []signatures.Tag) bool {
	for _, tag := range tags {
		if _, ok := m.fields[tag]; !ok {
			return false
		}
	}
	return true
}

func (m *metadata) Checksum(message []byte) []byte {
	m.Context.Write([]byte{byte(signatures.Tag_TAG_END)})
	m.Context.Write(message)
	return m.Context.Sum(nil)
}
