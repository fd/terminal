// Copyright 2010 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package editline

import (
	"fmt"
	"unicode/utf8"
)

// Buffer size
var (
	BufferCap = 4096
	BufferLen = 64 // Initial length
)

// == Init

/*var lines, columns int

func init() {
	lines, columns = terminal.SizeInChar()
}*/

// == Type

// A buffer represents the line buffer.
type buffer struct {
	columns   int // Number of columns for actual window
	promptLen int
	pos       int    // Pointer position into buffer
	size      int    // Amount of characters added
	data      []rune // Text buffer
	echoRune  rune
}

func newBuffer(promptLen, columns int, echoRune rune) *buffer {
	b := new(buffer)

	b.columns = columns
	b.promptLen = promptLen
	b.echoRune = echoRune
	b.data = make([]rune, BufferLen, BufferCap)

	return b
}

// == Output

// insertRune inserts a character in the cursor position.
func (b *buffer) insertRune(r rune) error {
	var useRefresh bool

	b.grow(b.size + 1) // Check if there is free space for one more character

	// Avoid a full update of the line.
	if b.pos == b.size {
		if b.echoRune == 0 {
			char := make([]byte, utf8.UTFMax)
			utf8.EncodeRune(char, r)

			if _, err := Output.Write(char); err != nil {
				return outputError(err.Error())
			}
		} else {
			char := make([]byte, utf8.UTFMax)
			utf8.EncodeRune(char, b.echoRune)

			if _, err := Output.Write(char); err != nil {
				return outputError(err.Error())
			}
		}
	} else {
		useRefresh = true
		copy(b.data[b.pos+1:b.size+1], b.data[b.pos:b.size])
	}

	b.data[b.pos] = r
	b.pos++
	b.size++

	if useRefresh {
		return b.refresh()
	}
	return nil
}

// insertRunes inserts several characters.
func (b *buffer) insertRunes(runes []rune) error {
	for _, r := range runes {
		if err := b.insertRune(r); err != nil {
			return err
		}
	}
	return nil
}

// toBytes returns a slice of the contents of the buffer.
func (b *buffer) toDisplayBytes() []byte {
	if b.echoRune == 0 {
		return b.toBytes()
	}

	chars := make([]byte, b.size*utf8.UTFMax)
	var end, runeLen int

	// == Each character (as integer) is encoded to []byte
	for i := 0; i < b.size; i++ {
		if i != 0 {
			runeLen = utf8.EncodeRune(chars[end:], b.echoRune)
			end += runeLen
		} else {
			runeLen = utf8.EncodeRune(chars, b.echoRune)
			end = runeLen
		}
	}
	return chars[:end]
}

// toBytes returns a slice of the contents of the buffer.
func (b *buffer) toBytes() []byte {
	chars := make([]byte, b.size*utf8.UTFMax)
	var end, runeLen int

	// == Each character (as integer) is encoded to []byte
	for i := 0; i < b.size; i++ {
		if i != 0 {
			runeLen = utf8.EncodeRune(chars[end:], b.data[i])
			end += runeLen
		} else {
			runeLen = utf8.EncodeRune(chars, b.data[i])
			end = runeLen
		}
	}
	return chars[:end]
}

// toString returns the contents of the buffer as a string.
func (b *buffer) toString() string { return string(b.data[b.promptLen:b.size]) }

// refresh refreshes the line.
func (b *buffer) refresh() (err error) {
	lastLine, _ := b.pos2xy(b.size)
	posLine, posColumn := b.pos2xy(b.pos)

	// To the first line.
	for ln := posLine; ln > 0; ln-- {
		if _, err = Output.Write(toPreviousLine); err != nil {
			return outputError(err.Error())
		}
	}

	// == Write the line
	if _, err = Output.Write(_CR); err != nil {
		return outputError(err.Error())
	}
	if _, err = Output.Write(b.toDisplayBytes()); err != nil {
		return outputError(err.Error())
	}
	if _, err = Output.Write(delToRight); err != nil {
		return outputError(err.Error())
	}

	// == Move cursor to original position.
	for ln := lastLine; ln > posLine; ln-- {
		if _, err = Output.Write(toPreviousLine); err != nil {
			return outputError(err.Error())
		}
	}
	if _, err = fmt.Fprintf(Output, "\r\033[%dC", posColumn); err != nil {
		return outputError(err.Error())
	}

	return nil
}

// == Movement

// start moves the cursor at the start.
func (b *buffer) start() (err error) {
	if b.pos == b.promptLen {
		return
	}

	for ln, _ := b.pos2xy(b.pos); ln > 0; ln-- {
		if _, err = Output.Write(CursorUp); err != nil {
			return outputError(err.Error())
		}
	}

	if _, err = fmt.Fprintf(Output, "\r\033[%dC", b.promptLen); err != nil {
		return outputError(err.Error())
	}
	b.pos = b.promptLen
	return
}

// end moves the cursor at the end.
// Returns the number of lines that fill in the data.
func (b *buffer) end() (lines int, err error) {
	if b.pos == b.size {
		return
	}

	lastLine, lastColumn := b.pos2xy(b.size)

	for ln, _ := b.pos2xy(b.pos); ln < lastLine; ln++ {
		if _, err = Output.Write(cursorDown); err != nil {
			return 0, outputError(err.Error())
		}
	}

	if _, err = fmt.Fprintf(Output, "\r\033[%dC", lastColumn); err != nil {
		return 0, outputError(err.Error())
	}
	b.pos = b.size
	return lastLine, nil
}

