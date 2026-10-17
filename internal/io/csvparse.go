package io

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// CSV parsing
//
// Records are parsed here rather than with encoding/csv because that package is
// lossy for data that has to round-trip. On read it normalises a "\r\n" inside
// a quoted field to "\n"; on write it drops a lone "\r" unless UseCRLF is set,
// which then turns every newline into "\r\n". A text column holding Windows
// line endings is ordinary data, so neither behaviour is acceptable: the bytes
// the export wrote have to be the bytes an import reads back.
//
// The grammar is RFC 4180 as the export writes it, with the tolerance a real
// file needs:
//
//   - a field is quoted when it contains a comma, a quote, a carriage return or
//     a newline;
//   - inside a quoted field, `""` is one quote and every other byte is literal;
//   - records end with "\r\n" or "\n", and a quoted field may span lines;
//   - a line ending carries no field content, so "a,\n" is two fields, the
//     second empty.
//
// Anything else — a quote in the middle of an unquoted field, a `"` where a
// comma or line ending belongs, a file ending inside a quoted field — is a
// parse error naming the line, because guessing at a damaged file is how a
// backup import silently changes data.
//
// The scanner works in runs rather than byte by byte: it finds the next special
// character with bytes.IndexByte over the buffered run and returns an unquoted
// field as a substring of it. On a million-row file that is the difference
// between the parser costing a third of the CPU and costing almost nothing.
//
// The window is kept small on purpose — see csvReadChunk. Every scan runs from
// the parse position to the end of the window, so buffering far ahead of the
// record being parsed makes each field scan bytes that are not its own.

var (
	errBareQuote     = errors.New(`bare " after a quoted field`)
	errQuoteInField  = errors.New(`unescaped " in an unquoted field`)
	errUnclosedQuote = errors.New("unexpected end of file inside a quoted field")

	// errParsedEOF signals the end of the input.
	errParsedEOF = errors.New("end of input")
)

// Special bytes, as byte slices so the lookups can use bytes.IndexByte.
var (
	newlineByte = []byte{'\n'}
	quoteByte   = []byte{'"'}
)

// csvParser reads records from an io.Reader.
//
// win is the window of bytes read so far and pos is how far the parse has got
// into it; whatever is left of the window beyond pos is unconsumed. Fields
// already returned by the current Read point into win, so the window is only
// rewritten at the top of Read, when no such field is live.
type csvParser struct {
	r    io.Reader
	win  []byte // bytes read so far
	pos  int    // parse position within win
	eof  bool   // the underlying reader reported EOF
	line int    // 1-based line the parser is on, for error messages

	field []byte // scratch for a field that has to be assembled

	reuse  bool
	record []string
}

// newCSVParser returns a parser reading from r.
//
// r is wrapped in a buffered reader sized for a bulk transfer; the parser's own
// window then only ever holds a few hundred bytes.
func newCSVParser(r io.Reader) *csvParser {
	if _, ok := r.(*bufio.Reader); !ok {
		r = bufio.NewReaderSize(r, csvBufioSize)
	}
	return &csvParser{r: r, line: 1}
}

// csvReadChunk is how many bytes a single read moves into the window.
//
// Both parsers scan from the parse position to the end of the window to find
// the next delimiter, so whatever is buffered beyond the record being read is
// scanned once per field. A larger window is therefore not free: measured on a
// million-row, 100 MB file, an import took 2.9 s with a 256-byte window, 3.1 s
// at 1 KB, 4.3 s at 4 KB, 7.8 s at 16 KB and 21 s at 64 KB — a straight line in
// window size, because the scan is what costs. 256 bytes is just above one row
// of the data this was measured on, and the read-ahead that keeps the syscall
// count down is done by the buffered source below instead.
const csvReadChunk = 256

// csvBufioSize is the read-ahead buffer under the parser.
//
// The small window above would otherwise cost a syscall per 256 bytes — 400,000
// reads on a 100 MB file. The buffered reader issues one syscall per 64 KB and
// the parser takes 256 bytes out of it per chunk, so the syscall count stays
// low while the window the scans walk stays short.
const csvBufioSize = 64 * 1024

