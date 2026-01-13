// Package config contains all configuration constants.
//
// These constants control:
// - Device identity (address in multi-device setups)
// - Hardware limits (number of drives, pin mappings)
// - Timing precision (timer resolution for sound generation)
// - Communication settings (serial baud rate, buffer sizes)
// - Protocol constants (message framing bytes)
package config

// =============================================================================
// DEVICE IDENTITY
// =============================================================================

// DeviceAddress is this device's address on the Moppy network.
// When the controller sends a message, it includes a target address.
// Only messages matching our DeviceAddress (or broadcast address 0x00) are processed.
// In a multi-Arduino setup, each Arduino has a unique address (0x01, 0x02, etc.)
const DeviceAddress byte = 0x01

// MinSubAddress is the lowest drive number this device responds to.
// Sub-addresses identify individual drives within a device.
// For an 8-drive setup, drives are numbered 1-8.
const MinSubAddress byte = 1

// MaxSubAddress is the highest drive number this device responds to.
// Together with MinSubAddress, this defines which drives this device controls.
const MaxSubAddress byte = 8

// =============================================================================
// HARDWARE CONFIGURATION
// =============================================================================

// NumDrives is the total number of floppy drives connected.
// Each drive needs 2 pins: one for STEP (pulse to move head) and one for DIRECTION.
// 8 drives = 16 pins, which fits on an Arduino Uno (pins 2-17).
const NumDrives = 8

// MaxPosition is the maximum head position for a 3.5" floppy drive.
// 3.5" drives have 80 tracks, and the head can step 158 times (0-158).
// When the head reaches 0 or MaxPosition, it bounces back (reverses direction).
// This bouncing is what creates the characteristic "floppy drive music" sound.
const MaxPosition = 158

// FirstPin is the first Arduino pin used for drive control.
// Pins 0-1 are reserved for Serial communication (TX/RX), so we start at pin 2.
// Pin mapping formula:
//   Drive N (1-8) uses:
//     - Step pin:      FirstPin + (N-1)*2     = 2, 4, 6, 8, 10, 12, 14, 16
//     - Direction pin: FirstPin + (N-1)*2 + 1 = 3, 5, 7, 9, 11, 13, 15, 17
const FirstPin = 2

// =============================================================================
// TIMING CONFIGURATION
// =============================================================================

// TimerResolution is the timer interrupt interval in microseconds.
// This determines how often the tick() function runs to update pin states.
//
// Why 40µs?
// - Lower values = more precise timing but higher CPU load
// - Higher values = less CPU load but notes may sound "off"
// - 40µs gives 25,000 interrupts/second, which is enough for musical precision
// - The highest playable note (~2093 Hz for C7) needs toggling every ~239µs
// - 40µs gives us ~6 toggles per wave cycle at the highest frequency
const TimerResolution = 40

// =============================================================================
// SERIAL COMMUNICATION
// =============================================================================

// SerialBaudRate is the speed for USB serial communication with the controller.
// 57600 baud is the standard Moppy protocol speed.
// At 57600 baud: ~5760 bytes/second, or ~174µs per byte.
const SerialBaudRate = 57600

// MessageBufferSize is the maximum size of an incoming message.
// Moppy messages are small (typically 5-10 bytes), but we allow headroom.
// Format: [START][ADDR][SUB][SIZE][CMD][...PAYLOAD...]
// Maximum payload is 255 bytes, plus 4 header bytes = 259 max.
const MessageBufferSize = 259

// =============================================================================
// MOPPY PROTOCOL CONSTANTS
// =============================================================================

// StartByte marks the beginning of every Moppy message.
// 0x4D = ASCII 'M' for "Moppy"
// The receiver scans for this byte to synchronise with the message stream.
const StartByte byte = 0x4D

// SystemAddress is used for broadcast/system-wide messages.
// Messages to address 0x00 are processed by ALL devices on the network.
// Used for commands like RESET, PING, SEQUENCE_START/STOP.
const SystemAddress byte = 0x00

// =============================================================================
// SYSTEM COMMANDS (sent to SystemAddress 0x00)
// =============================================================================

// These commands affect all devices or request system-wide actions.

// CmdPing requests all devices to respond with a Pong.
// Used by the controller to discover connected devices.
const CmdPing byte = 0x80

// CmdPong is the response to a Ping.
// Contains: [PONG, DeviceAddress, MinSubAddress, MaxSubAddress]
// Tells the controller what drives this device controls.
const CmdPong byte = 0x81

// CmdSequenceStart signals that music playback is beginning.
// Devices can use this to prepare (e.g., enable outputs).
const CmdSequenceStart byte = 0xFA

// CmdSequenceStop signals that music playback has stopped.
// Devices should silence all notes and optionally reset.
const CmdSequenceStop byte = 0xFC

// CmdReset tells all devices to reset to initial state.
// Drives return heads to position 0, all notes stop.
const CmdReset byte = 0xFF

// =============================================================================
// DEVICE COMMANDS (sent to specific DeviceAddress)
// =============================================================================

// These commands control individual drives within a device.

// DevCmdReset resets a specific drive (sub-address).
const DevCmdReset byte = 0x00

// DevCmdNoteOff stops the note playing on a drive.
// The drive stops stepping and goes silent.
const DevCmdNoteOff byte = 0x08

// DevCmdNoteOn starts playing a note on a drive.
// Payload: [note_number] where note_number is MIDI note 0-127.
// MIDI note 60 = Middle C (261.63 Hz)
const DevCmdNoteOn byte = 0x09

// DevCmdBendPitch applies pitch bend to the currently playing note.
// Payload: [bend_MSB, bend_LSB] - 14-bit value, center = 8192.
// Allows smooth pitch slides and vibrato effects.
const DevCmdBendPitch byte = 0x0E

// =============================================================================
// FEATURE FLAGS
// =============================================================================

// PlayStartupSound determines whether to play a short tune on boot.
// This confirms that all drives are working and helps with debugging.
// Set to false for silent startup.
const PlayStartupSound = true
