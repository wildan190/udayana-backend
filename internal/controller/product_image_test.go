package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testProductImages struct {
	key         string
	contentType string
	body        []byte
}

func (s *testProductImages) Upload(
	_ context.Context,
	key string,
	contentType string,
	body io.Reader,
	_ int64,
) error {
	s.key = key
	s.contentType = contentType
	var err error
	s.body, err = io.ReadAll(body)
	return err
}

func (s *testProductImages) Open(
	_ context.Context,
	_ string,
) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader(s.body)), s.contentType, nil
}

func TestUploadAndServeProductImage(t *testing.T) {
	images := &testProductImages{}
	controller := &Controller{images: images}
	imageBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("image", "product.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(imageBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	controller.uploadProductImage(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body)
	}
	if !strings.HasPrefix(images.key, "products-") || images.contentType != "image/jpeg" {
		t.Fatalf("stored image = (%q, %q)", images.key, images.contentType)
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["image_url"] != "/api/v1/product-images/"+images.key {
		t.Fatalf("image_url = %q", result["image_url"])
	}

	imageRequest := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/product-images/"+images.key,
		nil,
	)
	imageRequest.SetPathValue("key", images.key)
	imageResponse := httptest.NewRecorder()
	controller.Routes().ServeHTTP(imageResponse, imageRequest)

	if imageResponse.Code != http.StatusOK ||
		imageResponse.Header().Get("Content-Type") != "image/jpeg" ||
		!bytes.Equal(imageResponse.Body.Bytes(), imageBytes) {
		t.Fatalf("served image response = (%d, %q, %v)", imageResponse.Code, imageResponse.Header().Get("Content-Type"), imageResponse.Body.Bytes())
	}
}

func TestUploadProductImageRejectsUnsupportedContent(t *testing.T) {
	controller := &Controller{images: &testProductImages{}}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("image", "not-an-image.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "plain text"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-images", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	controller.uploadProductImage(response, request)

	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("upload status = %d, want %d", response.Code, http.StatusUnsupportedMediaType)
	}
}
