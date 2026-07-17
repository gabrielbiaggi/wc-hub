package application

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	repo "github.com/webcreations/wc-hub/back/internal/terminal/repository"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

type Gateway struct {
	repo               *repo.Postgres
	signer             ssh.Signer
	hostKey            ssh.HostKeyCallback
	upgrader           websocket.Upgrader
	audit              func(context.Context, repo.Target, string, string)
	expectedOriginHost string
}

func (g *Gateway) SetAudit(callback func(context.Context, repo.Target, string, string)) {
	g.audit = callback
}

type clientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

func NewGateway(repository *repo.Postgres, keyPath, knownHostsPath, publicURL string) (*Gateway, error) {
	if keyPath == "" || knownHostsPath == "" {
		return nil, fmt.Errorf("SSH private key and known_hosts paths are required")
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, err
	}
	expected, err := url.Parse(publicURL)
	if err != nil {
		return nil, err
	}
	return &Gateway{repo: repository, signer: signer, hostKey: callback, expectedOriginHost: expected.Host, upgrader: websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, Subprotocols: []string{"wc-hub-terminal"}, CheckOrigin: func(r *http.Request) bool {
		origin, err := url.Parse(r.Header.Get("Origin"))
		return err == nil && origin.Host == expected.Host
	}}}, nil
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin, originErr := url.Parse(r.Header.Get("Origin"))
	if originErr != nil || origin.Host != g.expectedOriginHost {
		http.Error(w, "origin rejected", http.StatusForbidden)
		return
	}
	protocols := websocket.Subprotocols(r)
	if len(protocols) != 2 || protocols[0] != "wc-hub-terminal" {
		http.Error(w, "terminal protocol rejected", http.StatusBadRequest)
		return
	}
	target, err := g.repo.Consume(r.Context(), protocols[1])
	if err != nil {
		http.Error(w, "invalid or expired terminal ticket", http.StatusUnauthorized)
		return
	}
	connection, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		_ = g.repo.Close(r.Context(), target.SessionID, "upgrade_failed")
		return
	}
	defer connection.Close()
	if g.audit != nil {
		g.audit(context.Background(), target, "opened", "one-use ticket consumed")
	}
	defer func() {
		if g.audit != nil {
			g.audit(context.Background(), target, "closed", "websocket session ended")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Hour)
	defer cancel()
	address := net.JoinHostPort(target.Address, strconv.Itoa(target.Port))
	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{User: target.SSHUser, Auth: []ssh.AuthMethod{ssh.PublicKeys(g.signer)}, HostKeyCallback: g.hostKey, Timeout: 10 * time.Second})
	if err != nil {
		_ = connection.WriteJSON(clientMessage{Type: "error", Data: err.Error()})
		_ = g.repo.Close(context.Background(), target.SessionID, "connect_failed")
		return
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		_ = g.repo.Close(context.Background(), target.SessionID, "session_failed")
		return
	}
	defer session.Close()
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err = session.RequestPty("xterm-256color", 32, 120, modes); err != nil {
		return
	}
	if err = session.Shell(); err != nil {
		return
	}
	var writeMu sync.Mutex
	writeOutput := func(reader io.Reader) {
		buffer := make([]byte, 4096)
		for {
			count, readErr := reader.Read(buffer)
			if count > 0 {
				writeMu.Lock()
				_ = connection.WriteJSON(clientMessage{Type: "output", Data: string(buffer[:count])})
				writeMu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}
	go writeOutput(stdout)
	go writeOutput(stderr)
	for {
		select {
		case <-ctx.Done():
			_ = g.repo.Close(context.Background(), target.SessionID, "timeout")
			return
		default:
			var message clientMessage
			if err = connection.ReadJSON(&message); err != nil {
				_ = g.repo.Close(context.Background(), target.SessionID, "closed")
				return
			}
			switch message.Type {
			case "input":
				_, _ = io.WriteString(stdin, message.Data)
			case "resize":
				if message.Cols > 0 && message.Rows > 0 {
					_ = session.WindowChange(message.Rows, message.Cols)
				}
			}
		}
	}
}
