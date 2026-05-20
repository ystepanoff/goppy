package main

import (
	"flag"
	"fmt"
	"time"

	"go.bug.st/serial"
)

const defaultBaud = 57600

// portFlags adds shared --port and --baud flags to fs.
type portFlags struct {
	port string
	baud int
}

func addPortFlags(fs *flag.FlagSet) *portFlags {
	pf := &portFlags{}
	fs.StringVar(&pf.port, "port", "", "serial port path (e.g. /dev/tty.usbmodem1101 or COM3) — required")
	fs.IntVar(&pf.baud, "baud", defaultBaud, "serial baud rate")
	return pf
}

func (pf *portFlags) open() (serial.Port, error) {
	if pf.port == "" {
		return nil, fmt.Errorf("--port is required (try `goppy ports` or check Arduino IDE for the path)")
	}
	p, err := serial.Open(pf.port, &serial.Mode{BaudRate: pf.baud})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", pf.port, err)
	}
	// Arduino Uno reboots on DTR; give the bootloader time to hand off.
	time.Sleep(2 * time.Second)
	return p, nil
}
