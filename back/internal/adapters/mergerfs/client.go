package mergerfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrPathDenied = errors.New("storage path is outside the configured root")

type Client struct{ root string }
type Entry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Directory  bool      `json:"directory"`
	ModifiedAt time.Time `json:"modified_at"`
	MIMEType   string    `json:"mime_type,omitempty"`
}

func New(root string) (*Client, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("MergerFS root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve MergerFS root: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return nil, errors.New("MergerFS root must be an existing directory")
	}
	return &Client{root: resolved}, nil
}
func (c *Client) Browse(ctx context.Context, relative string) ([]Entry, error) {
	target, err := c.resolve(relative)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(items))
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		result = append(result, c.entry(filepath.Join(target, item.Name()), info))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Directory != result[j].Directory {
			return result[i].Directory
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}
func (c *Client) Index(ctx context.Context, relative string, limit int) ([]Entry, error) {
	if limit < 1 || limit > 10000 {
		limit = 2000
	}
	target, err := c.resolve(relative)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0)
	err = filepath.WalkDir(target, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == target {
			return nil
		}
		if item.Type()&os.ModeSymlink != 0 {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := item.Info()
		if err == nil {
			result = append(result, c.entry(path, info))
		}
		if len(result) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	return result, err
}
func (c *Client) Open(relative string) (*os.File, os.FileInfo, error) {
	target, err := c.resolve(relative)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		file.Close()
		return nil, nil, errors.New("cannot stream a directory")
	}
	return file, info, nil
}

func (c *Client) CreateDirectory(parent, name string) (Entry, error) {
	target, err := c.newTarget(parent, name)
	if err != nil {
		return Entry{}, err
	}
	if err = os.Mkdir(target, 0o750); err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	return c.entry(target, info), nil
}

func (c *Client) WriteFile(ctx context.Context, parent, name string, source io.Reader) (Entry, error) {
	target, err := c.newTarget(parent, name)
	if err != nil {
		return Entry{}, err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return Entry{}, err
	}
	_, copyErr := io.Copy(file, &contextReader{ctx: ctx, reader: io.LimitReader(source, 256<<20)})
	closeErr := file.Close()
	if copyErr != nil {
		return Entry{}, copyErr
	}
	if closeErr != nil {
		return Entry{}, closeErr
	}
	info, err := os.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	return c.entry(target, info), nil
}

func (c *Client) Rename(relative, name string) (Entry, error) {
	source, err := c.resolve(relative)
	if err != nil {
		return Entry{}, err
	}
	target, err := c.newTarget(filepath.ToSlash(filepath.Dir(relative)), name)
	if err != nil {
		return Entry{}, err
	}
	if err = os.Rename(source, target); err != nil {
		return Entry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	return c.entry(target, info), nil
}

func (c *Client) Delete(relative string) error {
	target, err := c.resolve(relative)
	if err != nil {
		return err
	}
	if target == c.root {
		return ErrPathDenied
	}
	return os.Remove(target)
}

func (c *Client) newTarget(parent, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || len(name) > 255 || strings.ContainsAny(name, `/\\`) {
		return "", ErrPathDenied
	}
	parentPath, err := c.resolve(parent)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(parentPath)
	if err != nil || !info.IsDir() {
		return "", ErrPathDenied
	}
	target := filepath.Join(parentPath, name)
	rel, err := filepath.Rel(c.root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathDenied
	}
	return target, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}
func (c *Client) resolve(relative string) (string, error) {
	relative = strings.TrimSpace(strings.ReplaceAll(relative, "\\", "/"))
	relative = strings.TrimPrefix(relative, "/")
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." {
		clean = ""
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrPathDenied
	}
	candidate := filepath.Join(c.root, clean)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(c.root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathDenied
	}
	return resolved, nil
}
func (c *Client) entry(path string, info os.FileInfo) Entry {
	relative, _ := filepath.Rel(c.root, path)
	relative = filepath.ToSlash(relative)
	kind := ""
	if !info.IsDir() {
		kind = mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	}
	return Entry{Name: info.Name(), Path: relative, Size: info.Size(), Directory: info.IsDir(), ModifiedAt: info.ModTime().UTC(), MIMEType: kind}
}
