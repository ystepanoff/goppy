package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/ystepanoff/goppy/internal/protocol"
)

func cmdNote(args []string) error {
	fs := flag.NewFlagSet("note", flag.ExitOnError)
	pf := addPortFlags(fs)
	device := fs.Uint("device", 0x01, "target device address (1..127)")
	drive := fs.Uint("drive", 1, "target drive sub-address (1..8)")
	note := fs.Int("note", 60, "MIDI note number (0..127, 60 = middle C)")
	duration := fs.Duration("duration", 500*time.Millisecond,
		"hold time before sending NOTE_OFF; 0 leaves the note ringing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *note < 0 || *note > 127 {
		return fmt.Errorf("note out of range: %d (must be 0..127)", *note)
	}
	if *device == 0 || *device > 127 {
		return fmt.Errorf("device address out of range: %d", *device)
	}
	if *drive == 0 || *drive > 255 {
		return fmt.Errorf("drive sub-address out of range: %d", *drive)
	}

	port, err := pf.open()
	if err != nil {
		return err
	}
	defer port.Close()

	dev := byte(*device)
	sub := byte(*drive)

	if _, err := port.Write(protocol.NoteOn(dev, sub, byte(*note))); err != nil {
		return fmt.Errorf("write note on: %w", err)
	}
	fmt.Printf("note on  → device=0x%02X drive=%d note=%d\n", dev, sub, *note)

	if *duration > 0 {
		time.Sleep(*duration)
		if _, err := port.Write(protocol.NoteOff(dev, sub)); err != nil {
			return fmt.Errorf("write note off: %w", err)
		}
		fmt.Printf("note off → device=0x%02X drive=%d\n", dev, sub)
	}
	return nil
}
