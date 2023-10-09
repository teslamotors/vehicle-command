package authentication

import (
	"bytes"
	"crypto/sha512"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
)

type metadataItem struct {
	Tag   signatures.Tag
	Value []byte
}

func addItems(t *testing.T, meta *metadata, items []metadataItem) {
	for _, item := range items {
		if err := meta.Add(item.Tag, item.Value); err != nil {
			t.Fatalf("Error adding item %+v: %s", item, err)
		}
	}
}

func TestOutOfOrder(t *testing.T) {
	meta := newMetadata()
	if err := meta.Add(signatures.Tag_TAG_DOMAIN, []byte("hello")); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if err := meta.Add(signatures.Tag_TAG_PERSONALIZATION, []byte("world")); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if err := meta.Add(signatures.Tag_TAG_DOMAIN, []byte("world")); err != errOutOfOrderMetadata {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestValueTooLong(t *testing.T) {
	meta := newMetadata()
	value := make([]byte, 256)
	if err := meta.Add(signatures.Tag_TAG_DOMAIN, value); err != ErrMetadataFieldTooLong {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestCheckSum(t *testing.T) {
	items := []metadataItem{
		{signatures.Tag_TAG_SIGNATURE_TYPE, []byte{0x05}},
		{signatures.Tag_TAG_DOMAIN, []byte{0x02}},
		{signatures.Tag_TAG_PERSONALIZATION, []byte("testVIN")},
		{signatures.Tag_TAG_EPOCH, []byte{0xaa, 0xda, 0x92, 0x8a, 0x4f, 0x21, 0x5f, 0x55, 0xf9, 0xe6, 0xe4, 0x5e, 0x66, 0xb6, 0x52, 0x1e}},
		{signatures.Tag_TAG_EXPIRES_AT, []byte{0x00, 0x00, 0x0e, 0x74}},
		{signatures.Tag_TAG_COUNTER, []byte{0x00, 0x00, 0x05, 0x3a}},
	}
	correct := []byte{
		0xab, 0xab, 0x04, 0xd8, 0x04, 0x49, 0x98, 0x13, 0x38, 0x2e, 0xfd, 0x74,
		0xa0, 0x67, 0x91, 0xce, 0x2d, 0xe7, 0x77, 0x43, 0x96, 0x03, 0x24, 0x6d,
		0xfb, 0xaa, 0x83, 0x92, 0xca, 0x05, 0x86, 0x8e,
	}
	meta := newMetadata()
	addItems(t, meta, items)
	computed := meta.Checksum(nil)
	if !bytes.Equal(computed, correct) {
		t.Errorf("Computed incorrect checksum %x", computed)
	}
}

func TestHash512CheckSum(t *testing.T) {
	items := []metadataItem{
		{signatures.Tag_TAG_SIGNATURE_TYPE, []byte{0x05}},
		{signatures.Tag_TAG_DOMAIN, []byte{0x02}},
		{signatures.Tag_TAG_PERSONALIZATION, []byte("testVIN")},
		{signatures.Tag_TAG_EPOCH, []byte{0xaa, 0xda, 0x92, 0x8a, 0x4f, 0x21, 0x5f, 0x55, 0xf9, 0xe6, 0xe4, 0x5e, 0x66, 0xb6, 0x52, 0x1e}},
		{signatures.Tag_TAG_EXPIRES_AT, []byte{0x00, 0x00, 0x0e, 0x74}},
		{signatures.Tag_TAG_COUNTER, []byte{0x00, 0x00, 0x05, 0x3a}},
	}
	correct := []byte{
		0xdf, 0x4a, 0x60, 0xe0, 0x3f, 0xd4, 0xf7, 0x1a, 0x83, 0xe6, 0xb5, 0x6c,
		0xcf, 0x27, 0xcc, 0xf3, 0x90, 0x26, 0x9b, 0xa3, 0xfc, 0xcf, 0xaf, 0xd9,
		0xcb, 0x3a, 0x09, 0x25, 0xfc, 0x36, 0x84, 0x38, 0x66, 0xb4, 0x32, 0x66,
		0x55, 0xf1, 0xc9, 0xd5, 0x39, 0xc7, 0xff, 0xc6, 0xf3, 0x31, 0xba, 0x69,
		0x3e, 0x1c, 0x62, 0xd2, 0x37, 0xcb, 0x6c, 0xb5, 0xd9, 0xe6, 0x04, 0x39,
		0xf9, 0x8f, 0x22, 0x83,
	}
	meta := newMetadataHash(sha512.New())
	addItems(t, meta, items)
	computed := meta.Checksum(nil)

	if !bytes.Equal(computed, correct) {
		t.Errorf("Computed incorrect checksum %x", computed)
	}
}
