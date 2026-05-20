// Package smf is a minimal Standard MIDI File (SMF) reader.
//
// It supports formats 0/1/2 with PPQN (metric) division, decodes channel
// note-on/note-off and tempo meta events, and ignores everything else.
// The output is a flat, time-sorted slice of NoteEvents with absolute
// nanosecond offsets from the start of playback — exactly what the host
// CLI needs to drive the floppies.
package smf

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// EventKind distinguishes note on/off in the flat event stream.
type EventKind int

const (
	EventNoteOn EventKind = iota
	EventNoteOff
)

// NoteEvent is a flattened, absolutely-timed note event.
type NoteEvent struct {
	At       time.Duration // offset from start of song
	Kind     EventKind
	Channel  byte // 0..15
	Note     byte // MIDI note number 0..127
	Velocity byte // 0..127 (NOTE_OFF is velocity 0 here)
}

// Read parses an SMF file at path.
func Read(path string) ([]NoteEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse parses an SMF stream.
func Parse(r io.Reader) ([]NoteEvent, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	p := &parser{buf: data}
	return p.parse()
}

type parser struct {
	buf []byte
	pos int
}

type rawEvent struct {
	track    int
	absTicks uint64
	order    int
	// One of these is set:
	noteOn       bool
	noteOff      bool
	tempoChange  bool
	tempoUsPerQN uint32
	channel      byte
	note         byte
	velocity     byte
}

func (p *parser) parse() ([]NoteEvent, error) {
	if len(p.buf) < 14 {
		return nil, fmt.Errorf("smf: file too small")
	}
	if string(p.buf[0:4]) != "MThd" {
		return nil, fmt.Errorf("smf: missing MThd header")
	}
	headerLen := binary.BigEndian.Uint32(p.buf[4:8])
	if headerLen < 6 {
		return nil, fmt.Errorf("smf: bad header length %d", headerLen)
	}
	division := binary.BigEndian.Uint16(p.buf[12:14])
	if division&0x8000 != 0 {
		return nil, fmt.Errorf("smf: SMPTE timing not supported")
	}
	ppqn := uint32(division)
	if ppqn == 0 {
		return nil, fmt.Errorf("smf: invalid PPQN 0")
	}

	p.pos = 8 + int(headerLen)

	var raws []rawEvent
	trackIdx := 0
	order := 0
	for p.pos < len(p.buf) {
		if p.pos+8 > len(p.buf) {
			break
		}
		chunkID := string(p.buf[p.pos : p.pos+4])
		chunkLen := binary.BigEndian.Uint32(p.buf[p.pos+4 : p.pos+8])
		p.pos += 8
		end := p.pos + int(chunkLen)
		if end > len(p.buf) {
			return nil, fmt.Errorf("smf: chunk %q overflows file", chunkID)
		}
		if chunkID == "MTrk" {
			if err := p.parseTrack(p.buf[p.pos:end], trackIdx, &raws, &order); err != nil {
				return nil, fmt.Errorf("track %d: %w", trackIdx, err)
			}
			trackIdx++
		}
		p.pos = end
	}

	// Stable sort by absolute tick, then by track/order for determinism.
	sort.SliceStable(raws, func(i, j int) bool {
		if raws[i].absTicks != raws[j].absTicks {
			return raws[i].absTicks < raws[j].absTicks
		}
		if raws[i].track != raws[j].track {
			return raws[i].track < raws[j].track
		}
		return raws[i].order < raws[j].order
	})

	// Walk events accumulating real time as tempo changes.
	const defaultTempo uint32 = 500000 // µs per quarter (120 BPM)
	tempo := defaultTempo
	var (
		out          []NoteEvent
		lastTicks    uint64
		curTime      time.Duration
		usPerTick    = float64(tempo) / float64(ppqn)
	)
	for _, ev := range raws {
		dt := ev.absTicks - lastTicks
		curTime += time.Duration(float64(dt)*usPerTick) * time.Microsecond
		lastTicks = ev.absTicks
		switch {
		case ev.tempoChange:
			tempo = ev.tempoUsPerQN
			usPerTick = float64(tempo) / float64(ppqn)
		case ev.noteOn:
			out = append(out, NoteEvent{
				At: curTime, Kind: EventNoteOn,
				Channel: ev.channel, Note: ev.note, Velocity: ev.velocity,
			})
		case ev.noteOff:
			out = append(out, NoteEvent{
				At: curTime, Kind: EventNoteOff,
				Channel: ev.channel, Note: ev.note,
			})
		}
	}
	return out, nil
}

func (p *parser) parseTrack(track []byte, trackIdx int, out *[]rawEvent, order *int) error {
	var (
		pos      int
		absTicks uint64
		running  byte
	)
	for pos < len(track) {
		delta, n, err := readVarLen(track[pos:])
		if err != nil {
			return err
		}
		pos += n
		absTicks += uint64(delta)

		if pos >= len(track) {
			return fmt.Errorf("unexpected end of track")
		}
		status := track[pos]

		if status == 0xFF {
			// Meta event
			pos++
			if pos >= len(track) {
				return fmt.Errorf("truncated meta event")
			}
			metaType := track[pos]
			pos++
			length, n, err := readVarLen(track[pos:])
			if err != nil {
				return err
			}
			pos += n
			if pos+int(length) > len(track) {
				return fmt.Errorf("truncated meta data")
			}
			data := track[pos : pos+int(length)]
			pos += int(length)
			if metaType == 0x51 && len(data) == 3 {
				us := uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
				*out = append(*out, rawEvent{
					track: trackIdx, absTicks: absTicks, order: *order,
					tempoChange: true, tempoUsPerQN: us,
				})
				*order++
			}
			// 0x2F end-of-track and others: ignore
			continue
		}

		if status == 0xF0 || status == 0xF7 {
			// SysEx: skip its var-len-prefixed body
			pos++
			length, n, err := readVarLen(track[pos:])
			if err != nil {
				return err
			}
			pos += n + int(length)
			continue
		}

		// Channel message (with running status support).
		if status&0x80 == 0 {
			// Data byte; reuse running status, don't advance.
			if running == 0 {
				return fmt.Errorf("running status with no prior status")
			}
			status = running
		} else {
			running = status
			pos++
		}

		hi := status & 0xF0
		ch := status & 0x0F
		switch hi {
		case 0x80, 0x90, 0xA0, 0xB0, 0xE0:
			// Two data bytes
			if pos+2 > len(track) {
				return fmt.Errorf("truncated 2-byte channel msg")
			}
			d1 := track[pos]
			d2 := track[pos+1]
			pos += 2
			switch hi {
			case 0x90:
				if d2 == 0 {
					*out = append(*out, rawEvent{
						track: trackIdx, absTicks: absTicks, order: *order,
						noteOff: true, channel: ch, note: d1,
					})
				} else {
					*out = append(*out, rawEvent{
						track: trackIdx, absTicks: absTicks, order: *order,
						noteOn: true, channel: ch, note: d1, velocity: d2,
					})
				}
				*order++
			case 0x80:
				*out = append(*out, rawEvent{
					track: trackIdx, absTicks: absTicks, order: *order,
					noteOff: true, channel: ch, note: d1,
				})
				*order++
			}
		case 0xC0, 0xD0:
			if pos+1 > len(track) {
				return fmt.Errorf("truncated 1-byte channel msg")
			}
			pos++
		default:
			return fmt.Errorf("unknown status byte 0x%02X", status)
		}
	}
	return nil
}

func readVarLen(b []byte) (value uint32, n int, err error) {
	for i := 0; i < 4 && i < len(b); i++ {
		value = (value << 7) | uint32(b[i]&0x7F)
		if b[i]&0x80 == 0 {
			return value, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("variable-length quantity overran")
}
