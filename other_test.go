package main

import (
	"testing"

	"github.com/sigurn/crc16"
)

var varintTests = []struct {
	str   string
	size  int
	valid bool
}{
	{"\x00", 1, true},
	{"\x7f", 1, true},
	{"\x80", 1, false},
	{"\xc0", 2, false},
	{"\xc0\x80", 2, true},
	{"\xe0\x80\x80", 3, true},
	{"\xf0\x80\x80\x80", 4, true},
	{"\xf8\x80\x80\x80\x80", 5, true},
	{"\xfc\x80\x80\x80\x80\x80", 6, true},
}

func TestVarintLength(t *testing.T) {
	for _, tt := range varintTests {
		got, valid := varintLength([]byte(tt.str))
		if got != tt.size || valid != tt.valid {
			t.Errorf("Length(%x): got size=%v, valid=%v; want size=%v, valid=%v", tt.str, got, valid, tt.size, tt.valid)
		}
	}
}

var frameHeaderTests = []struct {
	str string
	pos int
}{
	{"\x00", -1},
	{"\xff\xf8\xca\x0c\x00\x7c", 0},
	{"\x00\xff\xf8\xca\x0c\x00\x7c", 1},
	{"\x00\x00\xff\xf8\xca\x0c\x00\x7c", 2},
	{"\xff\xf8\xff\xf8\xca\x0c\x00\x7c", 2},
	{"\xff\xf8\xca\x0c\x00\x7c\x00\x00\x00\x00\x00\x00\x00", 0}, // header followed by junk
	{"\xff\xf8\xca\x0c\x00\x7c\x00\x00\x00\x00\xa7\x34", 0},     // a complete frame
}

func TestFindFrameHeader(t *testing.T) {
	for _, tt := range frameHeaderTests {
		pos, found := findFrameHeader([]byte(tt.str))
		if !found && tt.pos == -1 {
			continue // PASS
		}
		if pos != tt.pos {
			t.Errorf("findFrameHeader(% x) = %v, found=%v; want %v, found=true", tt.str, pos, found, tt.pos)
		}
	}
}

var crc16tests = []struct {
	str string
}{
	{"\xff\xf8\xca\x0c\x00\x7c\x00\x00\x00\x00\xa7\x34"}, // a complete frame
}

func TestCRC16(t *testing.T) {
	for _, tt := range crc16tests {
		got := crc16.Checksum([]byte(tt.str), crcTable16)
		want := uint16(0)
		if got != want {
			t.Errorf("CRC16(%x) = %x, want %x", tt.str, got, want)
		}
	}
}
