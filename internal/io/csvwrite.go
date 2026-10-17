package io

import (
	"bufio"
	"io"
)

// newCSVWriter returns a writer for an exported CSV file.
//
// It is encoding/csv with one rule removed: a lone "\r" in a field is kept. The
// stock writer drops it (it only emits "\r" when UseCRLF is set, and then it
// emits "\r\n" for every newline, including inside fields), which would lose a
// carriage return from a text column on export. newCSVParser reads files back
// without the matching normalisation, so a value survives the round trip.
func newCSVWriter(w io.Writer) *csvWriter {
	return &csvWriter{w: bufio.NewWriterSize(w, 256*1024)}
}

// csvWriter writes records in RFC 4180 form.
//
// Output is buffered: the writer is called once per field, and a row is eight
// fields, so an unbuffered writer would be several syscalls per row.
type csvWriter struct {
	w   *bufio.Writer
	err error
}

// Write writes one record, quoting the fields that need it.
func (w *csvWriter) Write(record []string) error {
	if w.err != nil {
		return w.err
	}
	for i, field := range record {
		if i > 0 {
			if err := w.writeByte(','); err != nil {
				return err
			}
		}
		if err := w.writeField(field); err != nil {
			return err
		}
	}
	return w.writeString("\n")
}

// writeField writes one field, quoted when it contains a delimiter, a quote or
// a line ending — the same condition encoding/csv uses, so the files are
// interchangeable with it except for the carriage return it drops.
func (w *csvWriter) writeField(field string) error {
	if !fieldNeedsQuotes(field) {
		return w.writeString(field)
	}
	if err := w.writeByte('"'); err != nil {
		return err
	}
	for i := 0; i < len(field); i++ {
		switch field[i] {
		case '"':
			if err := w.writeString(`""`); err != nil {
				return err
			}
		default:
			if err := w.writeByte(field[i]); err != nil {
				return err
			}
		}
	}
	return w.writeByte('"')
}

// fieldNeedsQuotes reports whether a field has to be quoted to be read back.
func fieldNeedsQuotes(field string) bool {
	if field == "" {
		return false
	}
	for i := 0; i < len(field); i++ {
		switch field[i] {
		case ',', '"', '\r', '\n':
			return true
		}
	}
	return false
}

func (w *csvWriter) writeByte(b byte) error {
	if w.err != nil {
		return w.err
	}
	w.err = w.w.WriteByte(b)
	return w.err
}

func (w *csvWriter) writeString(s string) error {
	if w.err != nil {
		return w.err
	}
	_, w.err = w.w.WriteString(s)
	return w.err
}

// Flush writes anything buffered.
func (w *csvWriter) Flush() {
	if w.err != nil {
		return
	}
	w.err = w.w.Flush()
}

// Error reports the first write error.
func (w *csvWriter) Error() error { return w.err }
