package logger

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// MaxBodyLogged limits what we read. 1 << 20 = 1 MiB.
const MaxBodyLogged = 1 << 20

var allowedHeaders = map[string]bool{
	"content-type":   true,
	"user-agent":     true,
	"content-length": true,
	"x-trace-id":     true,
	"traceparent":    true,
	"authorization":  true,
	"set-cookie":     true,
}

// CaptureBody reads r.Body up to MaxBodyLogged bytes, closes it, and returns a new reader.
func CaptureBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil // nothing to capture
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBodyLogged))
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body)) // Reset readable stream
	return body, nil
}

func HeaderAttrs(hdr http.Header) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(hdr))
	for name, values := range hdr {
		lower := strings.ToLower(name)
		if !allowedHeaders[lower] {
			continue
		}
		joined := strings.Join(values, ", ")
		if strings.Contains(lower, "authorization") || lower == "set-cookie" {
			joined = "***"
		}
		attrs = append(attrs, slog.String("http.header."+lower, joined))
	}
	return attrs
}

// DecodeBody inspects r.Body, produces slog.Attrs, and puts a *copy* back.
func DecodeBody(contentType string, body []byte) ([]slog.Attr, error) {
	if len(body) == 0 {
		return nil, nil
	}

	ct, _, _ := mime.ParseMediaType(contentType)
	switch ct {
	case "application/json":
		return jsonAttrs(body)
	case "application/x-www-form-urlencoded":
		return formAttrs(body)
	default:
		return binaryAttrs(body), nil
	}
}

// QueryAttrs flattens url.Values into slog.Attrs with "http.query." prefix.
func QueryAttrs(q url.Values) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(q))
	for key, values := range q {
		if len(values) == 0 {
			continue
		}
		joined := strings.Join(values, ",")
		attrs = append(attrs, slog.String("http.query."+key, joined))
	}
	return attrs
}

// BodyAttrs inspects r.Body, produces slog.Attrs, and puts a *copy* back.
func BodyAttrs(r *http.Request) ([]slog.Attr, error) {
	// ---- 1. Read with size guard -----------------------------------------
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBodyLogged))
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body)) // hand it downstream intact

	// 👉 Skip logging if body is truly empty
	if len(body) == 0 {
		return nil, nil
	}

	// ---- 2. Decide by Content‑Type ---------------------------------------
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch ct {
	case "application/json":
		return jsonAttrs(body)
	case "application/x-www-form-urlencoded":
		return formAttrs(body)
	default:
		return binaryAttrs(body), nil
	}
}

// ---------- Helpers ------------------------------------------------------

// For application/json
func jsonAttrs(b []byte) ([]slog.Attr, error) {
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		// Not valid JSON – log string.
		return []slog.Attr{slog.String("http.body", string(b))}, nil
	}
	attrs := make([]slog.Attr, 0, 8)
	flattenJSON("http.body", data, &attrs)
	return attrs, nil
}

func flattenJSON(prefix string, v any, dst *[]slog.Attr) {
	switch t := v.(type) {
	case map[string]any:
		for k, v2 := range t {
			flattenJSON(prefix+"."+k, v2, dst)
		}
	case []any:
		n := len(t)
		switch {
		case n == 1:
			flattenJSON(prefix+".0", t[0], dst)
		case n > 1:
			flattenJSON(prefix+".0", t[0], dst)
			flattenJSON(prefix+"."+strconv.Itoa(n-1), t[n-1], dst)
		default:
			// empty array – skip
		}
	case string:
		*dst = append(*dst, slog.String(prefix, redactIfNeeded(t)))
	case float64:
		*dst = append(*dst, slog.Float64(prefix, t))
	case bool:
		*dst = append(*dst, slog.Bool(prefix, t))
	case nil:
		// skip nulls to cut noise
	default:
		*dst = append(*dst, slog.String(prefix, fmt.Sprintf("%v", t)))
	}
}

// For application/x-www-form-urlencoded
func formAttrs(b []byte) ([]slog.Attr, error) {
	vals, err := url.ParseQuery(string(b))
	if err != nil {
		return nil, err
	}
	attrs := make([]slog.Attr, 0, len(vals))
	for k, v := range vals {
		attrs = append(attrs, slog.String("http.body."+k, redactIfNeeded(strings.Join(v, ", "))))
	}
	return attrs, nil
}

// For everything else (binary, multipart, XML…)
func binaryAttrs(b []byte) []slog.Attr {
	if len(b) == 0 {
		return nil
	}
	const max = 256
	if len(b) <= max {
		return []slog.Attr{slog.String("http.body.base64", base64.StdEncoding.EncodeToString(b))}
	}
	return []slog.Attr{
		slog.Int("http.body.size_bytes", len(b)),
		slog.String("http.body.sample_base64", base64.StdEncoding.EncodeToString(b[:max])),
	}
}

// Very naive example redactor; plug real regexps/HashiCorp Vault here.
func redactIfNeeded(s string) string {
	if strings.Contains(strings.ToLower(s), "password") {
		return "***"
	}
	return s
}

// LogHTTPRequest logs the HTTP request metadata, headers, query, and optionally body.
func LogHTTPRequest(ctx context.Context, r *http.Request, direction string) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("http.direction", direction),
		slog.String("http.remote_addr", r.RemoteAddr),
		slog.String("http.method", r.Method),
		slog.String("http.path", r.URL.Path),
	}

	attrs = append(attrs, HeaderAttrs(r.Header)...)
	attrs = append(attrs, QueryAttrs(r.URL.Query())...)

	if body, err := CaptureBody(r); err == nil && len(body) > 0 {
		if bodyAttrs, err := DecodeBody(r.Header.Get("Content-Type"), body); err == nil {
			attrs = append(attrs, bodyAttrs...)
		} else {
			attrs = append(attrs, slog.String("http.body.error", err.Error()))
		}
	}

	return attrs
}

// LogHTTPResponse logs HTTP response metadata including headers and optional body content.
// `rw` must be a custom ResponseWriter that buffers body (e.g., *BufferedResponseWriter).
func LogHTTPResponse(ctx context.Context, req *http.Request, response_header http.Header, response_status int, response_body io.Reader, duration int64, direction string) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("http.direction", direction),
		slog.String("http.remote_addr", req.RemoteAddr),
		slog.String("http.method", req.Method),
		slog.String("http.path", req.URL.Path),
		slog.Int("http.status", response_status),
		slog.Int64("duration_ms", duration),
	}

	attrs = append(attrs, HeaderAttrs(response_header)...)
	attrs = append(attrs, QueryAttrs(req.URL.Query())...)

	if response_body != nil {
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, response_body); err == nil && buf.Len() > 0 {
			bAttrs, err := DecodeBody(response_header.Get("Content-Type"), buf.Bytes())
			if err == nil {
				attrs = append(attrs, bAttrs...)
			} else {
				attrs = append(attrs, slog.String("http.body.error", err.Error()))
			}
		}
	}
	return attrs
}
