// Command goppy is the host-side CLI for the goppy/arduino floppy-drive
// music firmware. It speaks the Moppy v2 protocol over a USB serial port.
//
// Usage:
//
//	goppy ping   --port /dev/tty.usbmodem...
//	goppy note   --port ... --drive 1 --note 60 [--duration 500ms]
//	goppy reset  --port ... [--drive N]
//	goppy play   --port ... song.mid
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "ping":
		err = cmdPing(args)
	case "note":
		err = cmdNote(args)
	case "reset":
		err = cmdReset(args)
	case "play":
		err = cmdPlay(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `goppy: Moppy v2 host CLI

Subcommands:
  ping   Discover a connected goppy/Moppy device.
  note   Send a single NOTE_ON (and optional auto NOTE_OFF) to a drive.
  reset  Reset all drives, or a specific drive with --drive.
  play   Stream a MIDI file to the device.

Run 'goppy <subcommand> -h' for subcommand flags.`)
}
