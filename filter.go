package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/go-daq/crc8"
)

const flacMagic = "fLaC"

const (
	frameHeaderMinLen = 4 + 1 + 0 + 0 + 1
	frameHeaderMaxLen = 4 + 7 + 2 + 2 + 1
)

// FixBytes fixes the flac file given as a byte slice in place,
// overwriting the sample rate in every frame with sampleRate.
func FixBytes(flacBytes []byte, sampleRate int) error {
	if len(flacBytes) < 4 {
		return errors.New("file too short")
	}
	if string(flacBytes[:4]) != flacMagic {
		return errors.New("not a FLAC file")
	}
	// Metadata blocks
	// https://xiph.org/flac/format.html#metadata_block
	p := flacBytes[4:]
	for lastBlock := false; !lastBlock; {
		if len(p) <= 0 {
			return errors.New("expected a metadata block")
		}
		if len(p) < 4 {
			return errors.New("truncated metadata block header")
		}
		lastBlock = (p[0]&0x80 != 0)
		blockType := p[0] & 0x7f
		if blockType == 0x7f {
			return errors.New("invalid metadata block type")
		}
		blockLength := int(p[1])<<16 | int(p[2])<<8 | int(p[3])

		if len(p) < 4+blockLength {
			return errors.New("truncated metadata block")
		}

		if err := fixFrame(p[0:blockLength], sampleRate); err != nil {
			return err
		}

		p = p[4+blockLength:]
	}

	// Audio frames
	// https://xiph.org/flac/format.html#frame
	for len(p) > 0 {
		if len(p) < frameHeaderMinLen {
			return errors.New("truncated frame header")
		}
		if !isSync(p[0:2]) {
			return errors.New("invalid frame sync code")
		}

		headerLen := 5
		// check if block size included in header
		switch p[2] >> 4 {
		case 0x6:
			headerLen += 1
		case 0x7:
			headerLen += 2
		}

		// check if sample rate included in header
		// TODO: check if sample rate matches rate in STREAMINFO
		switch p[2] & 0x0F {
		case 0xC, 0xD, 0xE:
			return errors.New("frame uses a nonstandard sample rate")
		}

		n, valid := varintLength(p[4:])
		if !valid {
			return errors.New("frame has invalid frame/sample number field")
		}
		headerLen += n

		// Unfortunately, FRAMEs don't have a length field.
		// (I guess it's implied by the data)
		// I don't really want to figure out how to decode subframes,
		// so instead we'll find the frame length by finding the start of the next frame.
		// The start of a frame can be identified by
		// 1) the frame sync code, 0b11111111 111110xx,
		// 2) and a valid CRC-8 at the end of the header

		// Sooo... that's what we'll do. Search for the sync code, check the CRC,
		// if it's right, we found a frame, if it's not, move on.

		pos, found := findFrameHeader(p[headerLen:])
		pos += headerLen
		const reasonableFrameSize = 16 * 1024
		var frameEnd int
		if found {
			frameEnd = pos
		} else {
			if len(p) < reasonableFrameSize {
				// i guess we are at EOF?
				frameEnd = len(p)
			} else {
				return errors.New("couldn't find next frame")
			}
		}

		// TODO: check CRC-16 for entire frame

		if err := fixFrame(p[:frameEnd], sampleRate); err != nil {
			return err
		}

		if frameEnd == 0 {
			panic("no progress")
		}

		p = p[frameEnd:]
	}

	return nil
}

func isSync(p []byte) bool {
	return len(p) >= 2 && p[0] == 0xFF && p[1]&^3 == 0xF8
}

var crcTable = crc8.MakeTable(0x07) // x^8 + x^2 + x^1 + x^0

