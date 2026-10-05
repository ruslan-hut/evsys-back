package mcpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
)

// roundFloats rewrites a JSON document so that no number carries more than
// two decimals. Averages and integrated energies come out of the reports as
// 29949.1015625 and the like: precision nobody reads, paid for in the model's
// context. The rewrite goes token by token, so field order and strings are
// left exactly as they were.
func roundFloats(data []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	out := bytes.NewBuffer(make([]byte, 0, len(data)))

	// written counts the tokens emitted at each open container level; in an
	// object, even counts mean a key comes next, odd a value
	type level struct {
		object  bool
		written int
	}
	var stack []level

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			out.WriteByte(byte(d))
			continue
		}
		if n := len(stack); n > 0 {
			top := &stack[n-1]
			switch {
			case top.object && top.written%2 == 1:
				out.WriteByte(':')
			case top.written > 0:
				out.WriteByte(',')
			}
			top.written++
		}
		switch v := tok.(type) {
		case json.Delim:
			out.WriteByte(byte(v))
			stack = append(stack, level{object: v == '{'})
		case json.Number:
			out.WriteString(roundNumber(v))
		case string:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			out.Write(b)
		case bool:
			out.WriteString(strconv.FormatBool(v))
		case nil:
			out.WriteString("null")
		}
	}
	return out.Bytes(), nil
}

func roundNumber(n json.Number) string {
	s := n.String()
	if !strings.ContainsAny(s, ".eE") {
		return s
	}
	f, err := n.Float64()
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return s
	}
	// adding zero turns the -0 of a small negative into 0
	return strconv.FormatFloat(math.Round(f*100)/100+0, 'f', -1, 64)
}
