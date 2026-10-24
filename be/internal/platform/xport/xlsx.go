package xport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// ponytail: single sheet, inline strings, no styles. Add excelize if you need
// formatting, formulas, multiple sheets or streaming of very large exports.
func xmlEsc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)))
	return b.String()
}

func colName(i int) string {
	s := ""
	for n := i + 1; n > 0; n = (n - 1) / 26 {
		s = string(rune('A'+(n-1)%26)) + s
	}
	return s
}

// XLSX builds a one-sheet workbook. Numbers stay numeric, the rest are text.
func XLSX(rows [][]any, sheet string) []byte {
	var sd strings.Builder
	for y, r := range rows {
		fmt.Fprintf(&sd, `<row r="%d">`, y+1)
		for x, v := range r {
			ref := fmt.Sprintf("%s%d", colName(x), y+1)
			switch t := v.(type) {
			case int, int32, int64, float64:
				fmt.Fprintf(&sd, `<c r="%s"><v>%v</v></c>`, ref, t)
			default:
				text := ""
				if v != nil {
					text = Cell(v)
					text = strings.TrimPrefix(text, "'") // xlsx cells are text, not formulas
					if s, ok := v.(string); ok {
						text = s
					}
				}
				fmt.Fprintf(&sd, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, xmlEsc(text))
			}
		}
		sd.WriteString(`</row>`)
	}
	name := xmlEsc(sheet)
	if len(name) > 31 {
		name = name[:31]
	}
	const ns = `xmlns="http://schemas.openxmlformats.org/`
	files := [][2]string{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types ` + ns + `package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships ` + ns + `package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?><workbook ` + ns + `spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="` + name + `" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships ` + ns + `package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet ` + ns + `spreadsheetml/2006/main"><sheetData>` + sd.String() + `</sheetData></worksheet>`},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, _ := zw.Create(f[0])
		_, _ = w.Write([]byte(f[1]))
	}
	_ = zw.Close()
	return buf.Bytes()
}
