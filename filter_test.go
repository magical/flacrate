package main

import (
	"bytes"
	"encoding/hex"
	"testing"
)

var testFile44100 = "664c6143000000221000100000000c00000c0ac44170000010004072783b" +
	"8efb99a9e5817067d68f61c684000044200000007265666572656e636520" +
	"6c6962464c414320312e332e322032303137303130310100000018000000" +
	"436f6d6d656e743d50726f63657373656420627920536f58fff8c90c00c1" +
	"00000000a1e6"

var testFile48000 = "664c6143000000221000100000000c00000c0bb80170000010004072783b" +
	"8efb99a9e5817067d68f61c684000044200000007265666572656e636520" +
	"6c6962464c414320312e332e322032303137303130310100000018000000" +
	"436f6d6d656e743d50726f63657373656420627920536f58fff8ca0c007c" +
	"00000000a734"

func TestFix(t *testing.T) {
	got, err := hex.DecodeString(testFile44100)
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(testFile48000)
	if err != nil {
		t.Fatal(err)
	}

	err = FixBytes(got, 48000)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("FixBytes failed.\ngot:\n%s\nwant:\n%s", hex.Dump(got), hex.Dump(want))
	}
}
