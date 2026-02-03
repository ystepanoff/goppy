// Package networks provides communication handlers for the Moppy protocol.
// This package implements serial (USB) communication with the Moppy controller.
package networks

import (
	"machine"

	"github.com/ystepanoff/goppy/arduino/config"
)

// =============================================================================
// MESSAGE CONSUMER INTERFACE
// =============================================================================

// MessageConsumer defines the interface for handling Moppy messages.
// Instruments (like FloppyDrives) implement this interface to receive commands.
type MessageConsumer interface {
	// HandleSystemMessage processes system-wide commands (sent to address 0x00).
	// These affect all devices: reset, sequence start/stop, etc.
	HandleSystemMessage(command byte, payload []byte)

	// HandleDeviceMessage processes device-specific commands.
	// subAddress identifies the specific drive (1-8).
	// command is the action (note on/off, pitch bend, etc.).
	// payload contains command-specific data.
	HandleDeviceMessage(subAddress byte, command byte, payload []byte)
}

// =============================================================================
// SERIAL HANDLER
// =============================================================================

// Serial handles USB serial communication with the Moppy controller.
// It reads incoming bytes, parses the Moppy protocol, and dispatches
// messages to a MessageConsumer (typically a FloppyDrives instance).
type Serial struct {
	consumer MessageConsumer

	// Message parsing state
	messagePos    int                            // Current position in message parsing state machine
	messageBuffer [config.MessageBufferSize]byte // Buffer for incoming message

	// Pre-built pong response
	// Format: [START][DEVICE=0x00][SUB=0x00][SIZE=4][PONG][ADDR][MIN][MAX]
	pongBytes [8]byte
}

// NewSerial creates a new Serial handler with the given message consumer.
func NewSerial(consumer MessageConsumer) *Serial {
	s := &Serial{
		consumer:   consumer,
		messagePos: 0,
	}

	// Pre-build the pong response bytes
	s.pongBytes = [8]byte{
		config.StartByte,
		config.SystemAddress,  // Device address (system)
		0x00,                  // Sub address
		0x04,                  // Size: 4 bytes follow
		config.CmdPong,        // Pong command
		config.DeviceAddress,  // Our device address
		config.MinSubAddress,  // First drive we control
		config.MaxSubAddress,  // Last drive we control
	}

	return s
}

// Begin initialises the serial port for Moppy communication.
// Must be called before ReadMessages.
func (s *Serial) Begin() {
	machine.Serial.Configure(machine.UARTConfig{
		BaudRate: config.SerialBaudRate,
	})
}

// =============================================================================
// MESSAGE READING STATE MACHINE
// =============================================================================

// ReadMessages reads and processes any available Moppy messages from serial.
// This should be called repeatedly in the main loop.
//
// Moppy message format:
//
//	Byte 0: START_BYTE (0x4D)
//	Byte 1: Device address (0x00 for system-wide)
//	Byte 2: Sub address (drive number, ignored for system messages)
//	Byte 3: Size of message body (bytes following this one)
//	Byte 4: Command byte
//	Byte 5+: Optional payload
//
// The state machine handles partial reads gracefully, allowing it to be
// called from a non-blocking main loop.
func (s *Serial) ReadMessages() {
	for s.processNextByte() {
		// Keep processing while there's data and we can make progress
	}
}

// processNextByte handles the next byte in the message parsing state machine.
// Returns true if processing should continue, false if we should wait for more data.
func (s *Serial) processNextByte() bool {
	// State 4 is special: we need to wait for the full payload
	if s.messagePos == 4 {
		payloadSize := int(s.messageBuffer[3])
		if machine.Serial.Buffered() < payloadSize {
			return false // Wait for full payload
		}
		s.readPayloadAndDispatch()
		return true
	}

	// For other states, we need at least one byte
	if machine.Serial.Buffered() == 0 {
		return false
	}

	// Read single byte for state machine progression
	var b [1]byte
	_, err := machine.Serial.Read(b[:])
	if err != nil {
		return false
	}

	switch s.messagePos {
	case 0:
		// State 0: Waiting for START_BYTE
		if b[0] == config.StartByte {
			s.messagePos = 1
		}
		// Otherwise, keep scanning for start byte

	case 1:
		// State 1: Read device address
		s.messageBuffer[1] = b[0]

		if b[0] == config.SystemAddress {
			// System messages are for everyone
			s.messagePos = 2
		} else if b[0] == config.DeviceAddress {
			// Message is for us
			s.messagePos = 2
		} else {
			// Not for us, reset
			s.messagePos = 0
		}

	case 2:
		// State 2: Read sub address (drive number)
		s.messageBuffer[2] = b[0]

		// Accept: 0x00 (all drives) or valid drive range
		if b[0] == 0x00 || (b[0] >= config.MinSubAddress && b[0] <= config.MaxSubAddress) {
			s.messagePos = 3
		} else {
			// Invalid sub address, reset
			s.messagePos = 0
		}

	case 3:
		// State 3: Read message body size
		s.messageBuffer[3] = b[0]
		s.messagePos = 4
	}

	return true
}

// readPayloadAndDispatch reads the command and payload, then dispatches to consumer.
func (s *Serial) readPayloadAndDispatch() {
	payloadSize := int(s.messageBuffer[3])

	// Read command byte and payload into buffer starting at position 4
	if payloadSize > 0 {
		machine.Serial.Read(s.messageBuffer[4 : 4+payloadSize])
	}

	// Dispatch based on message type
	if s.messageBuffer[1] == config.SystemAddress {
		// System message
		command := s.messageBuffer[4]
		if command == config.CmdPing {
			s.sendPong()
		} else {
			// Pass to consumer with payload (bytes after command)
			var payload []byte
			if payloadSize > 1 {
				payload = s.messageBuffer[5 : 4+payloadSize]
			}
			s.consumer.HandleSystemMessage(command, payload)
		}
	} else {
		// Device message
		subAddress := s.messageBuffer[2]
		command := s.messageBuffer[4]
		var payload []byte
		if payloadSize > 1 {
			payload = s.messageBuffer[5 : 4+payloadSize]
		}
		s.consumer.HandleDeviceMessage(subAddress, command, payload)
	}

	// Reset for next message
	s.messagePos = 0
}

// =============================================================================
// PONG RESPONSE
// =============================================================================

// sendPong sends a pong response to a ping request.
// This tells the controller what device address and drive range we handle.
func (s *Serial) sendPong() {
	machine.Serial.Write(s.pongBytes[:])
}
