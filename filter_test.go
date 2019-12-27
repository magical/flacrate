package main

import (
	"bytes"
	"encoding/hex"
	"io/ioutil"
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

func TestFixBytes(t *testing.T) {
	got, err := hex.DecodeString(testFile44100)
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(testFile48000)
	if err != nil {
		t.Fatal(err)
	}

	patcher := NewSampleRatePatcher(48000)

	err = FixBytes(got, patcher)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("FixBytes failed.\ngot:\n%s\nwant:\n%s", hex.Dump(got), hex.Dump(want))
	}
}

func TestFixBytes2(t *testing.T) {
	got, err := ioutil.ReadFile("testdata/derezz44100.flac")
	if err != nil {
		t.Fatal(err)
	}
	want, err := ioutil.ReadFile("testdata/derezz22050.flac")
	if err != nil {
		t.Fatal(err)
	}

	patcher := NewSampleRatePatcher(22050)

	if err := FixBytes(got, patcher); err != nil {
		t.Error(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("FixBytes failed")
	}
}

func TestFixBytes3(t *testing.T) {
	// This is a snippet of a file which has a bogus but valid-looking
	// frame header in the middle of a frame payload.
	data, err := ioutil.ReadFile("testdata/business2.flac")
	if err != nil {
		t.Fatal(err)
	}

	err = FixBytes(data, NullPatcher{})
	if err != nil {
		t.Errorf("want no error, got: %v", err)
	}
}

type NullPatcher struct{}

func (q NullPatcher) PatchFrame(p []byte, headerLen int) error { return nil }
