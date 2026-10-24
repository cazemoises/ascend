package httplab

import "testing"

func TestRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"method":"POST","path":"/items/2?q=abc","json":{"n":1}}`, true},
		{`{"method":"GET","path":"http://example.com"}`, false},
		{`{"method":"GET","path":"//example.com"}`, false},
		{`{"method":"GET","path":"/a\r\nHost: evil"}`, false},
		{`{"method":"CONNECT","path":"/"}`, false},
		{`{"method":"GET","path":"/"}`, true},
	} {
		_, err := ParseRequest(tc.raw)
		if (err == nil) != tc.valid {
			t.Errorf("%s: %v", tc.raw, err)
		}
	}
}
func TestResponseComparison(t *testing.T) {
	for _, tc := range []struct {
		actual, expected string
		want             bool
	}{
		{`{"status":201,"json":{"b":2,"a":[1,true,null]}}`, `{"status":201,"json":{"a":[1,true,null],"b":2}}`, true},
		{`{"status":200,"json":null}`, `{"status":204,"json":null}`, false},
		{`{"status":200,"json":[2,1]}`, `{"status":200,"json":[1,2]}`, false},
		{`{"status":200,"json":{"a":1,"b":2}}`, `{"status":200,"json":{"a":1}}`, false},
		{`{"status":200,"json":"1"}`, `{"status":200,"json":1}`, false},
	} {
		if got := Match(tc.actual, tc.expected); got != tc.want {
			t.Errorf("Match(%s,%s)=%v", tc.actual, tc.expected, got)
		}
	}
}

func TestJSONNumbersAndRawBody(t *testing.T) {
	if Match(`{"status":200,"json":1e999999999}`, `{"status":200,"json":1e999999999}`) {
		t.Fatal("unbounded numeric exponent accepted")
	}
	if Match(`{"status":200,"json":9007199254740993}`, `{"status":200,"json":9007199254740992}`) {
		t.Fatal("rounded distinct large integers")
	}
	if !Match(`{"status":200,"json":1.0}`, `{"status":200,"json":1}`) {
		t.Fatal("equivalent JSON numbers differ")
	}
	if Match(`{"status":200,"json":null,"raw_body":"not JSON"}`, `{"status":200,"json":null}`) {
		t.Fatal("non-JSON response accepted as null")
	}
	if Match(`{"status":200,"json":null}`, `{"status":200,"json":{}}`) {
		t.Fatal("null matched object")
	}
}
