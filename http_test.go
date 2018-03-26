package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"reflect"
	"time"
)

// Test that our http client will properly respect the cancellation of its context.
//
// We do this by making a request handler that first cancels the context
// then blocks until the test completes.
//
func TestGet_cancel(t *testing.T) {
	t.Parallel()
	doneChan := make(chan interface{})
	ctx, cancel := context.WithCancel(context.Background())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		select {
		case <-doneChan:
			// Expected
		case <-time.After(1 * time.Second):
			t.Error("took more than 1s to see cancellation")
		}
	}))
	defer srv.Close()

	err := NewClient().Get(ctx, srv.URL)
	if err == nil {
		t.Errorf("expected timeout, not success")
	}
	// Note this _must_ execute before srv.Close() so that the handler's
	// goroutine is allowed to finish before the server terminates.
	close(doneChan)
}

func TestGet_status(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`hello`))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := NewClient().Get(ctx, srv.URL)
	wantErr := &BadStatusError{Code: http.StatusTeapot, Body: []byte(`hello`)}
	if !reflect.DeepEqual(wantErr, err) {
		t.Errorf("Get() error = %v, want %v", err, wantErr)
	}
}

func TestGet_header(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-test-header") != "header-value" {
			t.Error("header not set", r.Header)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := NewClient().Get(ctx, srv.URL, WithHeader("x-test-header", "header-value")); err != nil {
		t.Errorf("Get() error = %v", err)
	}
}

func TestGet_headerParam(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-test-header") != "header-value" {
			t.Error("header not set", r.Header)
		}
		if r.URL.Query().Get("param") != "value" {
			t.Error("Unexpected param", r.URL.Query())
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := NewClient().Get(ctx, srv.URL,
		WithHeader("x-test-header", "header-value"),
		WithParam("param", "value")); err != nil {
		t.Errorf("Get() error = %v", err)
	}
}

func TestGet_param_simple(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "param=value" {
			t.Errorf("Unexpected url %s", r.URL)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := NewClient().Get(ctx, srv.URL, WithParam("param", "value")); err != nil {
		t.Errorf("Get() error = %v", err)
	}
}

func TestGet_param_multi(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "param1=value1&param2=value2" && r.URL.RawQuery != "param2=value2&param1=value1" {
			t.Errorf("Unexpected url %s", r.URL)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := NewClient().Get(ctx, srv.URL, WithParam("param1", "value1"), WithParam("param2", "value2")); err != nil {
		t.Errorf("Get() error = %v", err)
	}
}

func TestGet_json_response(t *testing.T) {
	t.Parallel()
	var currentHandler http.HandlerFunc
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentHandler(w, r)
	}))
	defer srv.Close()

	type payloadType struct {
		Name string
	}
	jsonTestCases := []struct {
		name       string
		respBody   string
		respStatus int
		wantErr    bool
		wantResp   payloadType
	}{
		{
			name:     "simple json response",
			respBody: `{"name": "alex"}`,
			wantResp: payloadType{Name: "alex"},
		},
		{
			name:     "invalid json response",
			respBody: `<html></html>`,
			wantErr:  true,
		},
		{
			name:       "bad return code",
			respBody:   `{"name": "alex"}`,
			respStatus: http.StatusNotFound,
			wantErr:    true,
		},
	}
	for _, tt := range jsonTestCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			currentHandler = func(w http.ResponseWriter, r *http.Request) {
				if tt.respStatus != 0 {
					w.WriteHeader(tt.respStatus)
				}
				w.Write([]byte(tt.respBody))
			}

			var resp payloadType
			if err := NewClient().Get(ctx, srv.URL, WithJSONResponse(&resp)); (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.wantResp, resp) {
				t.Errorf("Get() %v, want %v", resp, tt.wantResp)
			}
		})
	}
}