// if not found, returns len(p), false
func findFrameHeader(p []byte) (position int, found bool) {
	// TODO: limit search between min frame size and max frame size (from STREAMINFO block)
	//log.Println("findFrameHeader start")
	//if len(p) < 20 {
	//	log.Printf("%x", p)
	//}
	for i := 0; i < len(p); i++ {
		//log.Println("findFrameHeader", i)
		if j := bytes.IndexByte(p[i:], 0xFF); j < 0 {
			break
		} else {
			i += j
		}
		if isSync(p[i:]) {
			//log.Println("findFrameHeader: sync at", i)
			headerLen := 5

			// check if block size included in header
			switch p[i+2] >> 4 {
			case 0x6:
				headerLen += 1
			case 0x7:
				headerLen += 2
			}

			// check if sample rate included in header
			switch p[i+2] & 0x0F {
			case 0xC:
				headerLen += 1
			case 0xD, 0xE:
				headerLen += 2
			}

			n, _ := varintLength(p[i+4:])
			headerLen += n

			if i+headerLen > len(p) {
				continue
			}

			// does the CRC match?
			if crc8.Checksum(p[i:i+headerLen], crcTable) == 0 {
				return i, true
			} else {
				//b := p[i : i+headerLen]
				//h := crc8.Checksum(b, crcTable)
				//log.Printf("invalid CRC: %x = %d", b, h)
			}
		}
	}
	// TODO: return some sort of partial success
	// if we didn't find a valid header,
	// but we found a sync code with a truncated header?
	return len(p), false
}

// Computes the size of a compressed FLAC integer.
// Only used for the frame number/sample number in the FRAME header.
func varintLength(p []byte) (size int, valid bool) {
	if len(p) < 1 {
		return 0, false
	}

	var n int
	x := p[0]
	if x&0x80 == 0 { // 0xxxxxxx
		n = 1
	} else if x&0x80 != 0 && x&0x40 == 0 { // 10xxxxxx
		// continuation byte, invalid at start
		return 1, false
	} else if x&0xC0 != 0 && x&0x20 == 0 { // 110xxxxx
		n = 2
	} else if x&0xE0 != 0 && x&0x10 == 0 { // 1110xxxx
		n = 3
	} else if x&0xF0 != 0 && x&0x08 == 0 { // 11110xxx
		n = 4
	} else if x&0xF8 != 0 && x&0x04 == 0 { // 111110xx
		n = 5
	} else if x&0xFC != 0 && x&0x02 == 0 { // 1111110x
		n = 6
	} else if x&0xFE != 0 && x&0x01 == 0 { // 11111110
		n = 7
	} else {
		return 1, false
	}

	if n > len(p) {
		return n, false
	}

	valid = true
	for i := 1; i < n; i++ {
		if p[i]&0x80 == 0 || p[i]&0x40 != 0 { // 10xxxxxx
			valid = false
		}
	}
	return n, valid
}

const (
	blockStreaminfo = 0
)

// fixFrame sets the sample rate in a single frame.
// The provided byte slice must be a complete FLAC frame.
func fixFrame(frame []byte, sampleRate int) error {
	p := frame
	if isSync(frame) {
		// TODO
		return nil
	}

	// Metadata block
	if p[0]&0x7F == blockStreaminfo {
		fileRate := int(p[14])<<12 + int(p[15])<<4 + int(p[16]>>4)
		if fileRate == 0 {
			return errors.New("STREAMINFO: found sample rate of 0, which is invalid")
		}
		if fileRate == sampleRate {
			return fmt.Errorf("STREAMINFO: sample rate is already %d", sampleRate)
		}
		switch fileRate {
		case 8000, 16000, 24000, 32000, 48000, 96000, 192000,
			22050, 44100, 88200, 176400:
			// ok
			//fmt.Printf("STREAMINFO: found sample rate %d", fileRate)
		default:
			return fmt.Errorf("STREAMINFO: found nonstandard sample rate %d, which is unsupported", fileRate)
		}

		p[14] = byte(sampleRate >> 12)
		p[15] = byte(sampleRate >> 4)
		p[16] = byte(sampleRate<<4) | p[16]&0x0F
	}
	return nil
}
