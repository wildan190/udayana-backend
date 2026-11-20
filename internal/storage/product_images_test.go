package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3ProductImagesUploadAndOpen(t *testing.T) {
	const key = "products-0123456789abcdef0123456789abcdef.jpg"
	var saved string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-bucket/"+key {
			t.Errorf("request path = %q, want %q", r.URL.Path, "/test-bucket/"+key)
		}
		switch r.Method {
		case http.MethodPut:
			if got := r.Header.Get("Content-Type"); got != "image/jpeg" {
				t.Errorf("PUT Content-Type = %q, want image/jpeg", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read PUT body: %v", err)
			}
			saved = string(body)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = io.WriteString(w, saved)
		default:
			t.Errorf("unexpected S3 method %q", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	images, err := NewS3ProductImages(
		server.URL, "test-bucket", "us-east-1", "test-access", "test-secret", true,
	)
	if err != nil {
		t.Fatalf("NewS3ProductImages() error = %v", err)
	}
	if err := images.Upload(
		context.Background(), key, "image/jpeg", strings.NewReader("jpeg bytes"), 10,
	); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	image, contentType, err := images.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer image.Close()
	if contentType != "image/jpeg" {
		t.Fatalf("Open() Content-Type = %q, want image/jpeg", contentType)
	}
	body, err := io.ReadAll(image)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(body) != "jpeg bytes" {
		t.Fatalf("Open() body = %q, want uploaded bytes", body)
	}
}
