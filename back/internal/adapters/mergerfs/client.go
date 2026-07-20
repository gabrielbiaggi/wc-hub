package mergerfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var ErrPathDenied = errors.New("storage path is outside the configured root")

type Config struct {
	Root              string
	SSHAddress        string
	SSHUser           string
	SSHRoot           string
	SSHPrivateKeyPath string
	SSHKnownHostsPath string
}
type Client struct {
	root   string
	remote *remoteConfig
}
type remoteConfig struct {
	address, user, root string
	signer              ssh.Signer
	hostKey             ssh.HostKeyCallback
}
type Entry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	Directory  bool      `json:"directory"`
	ModifiedAt time.Time `json:"modified_at"`
	MIMEType   string    `json:"mime_type,omitempty"`
}

func New(root string) (*Client, error) { return NewWithConfig(Config{Root: root}) }

// NewWithConfig supports either a directly mounted filesystem or an SFTP
// connection to the Proxmox host that owns the MergerFS mount. Docker Desktop
// cannot reliably bind Windows SMB mapped drives, so the remote mode keeps the
// browser connected to the real pool without copying files through Windows.
func NewWithConfig(config Config) (*Client, error) {
	if strings.TrimSpace(config.SSHAddress) != "" || strings.TrimSpace(config.SSHRoot) != "" {
		if strings.TrimSpace(config.SSHAddress) == "" || strings.TrimSpace(config.SSHRoot) == "" || strings.TrimSpace(config.SSHPrivateKeyPath) == "" || strings.TrimSpace(config.SSHKnownHostsPath) == "" {
			return nil, errors.New("MergerFS SSH mode requires address, root, private key and known_hosts")
		}
		key, err := os.ReadFile(config.SSHPrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read MergerFS SSH private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse MergerFS SSH private key: %w", err)
		}
		hostKey, err := knownhosts.New(config.SSHKnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("load MergerFS known_hosts: %w", err)
		}
		root := path.Clean(config.SSHRoot)
		if !path.IsAbs(root) || root == "/" {
			return nil, errors.New("MergerFS SSH root must be a non-root absolute path")
		}
		user := strings.TrimSpace(config.SSHUser)
		if user == "" {
			user = "root"
		}
		return &Client{root: root, remote: &remoteConfig{address: strings.TrimSpace(config.SSHAddress), user: user, root: root, signer: signer, hostKey: hostKey}}, nil
	}
	if strings.TrimSpace(config.Root) == "" {
		return nil, errors.New("MergerFS root is required")
	}
	absolute, err := filepath.Abs(config.Root)
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
	if c.remote != nil {
		return c.remoteBrowse(ctx, relative)
	}
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
			return nil, err
		}
		result = append(result, c.entry(filepath.Join(target, item.Name()), info))
	}
	sortEntries(result)
	return result, nil
}
func (c *Client) Index(ctx context.Context, relative string, limit int) ([]Entry, error) {
	if c.remote != nil {
		return c.remoteIndex(ctx, relative, limit)
	}
	if limit < 1 || limit > 10000 {
		limit = 2000
	}
	target, err := c.resolve(relative)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0)
	err = filepath.WalkDir(target, func(itemPath string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if itemPath == target {
			return nil
		}
		if item.Type()&os.ModeSymlink != 0 {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		result = append(result, c.entry(itemPath, info))
		if len(result) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	return result, err
}
func (c *Client) Open(relative string) (io.ReadCloser, Entry, error) {
	if c.remote != nil {
		return c.remoteOpen(relative)
	}
	target, err := c.resolve(relative)
	if err != nil {
		return nil, Entry{}, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, Entry{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, Entry{}, err
	}
	if info.IsDir() {
		file.Close()
		return nil, Entry{}, errors.New("cannot stream a directory")
	}
	return file, c.entry(target, info), nil
}
func (c *Client) CreateDirectory(parent, name string) (Entry, error) {
	if c.remote != nil {
		return c.remoteMkdir(parent, name)
	}
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
	if c.remote != nil {
		return c.remoteWrite(ctx, parent, name, source)
	}
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
	if c.remote != nil {
		return c.remoteRename(relative, name)
	}
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
	if c.remote != nil {
		return c.remoteDelete(relative)
	}
	target, err := c.resolve(relative)
	if err != nil {
		return err
	}
	if target == c.root {
		return ErrPathDenied
	}
	return os.Remove(target)
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
func (c *Client) newTarget(parent, name string) (string, error) {
	name = strings.TrimSpace(name)
	if !validName(name) {
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
func (c *Client) entry(itemPath string, info os.FileInfo) Entry {
	relative, _ := filepath.Rel(c.root, itemPath)
	return entry(filepath.ToSlash(relative), info)
}
func entry(relative string, info os.FileInfo) Entry {
	kind := ""
	if !info.IsDir() {
		kind = mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name())))
	}
	return Entry{Name: info.Name(), Path: relative, Size: info.Size(), Directory: info.IsDir(), ModifiedAt: info.ModTime().UTC(), MIMEType: kind}
}
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 && !strings.ContainsAny(name, `/\\`)
}
func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Directory != entries[j].Directory {
			return entries[i].Directory
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
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

func (c *Client) remoteClient() (*sftp.Client, *ssh.Client, error) {
	r := c.remote
	connection, err := ssh.Dial("tcp", r.address, &ssh.ClientConfig{User: r.user, Auth: []ssh.AuthMethod{ssh.PublicKeys(r.signer)}, HostKeyCallback: r.hostKey, Timeout: 12 * time.Second})
	if err != nil {
		return nil, nil, fmt.Errorf("connect MergerFS host: %w", err)
	}
	client, err := sftp.NewClient(connection)
	if err != nil {
		connection.Close()
		return nil, nil, fmt.Errorf("open MergerFS SFTP: %w", err)
	}
	return client, connection, nil
}
func (c *Client) remotePath(relative string) (string, error) {
	relative = strings.Trim(strings.ReplaceAll(relative, "\\", "/"), "/")
	clean := path.Clean(relative)
	if clean == "." {
		clean = ""
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrPathDenied
	}
	return path.Join(c.remote.root, clean), nil
}
func (c *Client) remoteNew(parent, name string) (string, error) {
	if !validName(name) {
		return "", ErrPathDenied
	}
	base, err := c.remotePath(parent)
	if err != nil {
		return "", err
	}
	return path.Join(base, name), nil
}
func (c *Client) remoteBrowse(ctx context.Context, relative string) ([]Entry, error) {
	target, err := c.remotePath(relative)
	if err != nil {
		return nil, err
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer client.Close()
	items, err := client.ReadDir(target)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(items))
	for _, info := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result = append(result, entry(path.Join(strings.Trim(relative, "/"), info.Name()), info))
	}
	sortEntries(result)
	return result, nil
}
func (c *Client) remoteIndex(ctx context.Context, relative string, limit int) ([]Entry, error) {
	if limit < 1 || limit > 10000 {
		limit = 2000
	}
	target, err := c.remotePath(relative)
	if err != nil {
		return nil, err
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer client.Close()
	walker := client.Walk(target)
	result := make([]Entry, 0)
	for walker.Step() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if walker.Err() != nil {
			return nil, walker.Err()
		}
		if walker.Path() == target {
			continue
		}
		info := walker.Stat()
		if info == nil {
			continue
		}
		rel := strings.TrimPrefix(walker.Path(), strings.TrimRight(c.remote.root, "/")+"/")
		if rel == walker.Path() || rel == "" {
			return nil, ErrPathDenied
		}
		result = append(result, entry(rel, info))
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}
func (c *Client) remoteOpen(relative string) (io.ReadCloser, Entry, error) {
	target, err := c.remotePath(relative)
	if err != nil {
		return nil, Entry{}, err
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return nil, Entry{}, err
	}
	info, err := client.Stat(target)
	if err != nil {
		client.Close()
		conn.Close()
		return nil, Entry{}, err
	}
	if info.IsDir() {
		client.Close()
		conn.Close()
		return nil, Entry{}, errors.New("cannot stream a directory")
	}
	file, err := client.Open(target)
	if err != nil {
		client.Close()
		conn.Close()
		return nil, Entry{}, err
	}
	return &remoteFile{File: file, client: client, connection: conn}, entry(strings.Trim(strings.TrimPrefix(target, c.remote.root), "/"), info), nil
}

type remoteFile struct {
	*sftp.File
	client     *sftp.Client
	connection *ssh.Client
}

func (f *remoteFile) Close() error {
	fileErr := f.File.Close()
	clientErr := f.client.Close()
	connErr := f.connection.Close()
	if fileErr != nil {
		return fileErr
	}
	if clientErr != nil {
		return clientErr
	}
	return connErr
}
func (c *Client) remoteMkdir(parent, name string) (Entry, error) {
	target, err := c.remoteNew(parent, name)
	if err != nil {
		return Entry{}, err
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return Entry{}, err
	}
	defer conn.Close()
	defer client.Close()
	if err = client.Mkdir(target); err != nil {
		return Entry{}, err
	}
	info, err := client.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	return entry(strings.Trim(strings.TrimPrefix(target, c.remote.root), "/"), info), nil
}
func (c *Client) remoteWrite(ctx context.Context, parent, name string, source io.Reader) (Entry, error) {
	target, err := c.remoteNew(parent, name)
	if err != nil {
		return Entry{}, err
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return Entry{}, err
	}
	defer conn.Close()
	defer client.Close()
	file, err := client.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
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
	info, err := client.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	return entry(strings.Trim(strings.TrimPrefix(target, c.remote.root), "/"), info), nil
}
func (c *Client) remoteRename(relative, name string) (Entry, error) {
	source, err := c.remotePath(relative)
	if err != nil {
		return Entry{}, err
	}
	target, err := c.remoteNew(path.Dir(relative), name)
	if err != nil {
		return Entry{}, err
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return Entry{}, err
	}
	defer conn.Close()
	defer client.Close()
	if err = client.Rename(source, target); err != nil {
		return Entry{}, err
	}
	info, err := client.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	return entry(strings.Trim(strings.TrimPrefix(target, c.remote.root), "/"), info), nil
}
func (c *Client) remoteDelete(relative string) error {
	target, err := c.remotePath(relative)
	if err != nil {
		return err
	}
	if target == c.remote.root {
		return ErrPathDenied
	}
	client, conn, err := c.remoteClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer client.Close()
	info, err := client.Stat(target)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return client.RemoveDirectory(target)
	}
	return client.Remove(target)
}
