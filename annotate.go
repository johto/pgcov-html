package main

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"os"
	"strings"
	"sort"
)

const style string = `
<style type="text/css">
table.listing td {
	padding: 0px;
	font-size: 12px;
	vertical-align: top;
	padding-left: 10px;
}
table.listing td:first-child {
	text-align: right;
	font-weight: bold;
	vertical-align: center;
}
table.listing tr.miss td {
	background-color: #FFC8C8;
}
table.listing tr.hit td {
	background-color: #E8FFE8;
}
</style>
`

// As for the actual functions, we want to sort them roughly by the
// usefulness of the information they contain.  That would mean PL/PgSQL
// functions first, then SQL functions, and the rest at the very end.
// However, we currently don't have that information so just sort by
// whether prosrc is nil or not.
type FunctionSlice []*Function
func (fs FunctionSlice) Len() int {
	return len(fs)
}
func (fs FunctionSlice) Swap(i, j int) {
	fs[i], fs[j] = fs[j], fs[i]
}
func (fs FunctionSlice) Less(i, j int) bool {
	if (fs[i].prosrc == nil) != (fs[j].prosrc == nil) {
		return fs[i].prosrc != nil
	}
	return fs[i].Signature < fs[j].Signature
}


func Annotate(w io.Writer, functions FunctionSlice, hideSourceListFile string) error {
	var hideSourceList []string
	if hideSourceListFile != "" {
		fh, err := os.Open(hideSourceListFile)
		if err != nil {
			return fmt.Errorf("could not open file %#v for reading: %s", hideSourceListFile, err)
		}
		defer fh.Close()
		r := bufio.NewReader(fh)
		for {
			line, err := r.ReadString('\n')
			if err != nil && err != io.EOF {
				return fmt.Errorf("could not read from file %#v: %s", hideSourceListFile, err)
			}
			line = strings.TrimRight(line, "\r\n")
			if len(line) > 0 {
				hideSourceList = append(hideSourceList, line)
			}
			if err == io.EOF {
				break
			}
		}
		sort.Strings(hideSourceList)
	}

	fmt.Fprintf(w, "<html>%s<head><title>Coverage Report</title></head><body>\n", style)

	// see above
	sort.Sort(functions)

	for _, fn := range(functions) {
		fmt.Fprintf(w, "function %s:\n<br /><br />\n", html.EscapeString(fn.Signature))
		err := printSource(w, fn, hideSourceList)
		if err != nil {
			return err
		}
	}
	fmt.Fprint(w, "</body></html>")
	return nil
}

/* try and strip single-line comments */
func stripComments(lines []string) []string {
	stripped := make([]string, len(lines))
	for i, line := range(lines) {
		idx := strings.Index(line, "--")
		if idx > -1 {
			stripped[i] = line[:idx]
		} else {
			stripped[i] = line
		}
	}
	return stripped
}

func findNextLineno(lineno int32, lines []sourceLine) int32 {
	for _, line := range(lines) {
		if line.lineno > lineno {
			return line.lineno
		}
	}
	return 0
}

type sourceLineInfo struct {
	firstLineno int32
	lastLineno int32
	ncalls int32
}

func stripSomeStuff(lines []string) []string {
	for i, l := range(lines) {
		// PL/PgSQL doesn't tell us where these statements are
		if strings.ToLower(strings.TrimSpace(l)) == "end if;" ||
		   strings.ToLower(strings.TrimSpace(l)) == "end loop;" {
			lines[i] = ""
		}
	}
	return lines
}

func getLineInfo(split []string, lines []sourceLine) (info []sourceLineInfo, err error) {
	stripped := stripComments(split)
	stripped = stripSomeStuff(stripped)
	info = make([]sourceLineInfo, 0, len(lines))
	for idx, line := range(lines) {
		li := sourceLineInfo{
			firstLineno: line.lineno,
			ncalls: line.ncalls,
		}

		nextLineNo := findNextLineno(line.lineno, lines[idx:])
		if nextLineNo == 0 {
			li.lastLineno = int32(len(split))
		} else {
			li.lastLineno = nextLineNo - 1
		}

		for li.lastLineno > li.firstLineno &&
			strings.TrimSpace(stripped[li.lastLineno - 1]) == "" {
			li.lastLineno--
		}

		info = append(info, li)
	}
	return info, nil
}

func printSource(w io.Writer, fn *Function, hideSourceList []string) error {
	// see if we need to hide this source
	idx := sort.SearchStrings(hideSourceList, fn.Signature)
	if idx < len(hideSourceList) && hideSourceList[idx] == fn.Signature {
		fmt.Fprintf(w, "<p>(source code hidden)</p><br />\n")
		return nil
	}

	if fn.prosrc == nil {
		fmt.Fprintf(w, "<p>(no source code information)</p><br />\n")
		return nil
	}

	lines := strings.Split(*fn.prosrc, "\n")
	fmt.Fprintf(w, `<table class="listing">`)
	lineInfo, err := getLineInfo(lines, fn.sourceLines)
	if err != nil {
		return err
	}

	for lineno, line := range(lines) {
		lineno += 1

		class := "whitespace"
		for _, li := range(lineInfo) {
			if int32(lineno) >= li.firstLineno && int32(lineno) <= li.lastLineno {
				if li.ncalls == 0 {
					class = "miss"
				} else {
					class = "hit"
				}
				break
			}
		}

		fmt.Fprintf(w, `<tr class="%s"><td>%d<td><code><pre>%s</pre></code></td></tr>`,
			class, lineno,
			html.EscapeString(strings.Replace(line, "\t", "    ", -1)))
		fmt.Fprintln(w)
		_ = lineno
	}
	fmt.Fprintf(w, "</table><br />\n")
	return nil
}
