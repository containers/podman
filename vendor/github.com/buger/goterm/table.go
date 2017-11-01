package goterm

import (
	"bytes"
	"text/tabwriter"
)

// Tabwriter with own buffer:
//
//     	totals := tm.NewTable(0, 10, 5, ' ', 0)
// 		fmt.Fprintf(totals, "Time\tStarted\tActive\tFinished\n")
//		fmt.Fprintf(totals, "%s\t%d\t%d\t%d\n", "All", started, started-finished, finished)
//		tm.Println(totals)
//
//  Based on http://golang.org/pkg/text/tabwriter
type Table struct {
	tabwriter.Writer

	Buf *bytes.Buffer
}

// Same as here http://golang.org/pkg/text/tabwriter/#Writer.Init
func NewTable(minwidth, tabwidth, padding int, padchar byte, flags uint) *Table {
	tbl := new(Table)
	tbl.Buf = new(bytes.Buffer)
	tbl.Init(tbl.Buf, minwidth, tabwidth, padding, padchar, flags)

	return tbl
}

func (t *Table) String() string {
	t.Flush()
	return t.Buf.String()
}
