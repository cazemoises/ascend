package httplab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// Difference contains JSON literals, preserving exact numeric values. Nil means
// absent, distinct from the literal null. Paths follow JSONPath bracket notation.
type Difference struct {
	Path        string  `json:"path"`
	Kind        string  `json:"kind"`
	Expected    *string `json:"expected"`
	Actual      *string `json:"actual"`
	Explanation string  `json:"explanation"`
}
type Comparison struct {
	Differences []Difference `json:"differences"`
	Truncated   bool         `json:"truncated"`
}

// Compare is deterministic and bounded. Header expectations are a subset; extra
// transport headers do not affect the verdict. JSON objects use exact key sets.
func Compare(actual, expected string) (*Comparison, error) {
	a, err := ParseResponse(actual)
	if err != nil {
		return nil, err
	}
	e, err := ParseResponse(expected)
	if err != nil {
		return nil, err
	}
	c := &Comparison{Differences: make([]Difference, 0)}
	add := func(path, kind, message string, e, a any, hasE, hasA bool) {
		if len(c.Differences) >= 100 {
			c.Truncated = true
			return
		}
		literal := func(v any) *string { raw, _ := json.Marshal(v); s := string(raw); return &s }
		d := Difference{Path: path, Kind: kind, Explanation: message}
		if hasE {
			d.Expected = literal(e)
		}
		if hasA {
			d.Actual = literal(a)
		}
		c.Differences = append(c.Differences, d)
	}
	if a.Status != e.Status {
		add("$.status", "status", "O código HTTP recebido difere do esperado.", e.Status, a.Status, true, true)
	}
	keys := make([]string, 0, len(e.Headers))
	for k := range e.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, ok := a.Headers[k]
		if !ok || v != e.Headers[k] {
			add("$.headers["+strconv.Quote(k)+"]", "header", "Compare o header declarado pelo mentor; nomes ignoram maiúsculas, valores são exatos.", e.Headers[k], v, true, ok)
		}
	}
	if a.RawBody != nil {
		add("$.json", "invalid_json", "O corpo recebido não é JSON válido.", json.RawMessage(e.JSON), *a.RawBody, true, true)
		return c, nil
	}
	decode := func(raw []byte) any {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		_ = d.Decode(&v)
		return v
	}
	var walk func(string, any, any, int)
	walk = func(path string, e, a any, depth int) {
		if len(c.Differences) >= 100 || depth > 32 {
			c.Truncated = true
			return
		}
		if equalJSON(e, a) {
			return
		}
		if jsonType(e) != jsonType(a) {
			add(path, "type", fmt.Sprintf("O contrato exige %s, mas a resposta contém %s. Texto e número não são intercambiáveis.", jsonType(e), jsonType(a)), e, a, true, true)
			return
		}
		switch ev := e.(type) {
		case map[string]any:
			av := a.(map[string]any)
			keys := make([]string, 0, len(ev)+len(av))
			for k := range ev {
				keys = append(keys, k)
			}
			for k := range av {
				if _, ok := ev[k]; !ok {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			for _, k := range keys {
				ee, hasE := ev[k]
				aa, hasA := av[k]
				p := path + "[" + strconv.Quote(k) + "]"
				if !hasA {
					add(p, "missing", "Este campo está ausente na resposta recebida.", ee, nil, true, false)
				} else if !hasE {
					add(p, "extra", "Este campo extra não faz parte do JSON esperado.", nil, aa, false, true)
				} else {
					walk(p, ee, aa, depth+1)
				}
			}
		case []any:
			av := a.([]any)
			n := len(ev)
			if len(av) > n {
				n = len(av)
			}
			for i := 0; i < n; i++ {
				p := path + "[" + strconv.Itoa(i) + "]"
				if i >= len(ev) {
					add(p, "extra", "Este elemento extra não faz parte do array esperado.", nil, av[i], false, true)
				} else if i >= len(av) {
					add(p, "missing", "Este elemento está ausente; a ordem dos arrays importa.", ev[i], nil, true, false)
				} else {
					walk(p, ev[i], av[i], depth+1)
				}
			}
		default:
			add(path, "value", "O valor recebido difere do esperado. Arrays mantêm a ordem; objetos ignoram a ordem das chaves.", e, a, true, true)
		}
	}
	walk("$.json", decode(e.JSON), decode(a.JSON), 0)
	return c, nil
}
func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "booleano"
	case string:
		return "texto"
	case json.Number:
		return "número"
	case []any:
		return "array"
	default:
		return "objeto"
	}
}
