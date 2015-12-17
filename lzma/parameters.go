// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"fmt"
	"io"
)

// uint32LE reads an uint32 integer from a byte slize
func uint32LE(b []byte) uint32 {
	x := uint32(b[3]) << 24
	x |= uint32(b[2]) << 16
	x |= uint32(b[1]) << 8
	x |= uint32(b[0])
	return x
}

// uint64LE converts the uint64 value stored as little endian to an uint64
// value.
func uint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
	return x
}

// putUint32LE puts an uint32 integer into a byte slice that must have at least
// a lenght of 4 bytes.
func putUint32LE(b []byte, x uint32) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

// putUint64LE puts the uint64 value into the byte slice as little endian
// value. The byte slice b must have at least place for 8 bytes.
func putUint64LE(b []byte, x uint64) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

// noHeaderLen defines the value of the length field in the LZMA header.
const noHeaderLen uint64 = 1<<64 - 1

// Parameters provides the Parameters of an LZMA reader and LZMA writer.
type Parameters struct {
	Properties Properties
	DictCap    int
	// uncompressed size; negative value if no size given
	Size int64
	// minimum size for the buffer
	BufSize   int
	EOSMarker bool
}

// Default provides the default parameters for the LZMA writer.
var Default = Parameters{
	Properties: Properties{3, 0, 2},
	DictCap:    8 * 1024 * 1024,
	Size:       -1,
}

// normalizeReader normalizes the parameters for the LZMA reader.
func (p *Parameters) normalizeReader() {
	if p.DictCap < MinDictCap {
		p.DictCap = MinDictCap
	}
	if p.Size < 0 {
		p.EOSMarker = true
	}
	if p.BufSize <= 0 {
		p.BufSize = p.DictCap
	}
}

// readHeader reads the classic LZMA header.
func readHeader(r io.Reader) (p *Parameters, err error) {
	b := make([]byte, 13)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	props, err := PropertiesForCode(b[0])
	if err != nil {
		return nil, err
	}
	p = &Parameters{
		Properties: props,
		Size:       -1,
	}
	p.DictCap = int(uint32LE(b[1:]))
	if p.DictCap < 0 {
		return nil, errors.New(
			"LZMA header: dictionary capacity exceeds maximum " +
				"integer")
	}
	p.BufSize = p.DictCap
	u := uint64LE(b[5:])
	if u == noHeaderLen {
		p.EOSMarker = true
	} else {
		p.Size = int64(u)
		if p.Size < 0 {
			return nil, errors.New(
				"LZMA header: uncompressed length in header " +
					" out of int64 range")
		}
	}
	return p, nil
}

// writeHeader writes the header for classic LZMA files.
func writeHeader(w io.Writer, p *Parameters) error {
	if err := p.Properties.Verify(); err != nil {
		return err
	}
	b := make([]byte, 13)
	b[0] = p.Properties.Code()
	if !(0 <= p.DictCap && p.DictCap <= MaxDictCap) {
		return fmt.Errorf("write LZMA header: DictCap %d out of range",
			p.DictCap)
	}
	putUint32LE(b[1:5], uint32(p.DictCap))
	var l uint64
	if p.Size >= 0 {
		l = uint64(p.Size)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b[5:], l)
	_, err := w.Write(b)
	return err
}
