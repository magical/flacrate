// Copyright © 2019 Andrew Ekstedt
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"

	"github.com/go-daq/crc8"
	"github.com/sigurn/crc16"
)

const flacMagic = "fLaC"

const (
	frameHeaderMinLen = 4 + 1 + 0 + 0 + 1
	frameHeaderMaxLen = 4 + 7 + 2 + 2 + 1
)

type Patcher interface {
	// PatchFrame takes a single block of FLAC data,
	// either a metadata block or an audio frame,
	// and potentially modifies it.
	// HeaderLen is the size of the frame header.
	PatchFrame(frame []byte, headerLen int) error
}

type streaminfo struct {
	minFrameSize int
	maxFrameSize int
}

// FixBytes modifies a FLAC file in place, calling patcher.PatchFrame on every frame.
func FixBytes(flacBytes []byte, patcher Patcher) error {
	if len(flacBytes) < 4 {
		return errors.New("file too short")
	}
	if string(flacBytes[:4]) != flacMagic {
		return errors.New("not a FLAC file")
	}
	// Metadata blocks
	// https://xiph.org/flac/format.html#metadata_block
	var si streaminfo
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

		// get some info from the STREAMINFO block
		if blockType == blockStreaminfo {
			si.minFrameSize = int(p[8])<<16 | int(p[9])<<8 | int(p[10])
			si.maxFrameSize = int(p[11])<<16 | int(p[12])<<8 | int(p[13])
		}

		if err := patcher.PatchFrame(p[0:blockLength], 4); err != nil {
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

		searchStart := headerLen
		searchEnd := len(p)
		if searchStart < si.minFrameSize {
			searchStart = si.minFrameSize
			searchEnd = si.maxFrameSize + frameHeaderMaxLen
			if len(p) < searchEnd {
				searchEnd = len(p)
			}
		}
		pos, found := findFrameHeader(p[searchStart:searchEnd])
		pos += searchStart

		// XXX rewrite this logic
		const reasonableFrameSize = 16 * 1024
		var frameEnd int
		if found {
			frameEnd = pos
		} else {
			if len(p) <= reasonableFrameSize || len(p) <= si.maxFrameSize {
				// i guess we are at EOF?
				frameEnd = len(p)
			} else {
				//log.Printf("frame %x: can't find end, pos=%x, len=%x, reasonable=%x", len(flacBytes)-len(p), pos, len(p), reasonableFrameSize)
				return errors.New("couldn't find next frame")
			}
		}

		// check CRC-16 for entire frame
		// if it doesn't match and we have more search space to work with,
		// search for a different header
		h := crc16.Checksum(p[:frameEnd], crcTable16)
		if h != 0 {
			frameStart := len(flacBytes) - len(p)
			for h != 0 && pos+1 < searchEnd {
				log.Printf("frame %x: possibly bogus frame header found at %x", frameStart, frameStart+pos)
				newPos := pos + 1
				i, found := findFrameHeader(p[newPos:searchEnd])
				newPos += i
				if !found {
					break
				}
				h = crc16.Update(h, p[pos:newPos], crcTable16)
				pos = newPos
			}
			if h == 0 {
				//log.Printf("frame %x: real end at %x", frameStart, frameStart+pos)
				frameEnd = pos
			} else {
				return errors.New("invalid frame footer CRC")
			}
		}

		if err := patcher.PatchFrame(p[:frameEnd], headerLen); err != nil {
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

var crcTable16 = crc16.MakeTable(crc16.Params{Poly: 0x8005, Init: 0}) // x^16 + x^15 + x^2 + x^0

// if not found, returns len(p), false
func findFrameHeader(p []byte) (position int, found bool) {
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
				//log.Printf("invalid frame header CRC: %x = %d", b, h)
			}
		}
	}
	// TODO: return some sort of partial success
	// if we didn't find a valid header,
	// but we found a sync code with a truncated header?
	return len(p), false
}

const (
	blockStreaminfo = 0
)

// SampleRatePatcher is a Patcher which overwrites the sample rate of every frame.
type SampleRatePatcher struct {
	sampleRate int
}

func NewSampleRatePatcher(sampleRate int) *SampleRatePatcher {
	return &SampleRatePatcher{sampleRate}
}

// fixFrame sets the sample rate in a single frame.
// The provided byte slice must be a complete FLAC frame.
func (q *SampleRatePatcher) PatchFrame(frame []byte, headerLen int) error {
	p := frame
	if isSync(frame) {
		switch p[2] & 0x0F {
		case 0xC, 0xD, 0xE:
			return errors.New("frame uses a nonstandard sample rate")
		}

		sampleCode, err := getSampleRateCode(q.sampleRate)
		if err != nil {
			return err
		}
		p[2] = p[2]&0xF0 | byte(sampleCode)

		FixCRCs(frame, headerLen)

		return nil
	}

	// Metadata block
	if p[0]&0x7F == blockStreaminfo {
		fileRate := int(p[14])<<12 + int(p[15])<<4 + int(p[16]>>4)
		if fileRate == 0 {
			return errors.New("STREAMINFO: found sample rate of 0, which is invalid")
		}
		if fileRate == q.sampleRate {
			return fmt.Errorf("STREAMINFO: sample rate is already %d", q.sampleRate)
		}
		switch fileRate {
		case 8000, 16000, 24000, 32000, 48000, 96000, 192000,
			22050, 44100, 88200, 176400:
			// ok
			//fmt.Printf("STREAMINFO: found sample rate %d", fileRate)
		default:
			return fmt.Errorf("STREAMINFO: found nonstandard sample rate %d, which is unsupported", fileRate)
		}

		p[14] = byte(q.sampleRate >> 12)
		p[15] = byte(q.sampleRate >> 4)
		p[16] = byte(q.sampleRate<<4) | p[16]&0x0F
	}
	return nil
}

// returns the frame header code for standard sample rates
func getSampleRateCode(sampleRate int) (code int, err error) {
	// https://xiph.org/flac/format.html#frame_header
	switch sampleRate {
	case 88200:
		return 1, nil
	case 176400:
		return 2, nil
	case 192000:
		return 3, nil
	case 8000:
		return 4, nil
	case 16000:
		return 5, nil
	case 22050:
		return 6, nil
	case 24000:
		return 7, nil
	case 32000:
		return 8, nil
	case 44100:
		return 9, nil
	case 48000:
		return 10, nil
	case 96000:
		return 11, nil
	default:
		return 0, errors.New("nonstandard sample rate")
	}
}

// FixCRCs updates the header and footer CRCs for an audio frame.
// It does not do any checking to verify that the data
// given is actually an audio frame.
func FixCRCs(p []byte, headerLen int) {
	// Update the header CRC-8
	p[headerLen-1] = crc8.Checksum(p[:headerLen-1], crcTable)

	// Update the footer CRC-16
	// There is a cleverer way to do this, which would avoid having
	// to recompute the entire CRC, but this is easier
	h := crc16.Checksum(p[:len(p)-2], crcTable16)
	p[len(p)-2] = byte(h >> 8)
	p[len(p)-1] = byte(h)
}
