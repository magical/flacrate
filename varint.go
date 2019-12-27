package main

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
