// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package telemetry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const otlpEndpoint = "http://localhost:4318/v1/traces"

// Attribute is a key-value pair attached to an OTel span.
type Attribute struct {
	Key   string
	Value interface{}
}

// Attr is a convenience constructor for Attribute.
func Attr(key string, value interface{}) Attribute {
	return Attribute{Key: key, Value: value}
}

// ExportSpan posts a single span to the local OTLP/HTTP endpoint.
// Fire-and-forget: silently no-ops if the backend is unreachable.
func ExportSpan(name string, duration time.Duration, attrs ...Attribute) {
	endTime := time.Now().UTC()
	startTime := endTime.Add(-duration)

	traceID := randomHex(16)
	spanID := randomHex(8)

	// Build attributes array per OTLP JSON spec
	otelAttrs := make([]map[string]interface{}, 0, len(attrs)+1)
	otelAttrs = append(otelAttrs, otelStringAttr("devx.event", name))
	for _, a := range attrs {
		otelAttrs = append(otelAttrs, marshalAttr(a))
	}

	payload := map[string]interface{}{
		"resourceSpans": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						otelStringAttr("service.name", "devx"),
					},
				},
				"scopeSpans": []map[string]interface{}{
					{
						"scope": map[string]interface{}{
							"name":    "devx/telemetry",
							"version": "0.1.0",
						},
						"spans": []map[string]interface{}{
							{
								"traceId":            traceID,
								"spanId":             spanID,
								"name":               name,
								"kind":               1, // SPAN_KIND_INTERNAL
								"startTimeUnixNano":  fmt.Sprintf("%d", startTime.UnixNano()),
								"endTimeUnixNano":    fmt.Sprintf("%d", endTime.UnixNano()),
								"attributes":         otelAttrs,
								"status":             map[string]interface{}{},
								"droppedLinksCount":  0,
								"droppedEventsCount": 0,
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(otlpEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return // Backend not running — silent no-op
	}
	defer func() { _ = resp.Body.Close() }() 
}

// randomHex generates n random bytes and returns as hex string.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use zero bytes (extremely unlikely)
		return hex.EncodeToString(make([]byte, n))
	}
	return hex.EncodeToString(b)
}

// marshalAttr converts an Attribute to the OTLP JSON attribute format.
func marshalAttr(a Attribute) map[string]interface{} {
	switch v := a.Value.(type) {
	case string:
		return otelStringAttr(a.Key, v)
	case bool:
		return map[string]interface{}{
			"key":   a.Key,
			"value": map[string]interface{}{"boolValue": v},
		}
	case int:
		return map[string]interface{}{
			"key":   a.Key,
			"value": map[string]interface{}{"intValue": fmt.Sprintf("%d", v)},
		}
	case int64:
		return map[string]interface{}{
			"key":   a.Key,
			"value": map[string]interface{}{"intValue": fmt.Sprintf("%d", v)},
		}
	case float64:
		return map[string]interface{}{
			"key":   a.Key,
			"value": map[string]interface{}{"doubleValue": v},
		}
	default:
		return otelStringAttr(a.Key, fmt.Sprintf("%v", a.Value))
	}
}

// otelStringAttr creates an OTLP string attribute.
func otelStringAttr(key, value string) map[string]interface{} {
	return map[string]interface{}{
		"key":   key,
		"value": map[string]interface{}{"stringValue": value},
	}
}

// BuildOTLPPayload is exported for testing — constructs the OTLP JSON payload
// without actually sending it.
func BuildOTLPPayload(name string, duration time.Duration, attrs ...Attribute) ([]byte, error) {
	endTime := time.Now().UTC()
	startTime := endTime.Add(-duration)

	traceID := randomHex(16)
	spanID := randomHex(8)

	otelAttrs := make([]map[string]interface{}, 0, len(attrs)+1)
	otelAttrs = append(otelAttrs, otelStringAttr("devx.event", name))
	for _, a := range attrs {
		otelAttrs = append(otelAttrs, marshalAttr(a))
	}

	payload := map[string]interface{}{
		"resourceSpans": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						otelStringAttr("service.name", "devx"),
					},
				},
				"scopeSpans": []map[string]interface{}{
					{
						"scope": map[string]interface{}{
							"name":    "devx/telemetry",
							"version": "0.1.0",
						},
						"spans": []map[string]interface{}{
							{
								"traceId":            traceID,
								"spanId":             spanID,
								"name":               name,
								"kind":               1,
								"startTimeUnixNano":  fmt.Sprintf("%d", startTime.UnixNano()),
								"endTimeUnixNano":    fmt.Sprintf("%d", endTime.UnixNano()),
								"attributes":         otelAttrs,
								"status":             map[string]interface{}{},
								"droppedLinksCount":  0,
								"droppedEventsCount": 0,
							},
						},
					},
				},
			},
		},
	}

	return json.Marshal(payload)
}
