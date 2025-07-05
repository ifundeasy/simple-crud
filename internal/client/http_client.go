package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"simple-crud/internal/logger"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

var HttpClientTracer = otel.Tracer("HttpClient")

// HTTPClient wrapper with generic support
type HTTPClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
}

// RequestOptions for request configuration
type RequestOptions struct {
	Method      string
	URL         string
	Headers     map[string]string
	QueryParams map[string]string
	Body        interface{}
	Timeout     time.Duration
	Context     context.Context
}

// Response wrapper with generic type
type Response[T any] struct {
	Data       T
	StatusCode int
	Headers    http.Header
	RawBody    []byte
}

// NewHTTPClient creates a new HTTP client instance
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL: strings.TrimRight(baseURL, "/"),
		headers: make(map[string]string),
	}
}

// SetDefaultHeader adds a default header
func (c *HTTPClient) SetDefaultHeader(key, value string) {
	c.headers[key] = value
}

// SetDefaultHeaders adds multiple default headers
func (c *HTTPClient) SetDefaultHeaders(headers map[string]string) {
	for k, v := range headers {
		c.headers[k] = v
	}
}

// Do performs HTTP request with generic response type
func (c *HTTPClient) Do(opts RequestOptions, result interface{}) error {
	// Build URL with query parameters
	fullURL, err := c.buildURL(opts.URL, opts.QueryParams)
	if err != nil {
		logger.Error(opts.Context, "Failed to build URL", slog.Any("error", err))
		return fmt.Errorf("Failed to build URL: %w", err)
	}

	// Prepare request body
	var bodyReader io.Reader
	if opts.Body != nil {
		bodyBytes, err := c.encodeBody(opts.Body)
		if err != nil {
			logger.Error(opts.Context, "Failed to encode body", slog.Any("error", err))
			return fmt.Errorf("Failed to encode body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, span := HttpClientTracer.Start(ctx, "backend-http-request")

	req, err := http.NewRequestWithContext(ctx, opts.Method, fullURL, bodyReader)
	if err != nil {
		logger.Error(opts.Context, "Failed to create request", slog.Any("error", err))
		return fmt.Errorf("Failed to create request: %w", err)
	}

	// Inject standard otel headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Optional: Add trace ID as custom header for easier debugging/log correlation
	traceID := span.SpanContext().TraceID().String()
	c.setHeaders(req, map[string]string{
		"X-Trace-ID": traceID,
	})
	spanID := span.SpanContext().SpanID().String()

	logger.Info(ctx, "HttpClient request",
		slog.Any("headers", c.headers),
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("trace_id", traceID),
		slog.String("span_id", spanID),
	)

	// Set headers
	c.setHeaders(req, opts.Headers)

	// Set timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		logger.Error(opts.Context, "Failed to execute request", slog.String("error", err.Error()))
		return fmt.Errorf("Failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error(opts.Context, "Failed to read response body", slog.String("error", err.Error()))
		return fmt.Errorf("Failed to read response body: %w", err)
	}

	// Parse response based on result type
	if result != nil {
		// Check if result is a Response type
		if respPtr, ok := result.(*Response[interface{}]); ok {
			// Handle Response type
			var data interface{}
			if len(rawBody) > 0 {
				if err := json.Unmarshal(rawBody, &data); err != nil {
					// If JSON parsing fails, try to assign raw body
					if err := c.assignRawBody(opts, &data, rawBody); err != nil {
						logger.Error(opts.Context, "Failed to parse response", slog.Any("error", err))
						return fmt.Errorf("failed to parse response: %w", err)
					}
				}
			}
			respPtr.Data = data
			respPtr.StatusCode = resp.StatusCode
			respPtr.Headers = resp.Header
			respPtr.RawBody = rawBody

			// logger.Info(ctx, "HttpClient request", slog.Any("data", data))
		} else {
			// Handle direct type
			if len(rawBody) > 0 {
				if err := json.Unmarshal(rawBody, result); err != nil {
					// If JSON parsing fails, try to assign raw body
					if err := c.assignRawBody(opts, result, rawBody); err != nil {
						logger.Error(opts.Context, "Failed to parse response", slog.Any("error", err))
						return fmt.Errorf("failed to parse response: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// DoWithResponse performs HTTP request and returns Response struct
func (c *HTTPClient) DoWithResponse(opts RequestOptions) (*Response[interface{}], error) {
	result := &Response[interface{}]{}
	err := c.Do(opts, result)
	return result, err
}

// GET request
func (c *HTTPClient) Get(url string, result interface{}, opts ...RequestOptions) error {
	reqOpts := RequestOptions{
		Method: "GET",
		URL:    url,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.Do(reqOpts, result)
}

// GetWithResponse performs GET request and returns Response struct
func (c *HTTPClient) GetWithResponse(url string, opts ...RequestOptions) (*Response[interface{}], error) {
	reqOpts := RequestOptions{
		Method: "GET",
		URL:    url,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.DoWithResponse(reqOpts)
}

// POST request
func (c *HTTPClient) Post(url string, body interface{}, result interface{}, opts ...RequestOptions) error {
	reqOpts := RequestOptions{
		Method: "POST",
		URL:    url,
		Body:   body,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.Do(reqOpts, result)
}

// PostWithResponse performs POST request and returns Response struct
func (c *HTTPClient) PostWithResponse(url string, body interface{}, opts ...RequestOptions) (*Response[interface{}], error) {
	reqOpts := RequestOptions{
		Method: "POST",
		URL:    url,
		Body:   body,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.DoWithResponse(reqOpts)
}

// PUT request
func (c *HTTPClient) Put(url string, body interface{}, result interface{}, opts ...RequestOptions) error {
	reqOpts := RequestOptions{
		Method: "PUT",
		URL:    url,
		Body:   body,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.Do(reqOpts, result)
}

// PutWithResponse performs PUT request and returns Response struct
func (c *HTTPClient) PutWithResponse(url string, body interface{}, opts ...RequestOptions) (*Response[interface{}], error) {
	reqOpts := RequestOptions{
		Method: "PUT",
		URL:    url,
		Body:   body,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.DoWithResponse(reqOpts)
}

// PATCH request
func (c *HTTPClient) Patch(url string, body interface{}, result interface{}, opts ...RequestOptions) error {
	reqOpts := RequestOptions{
		Method: "PATCH",
		URL:    url,
		Body:   body,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.Do(reqOpts, result)
}

// PatchWithResponse performs PATCH request and returns Response struct
func (c *HTTPClient) PatchWithResponse(url string, body interface{}, opts ...RequestOptions) (*Response[interface{}], error) {
	reqOpts := RequestOptions{
		Method: "PATCH",
		URL:    url,
		Body:   body,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.DoWithResponse(reqOpts)
}

// DELETE request
func (c *HTTPClient) Delete(url string, result interface{}, opts ...RequestOptions) error {
	reqOpts := RequestOptions{
		Method: "DELETE",
		URL:    url,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.Do(reqOpts, result)
}

// DeleteWithResponse performs DELETE request and returns Response struct
func (c *HTTPClient) DeleteWithResponse(url string, opts ...RequestOptions) (*Response[interface{}], error) {
	reqOpts := RequestOptions{
		Method: "DELETE",
		URL:    url,
	}
	if len(opts) > 0 {
		reqOpts = c.mergeOptions(reqOpts, opts[0])
	}
	return c.DoWithResponse(reqOpts)
}

// buildURL builds complete URL with query parameters
func (c *HTTPClient) buildURL(endpoint string, queryParams map[string]string) (string, error) {
	var fullURL string

	// If endpoint is already a complete URL, use it directly
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		fullURL = endpoint
	} else {
		// Combine with base URL
		endpoint = strings.TrimLeft(endpoint, "/")
		fullURL = fmt.Sprintf("%s/%s", c.baseURL, endpoint)
	}

	// Add query parameters
	if len(queryParams) > 0 {
		u, err := url.Parse(fullURL)
		if err != nil {
			return "", err
		}

		q := u.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	return fullURL, nil
}

// encodeBody encodes request body
func (c *HTTPClient) encodeBody(body interface{}) ([]byte, error) {
	switch v := body.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case io.Reader:
		return io.ReadAll(v)
	default:
		return json.Marshal(body)
	}
}

// setHeaders sets request headers
func (c *HTTPClient) setHeaders(req *http.Request, headers map[string]string) {
	// Set default headers
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	// Set default Content-Type if body exists and not already set
	if req.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set headers from options (override defaults)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

// assignRawBody tries to assign raw body for certain data types
func (c *HTTPClient) assignRawBody(opts RequestOptions, data interface{}, rawBody []byte) error {
	switch v := data.(type) {
	case *string:
		*v = string(rawBody)
		return nil
	case *[]byte:
		*v = rawBody
		return nil
	default:
		logger.Error(opts.Context, "Cannot assign raw body to type", slog.Any("data", data))
		return fmt.Errorf("Cannot assign raw body to type %T", data)
	}
}

// mergeOptions merges request options
func (c *HTTPClient) mergeOptions(base, override RequestOptions) RequestOptions {
	if override.Method != "" {
		base.Method = override.Method
	}
	if override.URL != "" {
		base.URL = override.URL
	}
	if override.Body != nil {
		base.Body = override.Body
	}
	if override.Context != nil {
		base.Context = override.Context
	}
	if override.Timeout > 0 {
		base.Timeout = override.Timeout
	}

	// Merge headers
	if base.Headers == nil {
		base.Headers = make(map[string]string)
	}
	for k, v := range override.Headers {
		base.Headers[k] = v
	}

	// Merge query params
	if base.QueryParams == nil {
		base.QueryParams = make(map[string]string)
	}
	for k, v := range override.QueryParams {
		base.QueryParams[k] = v
	}

	return base
}

// Utility methods for checking response status
func (r *Response[T]) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

func (r *Response[T]) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

func (r *Response[T]) IsServerError() bool {
	return r.StatusCode >= 500
}

func (r *Response[T]) GetHeader(key string) string {
	return r.Headers.Get(key)
}
