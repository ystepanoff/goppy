package main

import (
	"github.com/ystepanoff/goppy/arduino/config"
	"github.com/ystepanoff/goppy/arduino/instruments"
	"github.com/ystepanoff/goppy/arduino/networks"
)

func main() {
	floppy := instruments.NewFloppyDrives()
	floppy.Setup()

	instruments.InitTimer(config.TimerResolution, floppy.Tick)

	serial := networks.NewSerial(floppy)
	serial.Begin()

	for {
		serial.ReadMessages()
	}
}
