package bdb

import (
	"bytes"
	"encoding/binary"

	"golang.org/x/xerrors"
)

// source: https://github.com/berkeleydb/libdb/blob/5b7b02ae052442626af54c176335b67ecc613a30/src/dbinc/db_page.h#L130
type HashMetadata struct {
	GenericMetadataPage
	MaxBucket   uint32 `struct:"uint32"` /* 72-75: ID of Maximum bucket in use */
	HighMask    uint32 `struct:"uint32"` /* 76-79: Modulo mask into table */
	LowMask     uint32 `struct:"uint32"` /* 80-83: Modulo mask into table lower half */
	FillFactor  uint32 `struct:"uint32"` /* 84-87: Fill factor */
	NumKeys     uint32 `struct:"uint32"` /* 88-91: Number of keys in hash table */
	CharKeyHash uint32 `struct:"uint32"` /* 92-95: Value of hash(CHARKEY) */
	// don't care about the rest...
}

type HashMetadataPage struct {
	HashMetadata
	Swapped bool
}

func ParseHashMetadataPage(data []byte) (*HashMetadataPage, error) {
	var pageMetadata HashMetadataPage
	var metadata HashMetadata

	pageMetadata.Swapped = false
	err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &metadata)
	if err != nil {
		return nil, xerrors.Errorf("failed to unpack HashMetadataPage: %w", err)
	}

	if metadata.Magic == HashMagicNumberBE {
		// Re-read the generic metadata as BigEndian
		pageMetadata.Swapped = true
		err := binary.Read(bytes.NewReader(data), binary.BigEndian, &metadata.GenericMetadataPage)
		if err != nil {
			return nil, xerrors.Errorf("failed to unpack HashMetadataPage: %w", err)
		}
	}

	pageMetadata.HashMetadata = metadata

	return &pageMetadata, pageMetadata.validate()
}

func (p *HashMetadata) validate() error {
	err := p.GenericMetadataPage.validate()
	if err != nil {
		return err
	}

	if p.Magic != HashMagicNumber {
		return xerrors.Errorf("unexpected DB magic number: %+v", p.Magic)
	}

	if p.PageType != HashMetadataPageType {
		return xerrors.Errorf("unexpected page type: %+v", p.PageType)
	}

	return nil
}
