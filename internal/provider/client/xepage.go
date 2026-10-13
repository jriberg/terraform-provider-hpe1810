// Port of ~/code/experiments/hpe/extractors/xe_page.py -- see that file's
// docstring for the two table shapes this parses.
//
// Note on traversal: the switch firmware's HTML puts <TH>/<TR> as direct
// children of <TABLE> with no <TBODY> wrapper. A strict browser-grade HTML5
// parser (which golang.org/x/net/html, underlying goquery, implements) would
// normally insert a synthetic <TBODY> around those rows, which would break a
// "direct children only" traversal the way the Python version does (Python's
// html.parser is lenient and doesn't insert one). We sidestep this by using
// Find() (any descendant) instead of direct-children selectors -- safe here
// because tables in this firmware are never nested inside one another.
package client

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Table struct {
	Caption    string
	Kind       string // "scalar" | "tabular"
	Record     map[string]string
	FieldNames map[string]string
	Rows       []map[string]string
	ID         string
}

func extractCellValue(td *goquery.Selection) string {
	if inp := td.Find("input").First(); inp.Length() > 0 {
		if v, exists := inp.Attr("value"); exists {
			return v
		}
		return ""
	}
	return strings.TrimSpace(td.Text())
}

func parseTable(table *goquery.Selection) Table {
	id, _ := table.Attr("id")
	caption := strings.TrimSpace(table.Find("caption").First().Text())

	var headers []string
	table.Find("th").Each(func(_ int, th *goquery.Selection) {
		headers = append(headers, strings.TrimSpace(th.Text()))
	})

	if len(headers) > 0 {
		var rows []map[string]string
		table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			tds := tr.Find("td")
			if tds.Length() != len(headers) {
				return
			}
			row := make(map[string]string, len(headers))
			tds.Each(func(i int, td *goquery.Selection) {
				// Later (visible) column wins on a duplicate header label --
				// see the same caveat noted in extractors/xe_page.py.
				row[headers[i]] = extractCellValue(td)
			})
			rows = append(rows, row)
		})
		return Table{Caption: caption, Kind: "tabular", Rows: rows, ID: id}
	}

	record := make(map[string]string)
	fieldNames := make(map[string]string)
	table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		defleft := tr.Find("td.defleft").First()
		defright := tr.Find("td.defright").First()
		if defleft.Length() == 0 || defright.Length() == 0 {
			return
		}
		label := strings.TrimSpace(defleft.Text())
		if label == "" {
			return
		}
		record[label] = extractCellValue(defright)
		if inp := defright.Find("input").First(); inp.Length() > 0 {
			if name, exists := inp.Attr("name"); exists && name != "" {
				fieldNames[label] = name
			}
		}
	})

	return Table{
		Caption:    caption,
		Kind:       "scalar",
		Record:     record,
		FieldNames: fieldNames,
		ID:         id,
	}
}

// ParseXETables parses every <TABLE> in html, in document order.
func ParseXETables(html string) ([]Table, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var tables []Table
	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		tables = append(tables, parseTable(table))
	})
	return tables, nil
}

// FetchTables fetches path and returns {caption: table} for every captioned
// table. Tables with no/blank caption are dropped -- use FetchAllTables +
// Table.ID for those (e.g. the MAC Table entries table on FDBSearch.html).
func (c *Client) FetchTables(path string) (map[string]Table, error) {
	html, err := c.Fetch(path)
	if err != nil {
		return nil, err
	}
	tables, err := ParseXETables(html)
	if err != nil {
		return nil, err
	}
	result := make(map[string]Table, len(tables))
	for _, t := range tables {
		if t.Caption != "" {
			result[t.Caption] = t
		}
	}
	return result, nil
}

// FetchAllTables fetches path and returns every table, including
// uncaptioned ones, in document order.
func (c *Client) FetchAllTables(path string) ([]Table, error) {
	html, err := c.Fetch(path)
	if err != nil {
		return nil, err
	}
	return ParseXETables(html)
}
