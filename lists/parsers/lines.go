package parsers

import (
	"bufio"
	"context"
	"encoding"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Lines splits `r` into a series of lines.
//
// Empty lines are skipped, and comments are stripped.
func Lines(r io.Reader) SeriesParser[string] {
	return newLines(r)
}

// LinesAs returns a parser that parses each line of `r` as a `T`.
func LinesAs[TPtr TextUnmarshaler[T], T any](r io.Reader) SeriesParser[*T] {
	return UnmarshalEach[TPtr](Lines(r))
}

// UnmarshalEach returns a parser that unmarshals each string of `inner` as a `T`.
func UnmarshalEach[TPtr TextUnmarshaler[T], T any](inner SeriesParser[string]) SeriesParser[*T] {
	stringToBytes := func(s string) []byte {
		return []byte(s)
	}

	return TryAdaptMethod(Adapt(inner, stringToBytes), TPtr.UnmarshalText)
}

type TextUnmarshaler[T any] interface {
	encoding.TextUnmarshaler
	*T
}

type lines struct {
	scanner *bufio.Scanner
	lineNo  uint
}

const (
	// initialScannerBuffer is the buffer bufio.Scanner starts with.
	initialScannerBuffer = 64 * 1024
	// maxScannerBuffer is the longest single line accepted from a source. bufio.Scanner defaults
	// to 64 KiB, which real lists exceed (minified lists, base64 blobs), and an over-long line
	// is a non-resumable error that discards the rest of the file. Raising the ceiling keeps
	// such a list usable while still bounding memory per line.
	maxScannerBuffer = 4 * 1024 * 1024
)

func newLines(r io.Reader) SeriesParser[string] {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	scanner.Buffer(make([]byte, initialScannerBuffer), maxScannerBuffer)

	return &lines{scanner: scanner}
}

func (l *lines) Position() string {
	return fmt.Sprintf("line %d", l.lineNo)
}

func (l *lines) Next(ctx context.Context) (string, error) {
	for {
		l.lineNo++

		if err := ctx.Err(); err != nil {
			return "", NewNonResumableError(err)
		}

		if !l.scanner.Scan() {
			break
		}

		text := strings.TrimSpace(l.scanner.Text())

		if len(text) == 0 {
			continue // empty line
		}

		if strings.HasPrefix(text, "!") || (l.lineNo == 1 && strings.HasPrefix(text, "[Adblock")) {
			continue // Adblock comment or identifier header
		}

		if idx := strings.IndexRune(text, '#'); idx != -1 {
			if idx == 0 {
				continue // commented line
			}

			// end of line comment
			text = text[:idx]
			text = strings.TrimRightFunc(text, unicode.IsSpace)
		}

		return text, nil
	}

	err := l.scanner.Err()
	if err != nil {
		// bufio.Scanner does not support continuing after an error
		return "", NewNonResumableError(err)
	}

	return "", NewNonResumableError(io.EOF)
}
