package httplab

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestComparison(t *testing.T) {
	for _, tc := range []struct{ name, expected, actual, kind, path string }{
		{"status", `{"status":201,"json":null}`, `{"status":200,"json":null}`, "status", "$.status"},
		{"header missing", `{"status":200,"headers":{"Content-Language":"pt"},"json":null}`, `{"status":200,"json":null}`, "header", `$.headers["content-language"]`},
		{"header value", `{"status":200,"headers":{"CONTENT-TYPE":"application/json"},"json":null}`, `{"status":200,"headers":{"content-type":"text/plain"},"json":null}`, "header", `$.headers["content-type"]`},
		{"missing nested", `{"status":200,"json":{"user":{"name":null}}}`, `{"status":200,"json":{"user":{}}}`, "missing", `$.json["user"]["name"]`},
		{"extra", `{"status":200,"json":{}}`, `{"status":200,"json":{"extra":1}}`, "extra", `$.json["extra"]`},
		{"types", `{"status":200,"json":{"value":1}}`, `{"status":200,"json":{"value":"1"}}`, "type", `$.json["value"]`},
		{"array value", `{"status":200,"json":[{"x":true}]}`, `{"status":200,"json":[{"x":false}]}`, "value", `$.json[0]["x"]`},
		{"array missing", `{"status":200,"json":[1,2]}`, `{"status":200,"json":[1]}`, "missing", `$.json[1]`},
		{"array extra", `{"status":200,"json":[1]}`, `{"status":200,"json":[1,2]}`, "extra", `$.json[1]`},
		{"exact large number", `{"status":200,"json":9007199254740993}`, `{"status":200,"json":9007199254740992}`, "value", "$.json"},
		{"quoted path", `{"status":200,"json":{"a.b[0]":1}}`, `{"status":200,"json":{"a.b[0]":2}}`, "value", `$.json["a.b[0]"]`},
		{"invalid JSON body", `{"status":200,"json":null}`, `{"status":200,"json":null,"raw_body":"<html>oops</html>"}`, "invalid_json", "$.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Compare(tc.actual, tc.expected)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Differences) != 1 || c.Differences[0].Kind != tc.kind || c.Differences[0].Path != tc.path {
				t.Fatalf("unexpected comparison: %+v", c)
			}
			if Match(tc.actual, tc.expected) {
				t.Fatal("mismatch accepted")
			}
		})
	}
}
func TestComparisonEqualityAndBounds(t *testing.T) {
	a := `{"status":200,"headers":{"content-type":"application/json","allow":"GET"},"json":{"n":1e0,"a":[]}}`
	e := `{"status":200,"headers":{"Content-Type":"application/json"},"json":{"a":[],"n":1.0}}`
	c, err := Compare(a, e)
	if err != nil || len(c.Differences) != 0 || !Match(a, e) {
		t.Fatalf("equivalent JSON/header subset mismatch: %+v %v", c, err)
	}
	expected := map[string]int{}
	for i := 0; i < 150; i++ {
		expected[fmt.Sprint(i)] = i
	}
	raw, _ := json.Marshal(expected)
	c, err = Compare(`{"status":200,"json":{}}`, `{"status":200,"json":`+string(raw)+`}`)
	if err != nil || len(c.Differences) != 100 || !c.Truncated {
		t.Fatalf("unbounded diff: %+v %v", c, err)
	}
	nested := strings.Repeat("[", 40) + "1" + strings.Repeat("]", 40)
	c, err = Compare(`{"status":200,"json":`+strings.Replace(nested, "1", "2", 1)+`}`, `{"status":200,"json":`+nested+`}`)
	if err != nil || !c.Truncated {
		t.Fatalf("depth not bounded: %+v %v", c, err)
	}
	for _, header := range []string{"Set-Cookie", "Authorization", "X-Api-Key", "Server", "Content-Type"} {
		value := "private"
		if header == "Content-Type" {
			value = "bad\\r\\nvalue"
		}
		if _, err := ParseResponse(`{"status":200,"json":null,"headers":{"` + header + `":"` + value + `"}}`); err == nil {
			t.Errorf("unsafe header accepted: %s", header)
		}
	}
}
