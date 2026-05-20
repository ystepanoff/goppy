package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/ystepanoff/goppy/internal/protocol"
	"github.com/ystepanoff/goppy/internal/smf"
	"go.bug.st/serial"
)

func cmdPlay(args []string) error {
	fs := flag.NewFlagSet("play", flag.ExitOnError)
	pf := addPortFlags(fs)
	device := fs.Uint("device", 0x01, "target device address")
	minDrive := fs.Uint("min-drive", 1, "first drive sub-address available for note allocation")
	maxDrive := fs.Uint("max-drive", 8, "last drive sub-address available for note allocation")
	noPing := fs.Bool("no-ping", false, "skip the discovery ping before playback")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: goppy play [flags] <song.mid>")
	}
	if *minDrive == 0 || *maxDrive < *minDrive || *maxDrive > 255 {
		return fmt.Errorf("invalid drive range %d..%d", *minDrive, *maxDrive)
	}
	path := fs.Arg(0)

	events, err := smf.Read(path)
	if err != nil {
		return fmt.Errorf("read midi: %w", err)
	}
	if len(events) == 0 {
		return fmt.Errorf("midi file contains no note events")
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At < events[j].At })

	port, err := pf.open()
	if err != nil {
		return err
	}
	defer port.Close()

	if !*noPing {
		if err := pingAndPrint(port); err != nil {
			fmt.Fprintln(os.Stderr, "warning: ping failed:", err)
		}
	}

	dev := byte(*device)
	allocator := newDriveAllocator(byte(*minDrive), byte(*maxDrive))

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	if _, err := port.Write(protocol.SequenceStart()); err != nil {
		return fmt.Errorf("send SEQ_START: %w", err)
	}
	defer func() {
		_, _ = port.Write(protocol.SequenceStop())
		_, _ = port.Write(protocol.Reset())
	}()

	fmt.Printf("playing %s — %d events, %s long\n",
		path, len(events), events[len(events)-1].At.Round(time.Millisecond))

	start := time.Now()
	for i, ev := range events {
		select {
		case <-stopCh:
			fmt.Fprintln(os.Stderr, "interrupted, stopping")
			return nil
		default:
		}

		wait := ev.At - time.Since(start)
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-stopCh:
				timer.Stop()
				fmt.Fprintln(os.Stderr, "interrupted, stopping")
				return nil
			}
		}

		switch ev.Kind {
		case smf.EventNoteOn:
			drive, ok := allocator.assign(ev.Channel, ev.Note)
			if !ok {
				continue
			}
			if _, err := port.Write(protocol.NoteOn(dev, drive, ev.Note)); err != nil {
				return fmt.Errorf("event %d: write NOTE_ON: %w", i, err)
			}
		case smf.EventNoteOff:
			drive, ok := allocator.release(ev.Channel, ev.Note)
			if !ok {
				continue
			}
			if _, err := port.Write(protocol.NoteOff(dev, drive)); err != nil {
				return fmt.Errorf("event %d: write NOTE_OFF: %w", i, err)
			}
		}
	}
	return nil
}

func pingAndPrint(port serial.Port) error {
	if err := port.SetReadTimeout(2 * time.Second); err != nil {
		return err
	}
	if _, err := port.Write(protocol.Ping()); err != nil {
		return err
	}
	pong, err := protocol.ReadPong(port)
	if err != nil {
		return err
	}
	fmt.Printf("device 0x%02X drives %d..%d ready\n",
		pong.DeviceAddress, pong.MinSubAddress, pong.MaxSubAddress)
	return port.SetReadTimeout(serial.NoTimeout)
}

// driveAllocator hands out drive sub-addresses for active notes.
// Each (channel, note) maps to at most one drive; round-robin across the
// configured range when notes overlap. If all drives are in use, new notes
// are dropped silently.
type driveAllocator struct {
	min, max byte
	next     byte
	active   map[uint16]byte
	used     map[byte]bool
}

func newDriveAllocator(min, max byte) *driveAllocator {
	return &driveAllocator{
		min:    min,
		max:    max,
		next:   min,
		active: make(map[uint16]byte),
		used:   make(map[byte]bool),
	}
}

func noteKey(channel, note byte) uint16 { return uint16(channel)<<8 | uint16(note) }

func (a *driveAllocator) assign(channel, note byte) (byte, bool) {
	k := noteKey(channel, note)
	if d, ok := a.active[k]; ok {
		return d, true
	}
	for i := 0; i <= int(a.max-a.min); i++ {
		d := a.next
		a.next++
		if a.next > a.max {
			a.next = a.min
		}
		if !a.used[d] {
			a.used[d] = true
			a.active[k] = d
			return d, true
		}
	}
	return 0, false
}

func (a *driveAllocator) release(channel, note byte) (byte, bool) {
	k := noteKey(channel, note)
	d, ok := a.active[k]
	if !ok {
		return 0, false
	}
	delete(a.active, k)
	delete(a.used, d)
	return d, true
}
