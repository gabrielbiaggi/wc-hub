// Package vnc exposes only explicitly configured VNC targets through a
// binary WebSocket-to-TCP bridge. It never receives or stores VNC passwords.
package vnc

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Target struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}
type Config struct {
	Targets     []string
	DialTimeout time.Duration
}
type Gateway struct {
	targets     map[string]Target
	dialTimeout time.Duration
	upgrader    websocket.Upgrader
}

func New(config Config) (*Gateway, error) {
	targets := map[string]Target{}
	for _, raw := range config.Targets {
		id, address, ok := strings.Cut(strings.TrimSpace(raw), "=")
		if !ok || !validID(id) {
			return nil, fmt.Errorf("VNC target must use id=host:port")
		}
		if _, _, err := net.SplitHostPort(strings.TrimSpace(address)); err != nil {
			return nil, fmt.Errorf("invalid VNC target %s", id)
		}
		if _, exists := targets[id]; exists {
			return nil, fmt.Errorf("duplicate VNC target %s", id)
		}
		targets[id] = Target{ID: id, Address: strings.TrimSpace(address)}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one VNC target is required")
	}
	timeout := config.DialTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Gateway{targets: targets, dialTimeout: timeout, upgrader: websocket.Upgrader{ReadBufferSize: 32 << 10, WriteBufferSize: 32 << 10, CheckOrigin: sameOrigin}}, nil
}
func validID(value string) bool {
	if len(value) < 1 || len(value) > 80 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func (g *Gateway) Targets() []Target {
	result := make([]Target, 0, len(g.targets))
	for _, item := range g.targets {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (g *Gateway) Serve(w http.ResponseWriter, r *http.Request, targetID string) error {
	target, ok := g.targets[targetID]
	if !ok {
		return fmt.Errorf("VNC target is not allowlisted")
	}
	connection, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer connection.Close()
	connection.SetReadLimit(2 << 20)
	tcp, err := net.DialTimeout("tcp", target.Address, g.dialTimeout)
	if err != nil {
		return err
	}
	defer tcp.Close()
	errCh := make(chan error, 2)
	var once sync.Once
	closeBoth := func() { once.Do(func() { _ = tcp.Close(); _ = connection.Close() }) }
	go func() {
		defer closeBoth()
		for {
			kind, payload, readErr := connection.ReadMessage()
			if readErr != nil {
				errCh <- readErr
				return
			}
			if kind != websocket.BinaryMessage {
				errCh <- fmt.Errorf("VNC bridge accepts binary frames only")
				return
			}
			if _, writeErr := tcp.Write(payload); writeErr != nil {
				errCh <- writeErr
				return
			}
		}
	}()
	go func() {
		defer closeBoth()
		buffer := make([]byte, 32<<10)
		for {
			count, readErr := tcp.Read(buffer)
			if count > 0 {
				if writeErr := connection.WriteMessage(websocket.BinaryMessage, buffer[:count]); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					errCh <- nil
				} else {
					errCh <- readErr
				}
				return
			}
		}
	}()
	return <-errCh
}
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}
