// Timer configuration for Arduino Uno (ATmega328P).
//
// Moppy needs a hardware timer firing at exactly TimerResolution µs (40 µs
// = 25 kHz by default). At this rate software timing is not viable - we
// must use Timer1 in CTC mode.
//
// Hardware setup on ATmega328P @ 16 MHz:
//   - Prescaler 1 -> 16 timer ticks per µs
//   - Timer1 is 16-bit, supports CTC (Clear Timer on Compare Match)
//   - OCR1A holds the compare value; counter resets to 0 on match
//   - Compare match raises the TIMER1_COMPA interrupt
//
// For 40 µs interval: OCR1A = 16 * 40 - 1 = 639 (fits in 16 bits).

//go:build avr

package instruments

import (
	"device/avr"
	"runtime/interrupt"
)

// cpuFrequencyMHz is the AVR clock rate. Arduino Uno runs at 16 MHz.
const cpuFrequencyMHz = 16

// timerCallback is invoked from the Timer1 compare-match ISR.
// Stored at package scope because the ISR cannot capture closures.
var timerCallback func()

// InitTimer configures Timer1 to fire callback every microseconds µs.
//
// Uses CTC mode with prescaler 1, so the maximum interval at 16 MHz is
// (2^16 - 1) / 16 ≈ 4096 µs. Moppy's 40 µs target sits well in range.
// callback runs in interrupt context - keep it tight.
func InitTimer(microseconds uint32, callback func()) {
	timerCallback = callback

	// Stop the timer while we configure it.
	avr.TCCR1A.Set(0)
	avr.TCCR1B.Set(0)

	// Reset the counter. Write high byte first - AVR latches it for the
	// paired low-byte write.
	avr.TCNT1H.Set(0)
	avr.TCNT1L.Set(0)

	// Compare value: 16 ticks/µs * microseconds - 1.
	// TinyGo's TCCR1B_WGM10 alias is actually datasheet WGM12 (bit 3).
	ticks := uint16(cpuFrequencyMHz*microseconds - 1)
	avr.OCR1AH.Set(byte(ticks >> 8))
	avr.OCR1AL.Set(byte(ticks))

	// Wire the ISR before unmasking the interrupt.
	interrupt.New(avr.IRQ_TIMER1_COMPA, timerISR)
	avr.TIMSK1.SetBits(avr.TIMSK1_OCIE1A)

	// Start Timer1: CTC mode (WGM12 = 1), prescaler = 1 (CS10 = 1).
	avr.TCCR1B.Set(avr.TCCR1B_WGM10 | avr.TCCR1B_CS10)
}

// timerISR is the Timer1 Compare A interrupt handler.
// It dispatches to the registered callback.
func timerISR(interrupt.Interrupt) {
	if timerCallback != nil {
		timerCallback()
	}
}
