// Package xport reads and writes CSV and minimal XLSX with no dependencies.
package xport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// Cell renders one value spreadsheet-safe: formula-injection neutralised.
func Cell(v any) string {
	var s string
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		s = t
	case *string:
		if t != nil {
			s = *t
		}
	default:
		s = fmt.Sprint(t)
	}
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		s = "'" + s
	}
	return s
}

// CSV writes a UTF-8 (BOM) CSV with CRLF rows.
func CSV(rows [][]any) []byte {
	var b bytes.Buffer
	b.WriteString("\ufeff")
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	for _, r := range rows {
		rec := make([]string, len(r))
		for i, v := range r {
			rec[i] = Cell(v)
		}
		_ = w.Write(rec)
	}
	w.Flush()
	return b.Bytes()
}

// ParseCSV reads a CSV, auto-detecting `,` vs `;` from the header line.
func ParseCSV(data []byte) ([][]string, error) {
	text := strings.TrimPrefix(string(data), "\ufeff")
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = detectDelimiter(text)
	r.FieldsPerRecord = -1
	r.LazyQuotes = false
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, rec)
	}
	for len(rows) > 0 && allEmpty(rows[len(rows)-1]) {
		rows = rows[:len(rows)-1]
	}
	return rows, nil
}

func allEmpty(r []string) bool {
	for _, c := range r {
		if c != "" {
			return false
		}
	}
	return true
}

func detectDelimiter(text string) rune {
	quoted, commas, semis := false, 0, 0
loop:
	for _, c := range text {
		switch {
		case c == '"':
			quoted = !quoted
		case !quoted && (c == '\n' || c == '\r'):
			break loop
		case !quoted && c == ',':
			commas++
		case !quoted && c == ';':
			semis++
		}
	}
	if semis > commas {
		return ';'
	}
	return ','
}
