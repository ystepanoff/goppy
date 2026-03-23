// Package instruments provides hardware instrument drivers for Moppy.
// FloppyDrives is the original Moppy instrument - floppy drive stepper motor music.
package instruments

import (
	"machine"
	"math"
	"time"

	"github.com/ystepanoff/goppy/arduino/config"
	"github.com/ystepanoff/goppy/arduino/notes"
)

// BendOctaves is the pitch bend range in octaves at full deflection.
// 200 cents / 1200 cents-per-octave = 1/6 octave.
const BendOctaves = 200.0 / 1200.0

// MaxFloppyNote is the highest MIDI note to attempt on floppy drives.
// Higher notes may work but can cause instability.
const MaxFloppyNote = 71

// firstDrive and lastDrive define the 1-based drive range.
const (
	firstDrive = 1
	lastDrive  = config.NumDrives
)

// FloppyDrives controls an array of floppy drives to produce music.
// Each drive's stepper motor head is pulsed at specific frequencies to
// generate audible tones.
type FloppyDrives struct {
	// All arrays are 1-indexed (index 0 unused) to match sub-address numbering.

	// currentPosition tracks the head position for each drive (0 to MaxPosition).
	currentPosition [lastDrive + 1]uint16

	// minPosition and maxPosition define the movement range per drive.
	// Can be narrowed to restrict head travel.
	minPosition [lastDrive + 1]uint16
	maxPosition [lastDrive + 1]uint16

	// currentPeriod is the current note period in timer ticks (0 = silent).
	currentPeriod [lastDrive + 1]uint16

	// originalPeriod is the period before pitch bend modifications.
	originalPeriod [lastDrive + 1]uint16

	// currentTick counts timer ticks since last pin toggle per drive.
	currentTick [lastDrive + 1]uint16

	// directionState tracks the direction pin state per drive (false=forward, true=reverse).
	directionState [lastDrive + 1]bool

	// stepState tracks the step pin toggle state per drive.
	stepState [lastDrive + 1]bool

	// pins caches the machine.Pin for each drive's step and direction pins.
	stepPins [lastDrive + 1]machine.Pin
	dirPins  [lastDrive + 1]machine.Pin
}

// NewFloppyDrives creates a new FloppyDrives instance.
func NewFloppyDrives() *FloppyDrives {
	fd := &FloppyDrives{}

	// Pre-calculate pin mappings and set default movement range.
	for d := byte(firstDrive); d <= lastDrive; d++ {
		fd.stepPins[d] = machine.Pin(config.FirstPin + (d-1)*2)
		fd.dirPins[d] = machine.Pin(config.FirstPin + (d-1)*2 + 1)
		fd.maxPosition[d] = config.MaxPosition
	}

	return fd
}

// Setup configures all drive pins as outputs and resets drives to position 0.
// Must be called before Tick or message handling.
func (fd *FloppyDrives) Setup() {
	// Configure all drive pins as outputs.
	for d := byte(firstDrive); d <= lastDrive; d++ {
		fd.stepPins[d].Configure(machine.PinConfig{Mode: machine.PinOutput})
		fd.dirPins[d].Configure(machine.PinConfig{Mode: machine.PinOutput})
	}

	// Reset all drives to position 0.
	fd.ResetAll()
	time.Sleep(500 * time.Millisecond)

	// Play startup sound if configured.
	if config.PlayStartupSound {
		fd.startupSound(firstDrive)
		time.Sleep(500 * time.Millisecond)
		fd.ResetAll()
	}
}

// Tick is called by the timer interrupt at TimerResolution intervals.
// It advances each active drive's tick counter and toggles the step pin
// when the note period is reached. This must be kept fast.
func (fd *FloppyDrives) Tick() {
	for d := byte(firstDrive); d <= lastDrive; d++ {
		if fd.currentPeriod[d] > 0 {
			fd.currentTick[d]++
			if fd.currentTick[d] >= fd.currentPeriod[d] {
				fd.togglePin(d)
				fd.currentTick[d] = 0
			}
		}
	}
}

// togglePin advances the stepper motor one step, reversing direction at boundaries.
func (fd *FloppyDrives) togglePin(driveNum byte) {
	// Reverse direction at position boundaries.
	if fd.currentPosition[driveNum] >= fd.maxPosition[driveNum] {
		fd.directionState[driveNum] = true // reverse
		fd.dirPins[driveNum].High()
	} else if fd.currentPosition[driveNum] <= fd.minPosition[driveNum] {
		fd.directionState[driveNum] = false // forward
		fd.dirPins[driveNum].Low()
	}

	// Update position.
	if fd.directionState[driveNum] {
		fd.currentPosition[driveNum]--
	} else {
		fd.currentPosition[driveNum]++
	}

	// Pulse the step pin.
	if fd.stepState[driveNum] {
		fd.stepPins[driveNum].High()
	} else {
		fd.stepPins[driveNum].Low()
	}
	fd.stepState[driveNum] = !fd.stepState[driveNum]
}

