package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"crypto/tls"
)

//
// Request contains the requested behavior of an HTTP request.
//
// It is an internal object primarly exported for testing
// with NewMockClient.
//
type Request struct {
	Method     string
	URL        string
	Params     url.Values
	Body       interface{}
	JSONOutput interface{}
	Output     io.Writer
	Header     http.Header
}

// RequestOption controls the behavior of the HTTP request.
type RequestOption func(*Request)

// WithJSONResponse will JSON Unmarshal the HTTP response body into this object.
func WithJSONResponse(o interface{}) RequestOption {
	return func(r *Request) {
		r.JSONOutput = o
		r.Header.Add("Accept", "application/json")
	}
}

// WithResponse will write the HTTP response to this writer.
func WithResponse(w io.Writer) RequestOption {
	return func(r *Request) {
		r.Output = w
	}
}

// WithParam will set the query parameter on the HTTP request url.
func WithParam(k, v string) RequestOption {
	return func(r *Request) {
		r.Params.Add(k, v)
	}
}

// WithJSONBody will JSON marshal this object as the HTTP request body.
func WithJSONBody(b interface{}) RequestOption {
	return func(r *Request) {
		if r.Method == "GET" {
			panic("GET requests cannot have a body")
		}
		r.Body = b
		r.Header.Add("Content-Type", "application/json")
	}
}

// WithHeader will set the HTTP Header on the request.
func WithHeader(k, v string) RequestOption {
	return func(r *Request) {
		r.Header.Add(k, v)
	}
}

// Client provides simpler high level interfaces for http querying.
//
// Unfortunately, go does not currently have good interfaces for
// doing http requests, requiring a lot of boilerplate and not
// being very testable (https://github.com/golang/go/issues/23707).
//
// The goal of this client is:
//  - Simple to use (e.g. a semantic api that handles common errors)
//  - Simple to test (e.g. a small interface that provides a semantic api)
//
type Client interface {
	Get(ctx context.Context, url string, options ...RequestOption) error
	Post(ctx context.Context, url string, options ...RequestOption) error
}

type client struct {
	client http.Client
}

// NewTLSClient constructs a Client from the given tls.Config.
func NewTLSClient(config *tls.Config) Client {
	return &client{
		http.Client{
			Transport: &http.Transport{
				TLSClientConfig: config,
			},
		},
	}
}

// NewClient constructs a Client.
func NewClient() Client {
	return &client{}
}

//
// NewMockClient constructs a Client that calls handleRequest instead of actually
// doing a network request.
//
func NewMockClient(handleRequest func(context.Context, *Request) error) Client {
	return &mockClient{handleRequest}
}

type mockClient struct {
	handleRequest func(context.Context, *Request) error
}

func (mc *mockClient) do(ctx context.Context, method, baseURL string, options ...RequestOption) error {
	var r = Request{
		URL:    baseURL,
		Method: method,
		Params: url.Values{},
	}
	for _, o := range options {
		o(&r)
	}

	return mc.handleRequest(ctx, &r)
}

func (mc *mockClient) Get(ctx context.Context, url string, options ...RequestOption) error {
	return mc.do(ctx, "GET", url, options...)
}

func (mc *mockClient) Post(ctx context.Context, url string, options ...RequestOption) error {
	return mc.do(ctx, "POST", url, options...)
}

func (c *client) Get(ctx context.Context, url string, options ...RequestOption) error {
	return c.do(ctx, "GET", url, options...)
}

func (c *client) Post(ctx context.Context, url string, options ...RequestOption) error {
	return c.do(ctx, "POST", url, options...)
}

func (req *Request) prepareRequest(ctx context.Context) (*http.Request, error) {
	var body io.Reader
	if req.Body != nil {
		j, err := json.Marshal(req.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(j)
	}
	var urlWithParams = req.URL
	if len(req.Params) > 0 {
		urlWithParams += "?" + req.Params.Encode()
	}
	r, err := http.NewRequest(req.Method, urlWithParams, body)
	if err != nil {
		return nil, err
	}
	r = r.WithContext(ctx)

	r.Header = req.Header
	return r, nil
}

type BadStatusError struct {
	Code int
	Body []byte
}

func (bse *BadStatusError) Error() string {
	return fmt.Sprintf("Got HTTP %d (%s): %q", bse.Code, http.StatusText(bse.Code), string(bse.Body))
}

func (req *Request) handleResponse(httpResp *http.Response) error {
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		buf, _ := ioutil.ReadAll(httpResp.Body)

		return &BadStatusError{Code: httpResp.StatusCode, Body: buf}
	}

	if req.Output != nil {
		if _, err := io.Copy(req.Output, httpResp.Body); err != nil {
			return err
		}
	} else if req.JSONOutput != nil {
		buf, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return err
		}

		if err = json.Unmarshal(buf, req.JSONOutput); err != nil {
			return err
		}
	}

	return nil
}

func (c *client) do(ctx context.Context, method, baseURL string, options ...RequestOption) error {
	var req = Request{
		Method: method,
		URL:    baseURL,
		Params: url.Values{},
		Header: http.Header{},
	}
	for _, o := range options {
		o(&req)
	}

	r, err := req.prepareRequest(ctx)
	if err != nil {
		return err
	}

	httpResp, err := c.client.Do(r)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	return req.handleResponse(httpResp)
}
