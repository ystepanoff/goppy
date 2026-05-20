package main

import (
	"flag"
	"fmt"

	"github.com/ystepanoff/goppy/internal/protocol"
)

func cmdReset(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	pf := addPortFlags(fs)
	device := fs.Uint("device", 0x01, "target device address (only used with --drive)")
	drive := fs.Int("drive", -1, "specific drive to reset; omit to broadcast a system RESET to all devices")
	if err := fs.Parse(args); err != nil {
		return err
	}

	port, err := pf.open()
	if err != nil {
		return err
	}
	defer port.Close()

	if *drive < 0 {
		if _, err := port.Write(protocol.Reset()); err != nil {
			return fmt.Errorf("write system reset: %w", err)
		}
		fmt.Println("system reset broadcast")
		return nil
	}
	if *drive == 0 || *drive > 255 {
		return fmt.Errorf("drive sub-address out of range: %d", *drive)
	}
	if _, err := port.Write(protocol.DriveReset(byte(*device), byte(*drive))); err != nil {
		return fmt.Errorf("write drive reset: %w", err)
	}
	fmt.Printf("drive reset → device=0x%02X drive=%d\n", *device, *drive)
	return nil
}
