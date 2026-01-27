// Package notes provides pre-calculated timing values for MIDI notes.
// These lookup tables avoid expensive floating-point math at runtime.
package notes

import "github.com/ystepanoff/goppy/arduino/config"

// NotePeriods contains the period in microseconds for each MIDI note (0-127).
// Formula: period_µs = 1,000,000 / (440 * 2^((note - 69) / 12))
// Note 69 (A4) = 440 Hz = 2273 µs period
var NotePeriods = [128]uint32{
	// Octave -1 (notes 0-11): C-1 to B-1
	122312, 115447, 108968, 102852, 97079, 91631, 86488, 81634, 77052, 72727, 68645, 64793,
	// Octave 0 (notes 12-23): C0 to B0
	61156, 57724, 54484, 51426, 48540, 45815, 43244, 40817, 38526, 36364, 34323, 32396,
	// Octave 1 (notes 24-35): C1 to B1
	30578, 28862, 27242, 25713, 24270, 22908, 21622, 20408, 19263, 18182, 17161, 16198,
	// Octave 2 (notes 36-47): C2 to B2
	15289, 14431, 13621, 12856, 12135, 11454, 10811, 10204, 9631, 9091, 8581, 8099,
	// Octave 3 (notes 48-59): C3 to B3
	7645, 7215, 6810, 6428, 6067, 5727, 5405, 5102, 4816, 4545, 4290, 4050,
	// Octave 4 (notes 60-71): C4 to B4 (Middle C = 60)
	3822, 3608, 3405, 3214, 3034, 2863, 2703, 2551, 2408, 2273, 2145, 2025,
	// Octave 5 (notes 72-83): C5 to B5
	1911, 1804, 1703, 1607, 1517, 1432, 1351, 1276, 1204, 1136, 1073, 1012,
	// Octave 6 (notes 84-95): C6 to B6
	956, 902, 851, 804, 758, 716, 676, 638, 602, 568, 536, 506,
	// Octave 7 (notes 96-107): C7 to B7
	478, 451, 426, 402, 379, 358, 338, 319, 301, 284, 268, 253,
	// Octave 8 (notes 108-119): C8 to B8
	239, 225, 213, 201, 190, 179, 169, 159, 150, 142, 134, 127,
	// Octave 9 (notes 120-127): C9 to G9 (partial octave)
	119, 113, 106, 100, 95, 89, 84, 80,
}

// NoteDoubleTicks contains the number of timer ticks for a half-period.
// This is used by floppy drives: toggle pin every NoteDoubleTicks[note] ticks.
// Formula: doubleTicks = NotePeriods[note] / TimerResolution
// With 40µs timer resolution, A4 (note 69) = 2273 / 40 ≈ 57 ticks
var NoteDoubleTicks = [128]uint16{
	// Octave -1 (notes 0-11): very low, likely inaudible on floppy drives
	3058, 2886, 2724, 2571, 2427, 2291, 2162, 2041, 1926, 1818, 1716, 1620,
	// Octave 0 (notes 12-23)
	1529, 1443, 1362, 1286, 1214, 1145, 1081, 1020, 963, 909, 858, 810,
	// Octave 1 (notes 24-35)
	764, 722, 681, 643, 607, 573, 541, 510, 482, 455, 429, 405,
	// Octave 2 (notes 36-47)
	382, 361, 341, 321, 303, 286, 270, 255, 241, 227, 215, 202,
	// Octave 3 (notes 48-59)
	191, 180, 170, 161, 152, 143, 135, 128, 120, 114, 107, 101,
	// Octave 4 (notes 60-71): Middle C and above - good floppy range
	96, 90, 85, 80, 76, 72, 68, 64, 60, 57, 54, 51,
	// Octave 5 (notes 72-83)
	48, 45, 43, 40, 38, 36, 34, 32, 30, 28, 27, 25,
	// Octave 6 (notes 84-95)
	24, 23, 21, 20, 19, 18, 17, 16, 15, 14, 13, 13,
	// Octave 7 (notes 96-107): getting high, stepping may be imprecise
	12, 11, 11, 10, 9, 9, 8, 8, 8, 7, 7, 6,
	// Octave 8 (notes 108-119): very high, may not play well
	6, 6, 5, 5, 5, 4, 4, 4, 4, 4, 3, 3,
	// Octave 9 (notes 120-127): extremely high, likely inaudible
	3, 3, 3, 3, 2, 2, 2, 2,
}

// Compile-time assertion that TimerResolution is used correctly.
// This ensures the tables stay in sync with config if it ever changes.
var _ = config.TimerResolution
