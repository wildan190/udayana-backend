package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type LocalProductImages struct {
	dir string
}

func NewLocalProductImages(dir string) (*LocalProductImages, error) {
	if dir == "" {
		dir = "./uploads"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &LocalProductImages{dir: dir}, nil
}

func (l *LocalProductImages) Upload(ctx context.Context, key string, contentType string, body io.Reader, size int64) error {
	if !objectKeyPattern.MatchString(key) {
		return errors.New("kunci gambar produk tidak valid")
	}
	destPath := filepath.Join(l.dir, key)
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, body)
	return err
}

func (l *LocalProductImages) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if !objectKeyPattern.MatchString(key) {
		return nil, "", ErrObjectNotFound
	}
	destPath := filepath.Join(l.dir, key)
	file, err := os.Open(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", ErrObjectNotFound
		}
		return nil, "", err
	}

	contentType := "image/jpeg"
	if len(key) > 4 {
		ext := key[len(key)-4:]
		if ext == ".png" {
			contentType = "image/png"
		} else if len(key) > 5 && key[len(key)-5:] == ".webp" {
			contentType = "image/webp"
		}
	}

	return file, contentType, nil
}

var _ ProductImages = (*LocalProductImages)(nil)
