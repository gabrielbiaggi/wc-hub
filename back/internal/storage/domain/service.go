package domain

import (
	"context"
	"io"
)

type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Directory bool   `json:"directory"`
}
type Service interface {
	List(context.Context, string, string) ([]Entry, error)
	Open(context.Context, string, string) (io.ReadCloser, error)
	Upload(context.Context, string, string, io.Reader) error
}
