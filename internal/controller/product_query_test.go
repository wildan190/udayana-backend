package controller

import (
	"net/http/httptest"
	"testing"
)

func TestPositiveQueryInt(t *testing.T) {
	tests := []struct {
		name     string
		rawQuery string
		key      string
		fallback int
		want     int
		wantErr  bool
	}{
		{name: "default", fallback: 20, want: 20},
		{name: "valid", rawQuery: "?page=3", key: "page", fallback: 1, want: 3},
		{name: "zero", rawQuery: "?page=0", key: "page", wantErr: true},
		{name: "not a number", rawQuery: "?page=abc", key: "page", wantErr: true},
		{name: "duplicate", rawQuery: "?page=1&page=2", key: "page", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/v1/products"+test.rawQuery, nil)
			got, err := positiveQueryInt(request, test.key, test.fallback)
			if (err != nil) != test.wantErr {
				t.Fatalf("positiveQueryInt() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("positiveQueryInt() = %d, want %d", got, test.want)
			}
		})
	}
}