// fill appends one chunk of bytes to the window. It returns errParsedEOF when
// the reader is exhausted.
func (p *csvParser) fill() error {
	for {
		if p.eof {
			if p.pos >= len(p.win) {
				return errParsedEOF
			}
			return nil
		}
		// One chunk is taken per call: letting a buffered source fill the whole
		// free space would put the parse position far behind the end of the
		// window, which is what makes the field scans expensive.
		if br, ok := p.r.(*bufio.Reader); ok {
			// Peek hands back the buffered bytes without copying them into the
			// reader's own buffer first, which Read would do for a request this
			// small.
			chunk, err := br.Peek(csvReadChunk)
			if len(chunk) > 0 {
				p.win = append(p.win, chunk...)
				br.Discard(len(chunk))
				return nil
			}
			if err == io.EOF {
				p.eof = true
				continue
			}
			if err != nil {
				return err
			}
			continue
		}

		start := len(p.win)
		free := p.makeRoom()
		if len(free) > csvReadChunk {
			free = free[:csvReadChunk]
		}
		n, err := p.r.Read(free)
		if n > 0 {
			p.win = p.win[:start+n]
			return nil
		}
		if err == io.EOF {
			p.eof = true
			continue
		}
		if err != nil {
			return err
		}
		// A Read that returns nothing and no error is allowed by io.Reader;
		// it is not the end of the input, so try again.
	}
}

// makeRoom returns free space at the end of the window for the next read,
// extending the window when there is not room for a full chunk.
func (p *csvParser) makeRoom() []byte {
	n := len(p.win)
	if cap(p.win)-n < csvReadChunk {
		// Grow rather than reuse: the window has to hold the unconsumed bytes
		// plus a full chunk, and a reallocation also drops the consumed
		// prefix that a long parse would otherwise pin for its whole life.
		grown := make([]byte, n, n+csvReadChunk)
		copy(grown, p.win)
		p.win = grown
	}
	return p.win[n:cap(p.win)]
}

// compact moves the unconsumed bytes to the front of the window.
//
// Called at the top of Read, when no field of the previous record still points
// into the window, so moving bytes cannot change a value already returned.
func (p *csvParser) compact() {
	if p.pos == 0 {
		return
	}
	rest := copy(p.win, p.win[p.pos:])
	p.win = p.win[:rest]
	p.pos = 0
}

// ensure makes at least one byte available at the parse position.
func (p *csvParser) ensure() error {
	for p.pos >= len(p.win) {
		if err := p.fill(); err != nil {
			return err
		}
	}
	return nil
}

// rest returns the unconsumed part of the window.
func (p *csvParser) rest() []byte { return p.win[p.pos:] }

// advance moves the parse position forward by n.
func (p *csvParser) advance(n int) { p.pos += n }

// Read parses the next record. It returns io.EOF at the end of the input.
func (p *csvParser) Read() ([]string, error) {
	p.compact()

	if p.reuse {
		p.record = p.record[:0]
	} else {
		p.record = nil
	}

	for {
		if err := p.ensure(); err != nil {
			if err == errParsedEOF {
				if len(p.record) > 0 {
					// A final record with no trailing line ending.
					return p.record, nil
				}
				return nil, io.EOF
			}
			return nil, err
		}

		field, err := p.parseField()
		if err != nil {
			return nil, err
		}
		p.record = append(p.record, field)

		if err := p.ensure(); err != nil {
			if err == errParsedEOF {
				return p.record, nil
			}
			return nil, err
		}

		switch p.rest()[0] {
		case ',':
			p.advance(1)
			continue
		case '\r':
			p.advance(1)
			// A stray or paired carriage return both end the record; the
			// newline of a CRLF is consumed when it is there.
			if err := p.ensure(); err == nil && p.rest()[0] == '\n' {
				p.advance(1)
			}
			p.line++
			return p.record, nil
		case '\n':
			p.advance(1)
			p.line++
			return p.record, nil
		default:
			return nil, fmt.Errorf("line %d: %w", p.line, errBareQuote)
		}
	}
}

