package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/edsrzf/mmap-go"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: flacrate -rate N [files...]")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	rateFlag := flag.Int("rate", 0, "desired sample rate in Hz (required)")
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "flacrate: error: please specify one or more files to patch")
		os.Exit(1)
	}
	if *rateFlag == 0 {
		fmt.Fprintln(os.Stderr, "flacrate: error: -rate flag is required")
		os.Exit(1)
	}
	switch *rateFlag {
	case 8000, 16000, 24000, 32000, 48000, 96000, 192000, 22050, 44100, 88200, 176400:
		// ok
	default:
		fmt.Fprintf(os.Stderr, "flacrate: error: -rate=%v is unsupported\n", *rateFlag)
		fmt.Fprint(os.Stderr, "flacrate: supported rates are 8000, 16000, 22050, 24000, 32000, 44100,\n")
		fmt.Fprint(os.Stderr, "          48000, 88200, 96000, 176400, and 192000 Hz\n")
	}

	for _, filename := range args {
		err := patchFile(filename, *rateFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%q: %v\n", filename, err)
		}
	}
}

func patchFile(filename string, sampleRate int) error {
	// open file
	f, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	// mmap it
	m, err := mmap.Map(f, mmap.RDWR, 0)
	if err != nil {
		return err
	}
	defer func() {
		if m != nil {
			m.Unmap()
		}
	}()
	// patch
	patcher := NewSampleRatePatcher(sampleRate)
	if err := FixBytes(m, patcher); err != nil {
		return err
	}
	// cleanup
	if err := m.Unmap(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil

}