// HandleSystemMessage processes system-wide commands (address 0x00).
func (fd *FloppyDrives) HandleSystemMessage(command byte, payload []byte) {
	switch command {
	case config.CmdReset:
		fd.ResetAll()
	case config.CmdSequenceStop:
		fd.haltAllDrives()
	}
}

// HandleDeviceMessage processes commands for individual drives.
func (fd *FloppyDrives) HandleDeviceMessage(subAddress byte, command byte, payload []byte) {
	switch command {
	case config.DevCmdReset:
		if subAddress == 0x00 {
			fd.ResetAll()
		} else {
			fd.reset(subAddress)
		}
	case config.DevCmdNoteOn:
		if len(payload) > 0 && payload[0] <= MaxFloppyNote {
			fd.currentPeriod[subAddress] = notes.NoteDoubleTicks[payload[0]]
			fd.originalPeriod[subAddress] = fd.currentPeriod[subAddress]
		}
	case config.DevCmdNoteOff:
		fd.currentPeriod[subAddress] = 0
		fd.originalPeriod[subAddress] = 0
	case config.DevCmdBendPitch:
		if len(payload) >= 2 {
			fd.bendPitch(subAddress, payload)
		}
	}
}

// bendPitch applies pitch bend to a drive's current note.
func (fd *FloppyDrives) bendPitch(driveNum byte, payload []byte) {
	// 14-bit signed value: -8192 to 8191
	bendDeflection := int16(payload[0])<<8 | int16(payload[1])

	if fd.originalPeriod[driveNum] == 0 {
		return
	}

	// Bend by BendOctaves at full deflection.
	// Doubling frequency = halving period, so divide by 2^(octaves * deflection/8192).
	divisor := math.Pow(2.0, BendOctaves*float64(bendDeflection)/8192.0)
	fd.currentPeriod[driveNum] = uint16(float64(fd.originalPeriod[driveNum]) / divisor)
}

// haltAllDrives immediately stops all notes.
func (fd *FloppyDrives) haltAllDrives() {
	for d := byte(firstDrive); d <= lastDrive; d++ {
		fd.currentPeriod[d] = 0
	}
}

// reset returns a single drive's head to position 0.
func (fd *FloppyDrives) reset(driveNum byte) {
	fd.currentPeriod[driveNum] = 0

	// Step backwards to position 0.
	fd.dirPins[driveNum].High()
	for s := uint16(0); s < fd.maxPosition[driveNum]; s += 2 {
		fd.stepPins[driveNum].High()
		fd.stepPins[driveNum].Low()
		time.Sleep(5 * time.Millisecond)
	}

	fd.currentPosition[driveNum] = 0
	fd.stepState[driveNum] = false
	fd.dirPins[driveNum].Low()
	fd.directionState[driveNum] = false
	fd.setMovement(driveNum, true)
}

// ResetAll returns all drives to position 0 simultaneously.
func (fd *FloppyDrives) ResetAll() {
	// Stop all drives and set direction to reverse.
	for d := byte(firstDrive); d <= lastDrive; d++ {
		fd.currentPeriod[d] = 0
		fd.dirPins[d].High()
	}

	// Step all drives back together.
	for s := uint16(0); s < config.MaxPosition; s += 2 {
		for d := byte(firstDrive); d <= lastDrive; d++ {
			fd.stepPins[d].High()
			fd.stepPins[d].Low()
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Reset all tracking state.
	for d := byte(firstDrive); d <= lastDrive; d++ {
		fd.currentPosition[d] = 0
		fd.stepState[d] = false
		fd.dirPins[d].Low()
		fd.directionState[d] = false
		fd.setMovement(d, true)
	}
}

// setMovement enables or restricts head movement for a drive.
// When disabled, the head is constrained to a tiny range around the center.
func (fd *FloppyDrives) setMovement(driveNum byte, enabled bool) {
	if enabled {
		fd.minPosition[driveNum] = 0
		fd.maxPosition[driveNum] = config.MaxPosition
	} else {
		fd.minPosition[driveNum] = 79
		fd.maxPosition[driveNum] = 81
	}
}

// startupSound plays a short confirmation tune on a single drive.
func (fd *FloppyDrives) startupSound(driveNum byte) {
	chargeNotes := [5]uint16{
		notes.NoteDoubleTicks[31], // G1
		notes.NoteDoubleTicks[36], // C2
		notes.NoteDoubleTicks[38], // D2
		notes.NoteDoubleTicks[43], // G2
		0,                         // silence
	}

	var lastRun time.Time
	for i := 0; i < 5; {
		now := time.Now()
		if now.Sub(lastRun) > 200*time.Millisecond {
			lastRun = now
			fd.currentPeriod[driveNum] = chargeNotes[i]
			i++
		}
	}
}
