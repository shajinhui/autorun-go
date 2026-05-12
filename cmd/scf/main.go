package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	handler "autorun-go/api"

	"github.com/tencentyun/scf-go-lib/cloudfunction"
)

type scfEvent struct {
	HTTPMethod string            `json:"httpMethod"`
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Message    string            `json:"Message"`
}

// HandleSCF adapts SCF event payloads to the existing HTTP handler.
func HandleSCF(ctx context.Context, event scfEvent) (string, error) {
	body := strings.TrimSpace(event.Body)
	if body == "" {
		body = strings.TrimSpace(event.Message)
	}

	method := strings.ToUpper(strings.TrimSpace(event.HTTPMethod))
	if method == "" {
		if body != "" {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}

	path := strings.TrimSpace(event.Path)
	if path == "" {
		path = "/api"
	}

	req, err := http.NewRequestWithContext(ctx, method, path, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	for k, v := range event.Headers {
		if strings.TrimSpace(k) != "" {
			req.Header.Set(k, v)
		}
	}
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler.Handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(respBody)), nil
}

func main() {
	cloudfunction.Start(HandleSCF)
}