// backward moves the cursor one character backward.
// Returns a boolean to know if the cursor is at the beginning of the line.
func (b *buffer) backward() (start bool, err error) {
	if b.pos == b.promptLen {
		return true, nil
	}
	b.pos--

	// If position is on the same line.
	if _, col := b.pos2xy(b.pos); col != 0 {
		if _, err = Output.Write(cursorBackward); err != nil {
			return false, outputError(err.Error())
		}
	} else {
		if _, err = Output.Write(CursorUp); err != nil {
			return false, outputError(err.Error())
		}
		if _, err = fmt.Fprintf(Output, "\033[%dC", b.columns); err != nil {
			return false, outputError(err.Error())
		}
	}
	return
}

// forward moves the cursor one character forward.
// Returns a boolean to know if the cursor is at the end of the line.
func (b *buffer) forward() (end bool, err error) {
	if b.pos == b.size {
		return true, nil
	}
	b.pos++

	if _, col := b.pos2xy(b.pos); col != 0 {
		if _, err = Output.Write(cursorForward); err != nil {
			return false, outputError(err.Error())
		}
	} else {
		if _, err = Output.Write(toNextLine); err != nil {
			return false, outputError(err.Error())
		}
	}
	return
}

// swap swaps the actual character by the previous one. If it is the end of the
// line then it is swapped the 2nd previous by the previous one.
func (b *buffer) swap() error {
	if b.pos == b.promptLen {
		return nil
	}

	if b.pos < b.size {
		aux := b.data[b.pos-1]
		b.data[b.pos-1] = b.data[b.pos]
		b.data[b.pos] = aux
		b.pos++
		// End of line
	} else {
		aux := b.data[b.pos-2]
		b.data[b.pos-2] = b.data[b.pos-1]
		b.data[b.pos-1] = aux
	}
	return b.refresh()
}

// wordBackward moves the cursor one word backward.
func (b *buffer) wordBackward() (err error) {
	for start := false; ; {
		start, err = b.backward()
		if start == true || err != nil || b.data[b.pos-1] == 32 {
			return
		}
	}
	panic("unreachable")
}

// wordForward moves the cursor one word forward.
func (b *buffer) wordForward() (err error) {
	for end := false; ; {
		end, err = b.forward()
		if end == true || err != nil || b.data[b.pos] == 32 {
			return
		}
	}
	panic("unreachable")
}

// == Delete

// deleteChar deletes the character in cursor.
func (b *buffer) deleteChar() (err error) {
	if b.pos == b.size {
		return
	}

	copy(b.data[b.pos:], b.data[b.pos+1:b.size])
	b.size--

	if lastLine, _ := b.pos2xy(b.size); lastLine == 0 {
		if _, err = Output.Write(delChar); err != nil {
			return outputError(err.Error())
		}
		return nil
	}
	return b.refresh()
}

// deleteCharPrev deletes the previous character from cursor.
func (b *buffer) deleteCharPrev() (err error) {
	if b.pos == b.promptLen {
		return
	}

	copy(b.data[b.pos-1:], b.data[b.pos:b.size])
	b.pos--
	b.size--

	if lastLine, _ := b.pos2xy(b.size); lastLine == 0 {
		if _, err = Output.Write(delBackspace); err != nil {
			return outputError(err.Error())
		}
		return nil
	}
	return b.refresh()
}

// deleteToRight deletes from current position until to end of line.
func (b *buffer) deleteToRight() (err error) {
	if b.pos == b.size {
		return
	}

	lastLine, _ := b.pos2xy(b.size)
	posLine, _ := b.pos2xy(b.pos)

	// To the last line.
	for ln := posLine; ln < lastLine; ln++ {
		if _, err = Output.Write(cursorDown); err != nil {
			return outputError(err.Error())
		}
	}
	// Delete all lines until the cursor position.
	for ln := lastLine; ln > posLine; ln-- {
		if _, err = Output.Write(delLine_cursorUp); err != nil {
			return outputError(err.Error())
		}
	}

	if _, err = Output.Write(delToRight); err != nil {
		return outputError(err.Error())
	}
	b.size = b.pos
	return nil
}

// deleteLine deletes full line.
func (b *buffer) deleteLine() error {
	lines, err := b.end()
	if err != nil {
		return err
	}

	for lines > 0 {
		if _, err = Output.Write(delLine_cursorUp); err != nil {
			return outputError(err.Error())
		}
		lines--
	}
	return nil
}

// == Utility

// grow grows buffer to guarantee space for n more byte.
func (b *buffer) grow(n int) {
	for n > len(b.data) {
		b.data = b.data[:len(b.data)+BufferLen]
	}
}

// pos2xy returns the coordinates of a position for a line of size given in
// columns.
func (b *buffer) pos2xy(pos int) (line, column int) {
	if pos < b.columns {
		return 0, pos
	}

	line = pos / b.columns
	column = pos - (line * b.columns) //- 1
	return
}
