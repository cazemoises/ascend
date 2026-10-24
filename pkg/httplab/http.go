// Package httplab defines the bounded HTTP contract shared by authoring and judging.
package httplab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
)

type Request struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	JSON   json.RawMessage `json:"json,omitempty"`
}
type Response struct {
	Headers map[string]string `json:"headers,omitempty"`
	Status  int               `json:"status"`
	JSON    json.RawMessage   `json:"json"`
	RawBody *string           `json:"raw_body,omitempty"`
}

func ParseRequest(raw string) (Request, error) {
	var r Request
	if len(raw) > 16384 {
		return r, fmt.Errorf("HTTP request exceeds 16 KiB")
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, fmt.Errorf("decode HTTP request: %w", err)
	}
	switch r.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		return r, fmt.Errorf("unsupported HTTP method")
	}
	u, err := url.ParseRequestURI(r.Path)
	if err != nil || !strings.HasPrefix(r.Path, "/") || strings.HasPrefix(r.Path, "//") || u.IsAbs() || u.Host != "" || strings.ContainsAny(r.Path, "\r\n\\") {
		return r, fmt.Errorf("path must be a relative HTTP path, with optional query parameters")
	}
	return r, nil
}
func ParseResponse(raw string) (Response, error) {
	var r Response
	if len(raw) > 16384 {
		return r, fmt.Errorf("HTTP response exceeds 16 KiB")
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, fmt.Errorf("decode HTTP response: %w", err)
	}
	if r.Status < 100 || r.Status > 599 || len(r.JSON) == 0 {
		return r, fmt.Errorf("response requires status (100–599) and json (null for empty body)")
	}
	var err error
	r.Headers, err = normalizeHeaders(r.Headers)
	if err != nil {
		return r, err
	}
	return r, nil
}

// Capture only pedagogical headers, never cookies, authentication or tracing data.
func AllowedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "content-type", "content-language", "cache-control", "allow", "content-encoding":
		return true
	default:
		return false
	}
}
func normalizeHeaders(headers map[string]string) (map[string]string, error) {
	normalized := make(map[string]string, len(headers))
	for name, value := range headers {
		key := strings.ToLower(name)
		if !AllowedHeader(key) || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("unsupported or invalid HTTP header")
		}
		if _, exists := normalized[key]; exists {
			return nil, fmt.Errorf("duplicate HTTP header")
		}
		normalized[key] = value
	}
	return normalized, nil
}
func Match(actual, expected string) bool {
	a, err := ParseResponse(actual)
	if err != nil {
		return false
	}
	e, err := ParseResponse(expected)
	if err != nil || a.Status != e.Status || a.RawBody != nil {
		return false
	}
	for name, value := range e.Headers {
		if actual, ok := a.Headers[name]; !ok || actual != value {
			return false
		}
	}
	var av, ev any
	ad, ed := json.NewDecoder(bytes.NewReader(a.JSON)), json.NewDecoder(bytes.NewReader(e.JSON))
	ad.UseNumber()
	ed.UseNumber()
	if ad.Decode(&av) != nil || ed.Decode(&ev) != nil {
		return false
	}
	return equalJSON(av, ev)
}

func equalJSON(a, b any) bool {
	switch a := a.(type) {
	case json.Number:
		b, ok := b.(json.Number)
		if !ok {
			return false
		}
		if !boundedNumber(a) || !boundedNumber(b) {
			return false
		}
		ar, ok := new(big.Rat).SetString(string(a))
		if !ok {
			return false
		}
		br, ok := new(big.Rat).SetString(string(b))
		return ok && ar.Cmp(br) == 0
	case map[string]any:
		b, ok := b.(map[string]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for k, v := range a {
			bv, ok := b[k]
			if !ok || !equalJSON(v, bv) {
				return false
			}
		}
		return true
	case []any:
		b, ok := b.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i, v := range a {
			if !equalJSON(v, b[i]) {
				return false
			}
		}
		return true
	case nil:
		return b == nil
	case string:
		b, ok := b.(string)
		return ok && a == b
	case bool:
		b, ok := b.(bool)
		return ok && a == b
	default:
		return false
	}
}

// Bound exponent expansion before big.Rat allocation: responses are untrusted.
func boundedNumber(n json.Number) bool {
	raw := string(n)
	if len(raw) > 512 {
		return false
	}
	if i := strings.IndexAny(raw, "eE"); i >= 0 {
		exp, err := strconv.Atoi(raw[i+1:])
		return err == nil && exp >= -1024 && exp <= 1024
	}
	return true
}
