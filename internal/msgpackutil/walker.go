// Package msgpackutil provides low-level msgpack byte-walking utilities for
// patching MasterMemory table blobs without full deserialization.
//
// The game's C# MessagePack uses int64 (0xd3) encoding for timestamps.
// We walk raw bytes to find and patch these values in-place, preserving
// all other data exactly as-is.
package msgpackutil

import (
	"encoding/binary"
	"fmt"
)

// SkipValue skips one complete msgpack value starting at pos and returns
// the position immediately after it. Supports all standard msgpack types.
func SkipValue(data []byte, pos int) (int, error) {
	if pos >= len(data) {
		return pos, fmt.Errorf("unexpected end of data at pos %d", pos)
	}
	tag := data[pos]

	// positive fixint (0x00–0x7f) or negative fixint (0xe0–0xff)
	if tag <= 0x7f || tag >= 0xe0 {
		return pos + 1, nil
	}

	// fixstr (0xa0–0xbf)
	if tag >= 0xa0 && tag <= 0xbf {
		return pos + 1 + int(tag&0x1f), nil
	}

	// fixarray (0x90–0x9f)
	if tag >= 0x90 && tag <= 0x9f {
		n := int(tag & 0x0f)
		p := pos + 1
		var err error
		for i := 0; i < n; i++ {
			p, err = SkipValue(data, p)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	}

	// fixmap (0x80–0x8f)
	if tag >= 0x80 && tag <= 0x8f {
		n := int(tag & 0x0f)
		p := pos + 1
		var err error
		for i := 0; i < n*2; i++ {
			p, err = SkipValue(data, p)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	}

	// Fixed-size types
	switch tag {
	case 0xc0, 0xc2, 0xc3: // nil, false, true
		return pos + 1, nil
	case 0xca: // float32
		return pos + 5, nil
	case 0xcb: // float64
		return pos + 9, nil
	case 0xcc: // uint8
		return pos + 2, nil
	case 0xcd: // uint16
		return pos + 3, nil
	case 0xce: // uint32
		return pos + 5, nil
	case 0xcf: // uint64
		return pos + 9, nil
	case 0xd0: // int8
		return pos + 2, nil
	case 0xd1: // int16
		return pos + 3, nil
	case 0xd2: // int32
		return pos + 5, nil
	case 0xd3: // int64
		return pos + 9, nil
	case 0xd4: // fixext 1
		return pos + 3, nil
	case 0xd5: // fixext 2
		return pos + 4, nil
	case 0xd6: // fixext 4
		return pos + 6, nil
	case 0xd7: // fixext 8
		return pos + 10, nil
	case 0xd8: // fixext 16
		return pos + 18, nil
	}

	// Length-prefixed: bin, str, ext
	switch tag {
	case 0xc4: // bin8
		return pos + 2 + int(data[pos+1]), nil
	case 0xc5: // bin16
		return pos + 3 + int(binary.BigEndian.Uint16(data[pos+1:])), nil
	case 0xc6: // bin32
		return pos + 5 + int(binary.BigEndian.Uint32(data[pos+1:])), nil
	case 0xd9: // str8
		return pos + 2 + int(data[pos+1]), nil
	case 0xda: // str16
		return pos + 3 + int(binary.BigEndian.Uint16(data[pos+1:])), nil
	case 0xdb: // str32
		return pos + 5 + int(binary.BigEndian.Uint32(data[pos+1:])), nil
	case 0xc7: // ext8
		return pos + 3 + int(data[pos+1]), nil // +1 len, +1 type, +n data
	case 0xc8: // ext16
		return pos + 4 + int(binary.BigEndian.Uint16(data[pos+1:])), nil
	case 0xc9: // ext32
		return pos + 6 + int(binary.BigEndian.Uint32(data[pos+1:])), nil
	}

	// Array / map 16/32
	switch tag {
	case 0xdc: // array16
		n := int(binary.BigEndian.Uint16(data[pos+1:]))
		p := pos + 3
		var err error
		for i := 0; i < n; i++ {
			p, err = SkipValue(data, p)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	case 0xdd: // array32
		n := int(binary.BigEndian.Uint32(data[pos+1:]))
		p := pos + 5
		var err error
		for i := 0; i < n; i++ {
			p, err = SkipValue(data, p)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	case 0xde: // map16
		n := int(binary.BigEndian.Uint16(data[pos+1:]))
		p := pos + 3
		var err error
		for i := 0; i < n*2; i++ {
			p, err = SkipValue(data, p)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	case 0xdf: // map32
		n := int(binary.BigEndian.Uint32(data[pos+1:]))
		p := pos + 5
		var err error
		for i := 0; i < n*2; i++ {
			p, err = SkipValue(data, p)
			if err != nil {
				return p, err
			}
		}
		return p, nil
	}

	return pos, fmt.Errorf("unknown msgpack tag 0x%02x at pos %d", tag, pos)
}

// ReadArrayLen reads an array header at pos and returns (element_count, first_element_pos).
func ReadArrayLen(data []byte, pos int) (int, int, error) {
	tag := data[pos]
	if tag >= 0x90 && tag <= 0x9f {
		return int(tag & 0x0f), pos + 1, nil
	}
	if tag == 0xdc {
		return int(binary.BigEndian.Uint16(data[pos+1:])), pos + 3, nil
	}
	if tag == 0xdd {
		return int(binary.BigEndian.Uint32(data[pos+1:])), pos + 5, nil
	}
	return 0, pos, fmt.Errorf("expected array at pos %d, got tag 0x%02x", pos, tag)
}

// ReadInt reads an integer at pos without advancing. Returns the value and true,
// or 0 and false if the value at pos is not an integer type.
func ReadInt(data []byte, pos int) (int64, bool) {
	tag := data[pos]
	if tag <= 0x7f {
		return int64(tag), true
	}
	if tag >= 0xe0 {
		return int64(int8(tag)), true
	}
	switch tag {
	case 0xcc:
		return int64(data[pos+1]), true
	case 0xcd:
		return int64(binary.BigEndian.Uint16(data[pos+1:])), true
	case 0xce:
		return int64(binary.BigEndian.Uint32(data[pos+1:])), true
	case 0xd0:
		return int64(int8(data[pos+1])), true
	case 0xd1:
		return int64(int16(binary.BigEndian.Uint16(data[pos+1:]))), true
	case 0xd2:
		return int64(int32(binary.BigEndian.Uint32(data[pos+1:]))), true
	case 0xd3:
		return int64(binary.BigEndian.Uint64(data[pos+1:])), true
	}
	return 0, false
}

// ReadLZ4ExtHeader parses the uncompressed-length prefix from an ExtType(99) payload.
// C# MessagePack writes the original size as a msgpack int before the LZ4 bytes.
func ReadLZ4ExtHeader(extData []byte) (uncompressedLen int, lz4Data []byte, err error) {
	tag := extData[0]
	switch {
	case tag <= 0x7f: // positive fixint
		return int(tag), extData[1:], nil
	case tag == 0xd2: // int32
		return int(int32(binary.BigEndian.Uint32(extData[1:5]))), extData[5:], nil
	case tag == 0xce: // uint32
		return int(binary.BigEndian.Uint32(extData[1:5])), extData[5:], nil
	case tag == 0xd1: // int16
		return int(int16(binary.BigEndian.Uint16(extData[1:3]))), extData[3:], nil
	case tag == 0xcd: // uint16
		return int(binary.BigEndian.Uint16(extData[1:3])), extData[3:], nil
	default:
		return 0, nil, fmt.Errorf("unexpected msgpack tag 0x%02x in LZ4 ext header", tag)
	}
}
