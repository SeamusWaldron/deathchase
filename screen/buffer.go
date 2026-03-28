package screen

// Buffer emulates the ZX Spectrum display file and attribute memory.
//
// Display file: $4000-$57FF (6144 bytes) — pixel data
// Attribute file: $5800-$5AFF (768 bytes) — colour attributes
//
// The ZX Spectrum display has a non-linear layout:
//   Address format: 010TTSSS RRRCCCCC
//   TT = third (0-2), SSS = scan line within char row (0-7)
//   RRR = character row within third (0-7), CCCCC = column (0-31)
//
// Attribute format: FBPPPIII
//   F = flash, B = bright, PPP = paper colour, III = ink colour

const (
	DisplayStart = 0x4000
	DisplaySize  = 6144
	AttrStart    = 0x5800
	AttrSize     = 768
	ScreenWidth  = 256 // pixels
	ScreenHeight = 192 // pixels
	Cols         = 32  // character columns
	Rows         = 24  // character rows
)

type Buffer struct {
	Display [DisplaySize]byte // Pixel data ($4000-$57FF)
	Attrs   [AttrSize]byte    // Attributes ($5800-$5AFF)
}

func NewBuffer() *Buffer {
	return &Buffer{}
}

// Clear zeros the display and sets all attributes to the given value.
func (b *Buffer) Clear(attr byte) {
	for i := range b.Display {
		b.Display[i] = 0
	}
	for i := range b.Attrs {
		b.Attrs[i] = attr
	}
}

// Peek reads a byte from the emulated memory at the given ZX Spectrum address.
func (b *Buffer) Peek(addr uint16) byte {
	if addr >= DisplayStart && addr < DisplayStart+DisplaySize {
		return b.Display[addr-DisplayStart]
	}
	if addr >= AttrStart && addr < AttrStart+AttrSize {
		return b.Attrs[addr-AttrStart]
	}
	return 0
}

// Poke writes a byte to the emulated memory at the given ZX Spectrum address.
func (b *Buffer) Poke(addr uint16, val byte) {
	if addr >= DisplayStart && addr < DisplayStart+DisplaySize {
		b.Display[addr-DisplayStart] = val
	}
	if addr >= AttrStart && addr < AttrStart+AttrSize {
		b.Attrs[addr-AttrStart] = val
	}
}

// DisplayAddr returns the ZX Spectrum display file address for a given
// pixel coordinate (x in 0..255, y in 0..191).
func DisplayAddr(x, y int) uint16 {
	// 010TTSSS RRRCCCCC
	third := y / 64
	charRow := (y % 64) / 8
	scanLine := y % 8
	col := x / 8
	hi := 0x40 | (third << 3) | scanLine
	lo := (charRow << 5) | col
	return uint16(hi)<<8 | uint16(lo)
}

// AttrAddr returns the ZX Spectrum attribute address for a character cell.
func AttrAddr(col, row int) uint16 {
	return AttrStart + uint16(row)*Cols + uint16(col)
}

// AttrAddrFromPixel returns the attribute address for a pixel coordinate.
func AttrAddrFromPixel(x, y int) uint16 {
	return AttrAddr(x/8, y/8)
}

// DisplayAddrToAttr converts a display file address to its corresponding
// attribute address.
func DisplayAddrToAttr(displayAddr uint16) uint16 {
	offset := displayAddr - DisplayStart
	// Extract third and char row
	hi := byte(offset >> 8)
	lo := byte(offset)
	third := (hi >> 3) & 0x03
	charRow := (lo >> 5) & 0x07
	col := lo & 0x1F
	row := third*8 + charRow
	return AttrStart + uint16(row)*Cols + uint16(col)
}

// DrawByte writes a byte of pixel data at the given display address using OR mode.
func (b *Buffer) DrawByteOR(addr uint16, val byte) {
	if addr >= DisplayStart && addr < DisplayStart+DisplaySize {
		b.Display[addr-DisplayStart] |= val
	}
}

// DrawByteOverwrite writes a byte of pixel data at the given display address.
func (b *Buffer) DrawByteOverwrite(addr uint16, val byte) {
	if addr >= DisplayStart && addr < DisplayStart+DisplaySize {
		b.Display[addr-DisplayStart] = val
	}
}

// DrawSprite8 draws an 8-pixel-wide sprite (1 byte per row) at the given
// display address, using OR mode. Returns the display address after the last row.
func (b *Buffer) DrawSprite8OR(addr uint16, data []byte) {
	for _, row := range data {
		b.DrawByteOR(addr, row)
		addr = NextScanLine(addr)
	}
}

// NextScanLine returns the display address of the next scan line down
// from the given address, handling the ZX Spectrum's interleaved layout.
func NextScanLine(addr uint16) uint16 {
	// Increment the high byte (scan line within character)
	addr += 0x0100
	if addr&0x0700 != 0 {
		return addr // Still within the same character row
	}
	// Wrapped past scan line 7 — move to next character row
	addr -= 0x0700          // Reset scan line bits
	addr += 0x0020          // Move to next character row
	if addr&0x00E0 != 0 {   // Not zero means still in same third
		return addr
	}
	// Wrapped past character row 7 — but we've already done the +0x20
	// so we're fine (moved to next third)
	return addr
}
