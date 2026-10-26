// Package sse provides small helpers for reading and writing Server-Sent Events.
package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Event is one Server-Sent Event.
type Event struct {
	Event string
	Data  []byte
	ID    string
}

// Reader reads SSE events from an upstream response body.
type Reader struct {
	scanner *bufio.Scanner
}

func NewReader(r io.Reader) *Reader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &Reader{scanner: s}
}

func (r *Reader) Next() (Event, error) {
	var ev Event
	var data []string
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			if ev.Event == "" && len(data) == 0 && ev.ID == "" {
				continue
			}
			ev.Data = []byte(strings.Join(data, "\n"))
			return ev, nil
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			field, value = line, ""
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			ev.Event = value
		case "data":
			data = append(data, value)
		case "id":
			ev.ID = value
		}
	}
	if err := r.scanner.Err(); err != nil {
		return ev, err
	}
	if len(data) == 0 && ev.Event == "" {
		return ev, io.EOF
	}
	ev.Data = []byte(strings.Join(data, "\n"))
	return ev, nil
}

// Writer writes SSE events to a downstream client.
type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

func (w *Writer) WriteEvent(event string, data any) error {
	var payload []byte
	switch v := data.(type) {
	case []byte:
		payload = v
	case string:
		payload = []byte(v)
	default:
		var err error
		payload, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}
	if event != "" {
		if _, err := fmt.Fprintf(w.w, "event: %s\n", event); err != nil {
			return err
		}
	}
	for _, line := range strings.Split(string(payload), "\n") {
		if _, err := fmt.Fprintf(w.w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w.w, "\n")
	return err
}

func (w *Writer) WriteDone() error {
	_, err := io.WriteString(w.w, "data: [DONE]\n\n")
	return err
}
