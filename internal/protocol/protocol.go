// Package protocol encodes and decodes Moppy v2 wire frames for the host side.
//
// Frame layout (matches the firmware in goppy/arduino):
//
//	[START=0x4D] [DEVICE_ADDR] [SUB_ADDR] [SIZE] [COMMAND] [PAYLOAD...]
//
// SIZE counts the bytes that follow it (command + payload).
package protocol

import (
	"errors"
	"fmt"
	"io"
)

const (
	StartByte     byte = 0x4D
	SystemAddress byte = 0x00
)

// System commands (sent to SystemAddress).
const (
	CmdPing          byte = 0x80
	CmdPong          byte = 0x81
	CmdSequenceStart byte = 0xFA
	CmdSequenceStop  byte = 0xFC
	CmdReset         byte = 0xFF
)

// Device commands (sent to a specific device address + sub address).
const (
	DevCmdReset       byte = 0x00
	DevCmdNoteOff     byte = 0x08
	DevCmdNoteOn      byte = 0x09
	DevCmdBendPitch   byte = 0x0E
	DevCmdSetMovement byte = 0x64
)

// PitchBendCenter is the neutral pitch-bend value (no bend).
const PitchBendCenter uint16 = 8192

// Pong is the decoded reply to a PING.
type Pong struct {
	DeviceAddress byte
	MinSubAddress byte
	MaxSubAddress byte
}

// EncodeFrame builds a complete Moppy frame.
// payload may be nil; the command byte itself counts toward SIZE.
func EncodeFrame(deviceAddr, subAddr, command byte, payload []byte) []byte {
	size := byte(1 + len(payload))
	frame := make([]byte, 0, 4+int(size))
	frame = append(frame, StartByte, deviceAddr, subAddr, size, command)
	frame = append(frame, payload...)
	return frame
}

// System helpers ------------------------------------------------------------

func Ping() []byte {
	return EncodeFrame(SystemAddress, 0x00, CmdPing, nil)
}

func Reset() []byte {
	return EncodeFrame(SystemAddress, 0x00, CmdReset, nil)
}

func SequenceStart() []byte {
	return EncodeFrame(SystemAddress, 0x00, CmdSequenceStart, nil)
}

func SequenceStop() []byte {
	return EncodeFrame(SystemAddress, 0x00, CmdSequenceStop, nil)
}

// Device helpers ------------------------------------------------------------

func NoteOn(deviceAddr, subAddr, note byte) []byte {
	return EncodeFrame(deviceAddr, subAddr, DevCmdNoteOn, []byte{note})
}

func NoteOff(deviceAddr, subAddr byte) []byte {
	return EncodeFrame(deviceAddr, subAddr, DevCmdNoteOff, nil)
}

func DriveReset(deviceAddr, subAddr byte) []byte {
	return EncodeFrame(deviceAddr, subAddr, DevCmdReset, nil)
}

// PitchBend encodes a 14-bit bend value (0..16383, center 8192).
func PitchBend(deviceAddr, subAddr byte, bend uint16) []byte {
	if bend > 0x3FFF {
		bend = 0x3FFF
	}
	msb := byte((bend >> 7) & 0x7F)
	lsb := byte(bend & 0x7F)
	return EncodeFrame(deviceAddr, subAddr, DevCmdBendPitch, []byte{msb, lsb})
}

// SetMovement clamps (clamp=true) head travel to a 2-track wiggle, or
// restores the full 0-158 range (clamp=false). Polarity matches the firmware.
func SetMovement(deviceAddr, subAddr byte, clamp bool) []byte {
	flag := byte(0)
	if clamp {
		flag = 1
	}
	return EncodeFrame(deviceAddr, subAddr, DevCmdSetMovement, []byte{flag})
}

// ReadPong reads bytes from r until it parses a Pong reply or EOF.
// It tolerates noise bytes between frames and ignores frames that aren't Pongs.
func ReadPong(r io.Reader) (Pong, error) {
	var (
		buf   [1]byte
		state int
		hdr   [4]byte // START, DEVICE, SUB, SIZE
	)
	for {
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return Pong{}, err
		}
		b := buf[0]
		switch state {
		case 0:
			if b == StartByte {
				hdr[0] = b
				state = 1
			}
		case 1:
			hdr[1] = b
			state = 2
		case 2:
			hdr[2] = b
			state = 3
		case 3:
			hdr[3] = b
			size := int(b)
			if size < 1 {
				state = 0
				continue
			}
			body := make([]byte, size)
			if _, err := io.ReadFull(r, body); err != nil {
				return Pong{}, err
			}
			state = 0
			if hdr[1] != SystemAddress || body[0] != CmdPong {
				continue
			}
			if size < 4 {
				return Pong{}, fmt.Errorf("pong payload too short: %d", size)
			}
			return Pong{
				DeviceAddress: body[1],
				MinSubAddress: body[2],
				MaxSubAddress: body[3],
			}, nil
		}
	}
}

// ErrNoPong is returned when the device fails to respond to a PING in time.
var ErrNoPong = errors.New("no pong received")
