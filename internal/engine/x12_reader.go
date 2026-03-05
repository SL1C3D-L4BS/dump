// Package engine: X12 stateful reader with hierarchical loops (837 claims, 835 payments).
// Parses segment-terminator (~) and element-separator (*) delimited EDI, maintains a
// context stack from LoopTriggers, builds a nested map, and yields one row per YieldTrigger (e.g. CLM).

package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SL1C3D-L4BS/dump/internal/dialects"
)

// X12Reader reads X12 EDI (e.g. 837, 835), maintains loop context, builds a nested
// map per transaction, and implements io.Reader by streaming one JSONL row per yield trigger.
type X12Reader struct {
	scanner     *bufio.Scanner
	dialect     *dialects.Dialect
	segTerm     string
	elemSep     string
	contextStack []string
	tree        map[string]interface{}
	inTransaction bool
	buf         []byte
	segmentIndex map[string]int
}

// NewX12Reader creates an X12 reader that yields one JSONL row per YieldTrigger segment (e.g. CLM).
func NewX12Reader(r io.Reader, dialect *dialects.Dialect) *X12Reader {
	segTerm := dialect.Delimiters.Segment
	if segTerm == "" {
		segTerm = "~"
	}
	elemSep := dialect.Delimiters.Field
	if elemSep == "" {
		elemSep = "*"
	}
	sc := bufio.NewScanner(r)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxCapacity)
	sc.Split(splitX12Segments(segTerm))
	return &X12Reader{
		scanner:       sc,
		dialect:        dialect,
		segTerm:        segTerm,
		elemSep:        elemSep,
		contextStack:   nil,
		tree:           nil,
		inTransaction:  false,
		segmentIndex:   make(map[string]int),
	}
}

func splitX12Segments(segTerm string) bufio.SplitFunc {
	termBytes := []byte(segTerm)
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if len(data) == 0 {
			if atEOF {
				return 0, nil, nil
			}
			return 0, nil, nil
		}
		idx := bytes.Index(data, termBytes)
		if idx < 0 {
			if atEOF {
				return len(data), bytes.TrimSpace(data), nil
			}
			return 0, nil, nil
		}
		token = bytes.TrimSpace(data[:idx])
		advance = idx + len(termBytes)
		return advance, token, nil
	}
}

// Read implements io.Reader. Streams JSONL (one line per yielded claim/row).
func (x *X12Reader) Read(p []byte) (n int, err error) {
	for len(x.buf) < len(p) {
		row, err := x.nextRow()
		if err == io.EOF {
			n = copy(p, x.buf)
			x.buf = x.buf[n:]
			if len(x.buf) > 0 {
				return n, nil
			}
			return n, io.EOF
		}
		if err != nil {
			return 0, err
		}
		if row == nil {
			continue
		}
		line, err := json.Marshal(row)
		if err != nil {
			return 0, err
		}
		x.buf = append(x.buf, line...)
		x.buf = append(x.buf, '\n')
	}
	n = copy(p, x.buf)
	x.buf = x.buf[n:]
	return n, nil
}

func (x *X12Reader) nextRow() (map[string]interface{}, error) {
	for x.scanner.Scan() {
		segRaw := x.scanner.Text()
		segRaw = strings.TrimSpace(segRaw)
		if segRaw == "" {
			continue
		}
		segID, elements := x.parseSegment(segRaw)
		if segID == "" {
			continue
		}

		stSeg := x.dialect.TransactionBoundary.Start
		seSeg := x.dialect.TransactionBoundary.End
		if stSeg == "" {
			stSeg = "ST"
		}
		if seSeg == "" {
			seSeg = "SE"
		}

		if segID == stSeg {
			x.inTransaction = true
			x.contextStack = nil
			x.tree = make(map[string]interface{})
			x.segmentIndex = make(map[string]int)
		}
		if segID == seSeg {
			x.inTransaction = false
			continue
		}

		if !x.inTransaction {
			continue
		}

		// Evaluate loop triggers
		if rules := x.dialect.LoopTriggers[segID]; len(rules) > 0 {
			for _, r := range rules {
				if r.ElementIndex >= 0 && r.ElementIndex < len(elements) && strings.TrimSpace(elements[r.ElementIndex]) == r.Value {
					x.contextStack = append(x.contextStack, r.EnterLoop)
					break
				}
			}
		}

		// Insert segment at leaf of context stack
		x.insertSegment(segID, elements)

		// Yield trigger
		if x.dialect.YieldTrigger.Segment != "" && segID == x.dialect.YieldTrigger.Segment {
			out := x.copyTree(x.tree)
			return out, nil
		}
	}
	if err := x.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func (x *X12Reader) parseSegment(segRaw string) (segID string, elements []string) {
	parts := strings.Split(segRaw, x.elemSep)
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	if len(parts) < 1 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// insertSegment places the segment into the tree at the path given by contextStack.
func (x *X12Reader) insertSegment(segID string, elements []string) {
	idx := x.segmentIndex[segID]
	x.segmentIndex[segID] = idx + 1
	key := segID
	if idx > 0 {
		key = fmt.Sprintf("%s_%d", segID, idx)
	}
	segMap := make(map[string]interface{})
	names := x.dialect.Segments[segID]
	for i, v := range elements {
		name := x12FieldNameAt(names, i)
		segMap[name] = v
	}

	cur := x.tree
	for i, loopName := range x.contextStack {
		next, _ := cur[loopName].(map[string]interface{})
		if next == nil {
			next = make(map[string]interface{})
			cur[loopName] = next
		}
		cur = next
		_ = i
	}
	cur[key] = segMap
}

func x12FieldNameAt(names []string, i int) string {
	if names != nil && i+1 < len(names) && names[i+1] != "" {
		return names[i+1]
	}
	return "Field" + fmt.Sprint(i+1)
}

func (x *X12Reader) copyTree(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if nm, ok := v.(map[string]interface{}); ok {
			out[k] = x.copyTree(nm)
		} else {
			out[k] = v
		}
	}
	return out
}