// parseField reads one field, quoted or unquoted.
func (p *csvParser) parseField() (string, error) {
	if p.rest()[0] != '"' {
		return p.parseUnquoted()
	}

	p.advance(1)
	p.field = p.field[:0]

	for {
		if err := p.ensure(); err != nil {
			if err == errParsedEOF {
				return "", fmt.Errorf("line %d: %w", p.line, errUnclosedQuote)
			}
			return "", err
		}

		// Copy everything up to the next quote or newline in one go.
		buf := p.rest()
		i := indexQuotedSpecial(buf)
		if i < 0 {
			p.field = append(p.field, buf...)
			if n := bytes.Count(buf, newlineByte); n > 0 {
				p.line += n
			}
			p.advance(len(buf))
			continue
		}
		if i > 0 {
			p.field = append(p.field, buf[:i]...)
			if n := bytes.Count(buf[:i], newlineByte); n > 0 {
				p.line += n
			}
			p.advance(i)
		}

		if p.rest()[0] == '\n' {
			// A newline inside a quoted field is data: that is what a
			// multi-line value is.
			p.field = append(p.field, '\n')
			p.advance(1)
			p.line++
			continue
		}

		// A quote: doubled is a literal, otherwise the field ends here.
		p.advance(1)
		if err := p.ensure(); err == nil && p.rest()[0] == '"' {
			p.field = append(p.field, '"')
			p.advance(1)
			continue
		}

		// The field is closed. What follows must be a comma or an ending.
		if err := p.ensure(); err == nil {
			switch p.rest()[0] {
			case ',', '\n', '\r':
			default:
				return "", fmt.Errorf("line %d: %w", p.line, errBareQuote)
			}
		}
		// A quoted field is assembled as it is scanned, so p.field holds it.
		return string(p.field), nil
	}
}

// parseUnquoted reads a field with no quotes.
//
// The common case — the field ends inside the bytes already buffered — is
// returned as a string view over the window, which is one allocation and no
// copy. Only a field that spans reads has to be assembled first.
func (p *csvParser) parseUnquoted() (string, error) {
	assembled := false
	p.field = p.field[:0]

	for {
		if err := p.ensure(); err != nil {
			if err == errParsedEOF {
				return string(p.field), nil
			}
			return "", err
		}

		buf := p.rest()
		i := indexUnquotedSpecial(buf)
		if i < 0 {
			// The field continues past what is buffered.
			p.field = append(p.field, buf...)
			assembled = true
			p.advance(len(buf))
			continue
		}

		switch buf[i] {
		case '"':
			return "", fmt.Errorf("line %d: %w", p.line, errQuoteInField)
		}

		if !assembled {
			field := string(buf[:i])
			p.advance(i)
			return field, nil
		}
		p.field = append(p.field, buf[:i]...)
		p.advance(i)
		return string(p.field), nil
	}
}

// indexQuotedSpecial returns the offset of the next quote or newline, or -1.
func indexQuotedSpecial(buf []byte) int {
	q := bytes.IndexByte(buf, '"')
	n := bytes.IndexByte(buf, '\n')
	switch {
	case q < 0:
		return n
	case n < 0:
		return q
	case n < q:
		return n
	default:
		return q
	}
}

// indexUnquotedSpecial returns the offset of the next delimiter, line ending or
// quote, or -1. The result is the earliest of them, not whichever byte happens
// to be searched first.
//
// The order matters for speed. IndexByte scans until it finds the byte or runs
// out, so a byte that is rare in the data — a carriage return and a quote are
// both absent from an exported file except as quoting — costs a full scan every
// time. The search therefore starts with the rarest and narrows the range it
// has to scan with each hit, which keeps the common case to one scan of the
// remaining field.
func indexUnquotedSpecial(buf []byte) int {
	best := len(buf)
	if i := bytes.IndexByte(buf[:best], '"'); i >= 0 {
		best = i
	}
	if i := bytes.IndexByte(buf[:best], '\r'); i >= 0 {
		best = i
	}
	if i := bytes.IndexByte(buf[:best], '\n'); i >= 0 {
		best = i
	}
	if i := bytes.IndexByte(buf[:best], ','); i >= 0 {
		best = i
	}
	if best == len(buf) {
		return -1
	}
	return best
}
