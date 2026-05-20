package main

import (
	"github.com/ystepanoff/goppy/firmware/config"
	"github.com/ystepanoff/goppy/firmware/instruments"
	"github.com/ystepanoff/goppy/firmware/networks"
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
