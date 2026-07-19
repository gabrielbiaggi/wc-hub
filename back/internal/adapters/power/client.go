package power

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	NUTAddress string
	UPSName    string
	WOLTargets []string
	Broadcast  string
}

type Target struct {
	ID  string `json:"id"`
	MAC string `json:"mac"`
}

type Status struct {
	Configured     bool      `json:"configured"`
	Online         bool      `json:"online"`
	BatteryPercent *int      `json:"batteryPercent,omitempty"`
	LoadPercent    *int      `json:"loadPercent,omitempty"`
	RuntimeSeconds *int      `json:"runtimeSeconds,omitempty"`
	UPSStatus      string    `json:"upsStatus,omitempty"`
	CheckedAt      time.Time `json:"checkedAt"`
	Error          string    `json:"error,omitempty"`
}

type Client struct {
	nutAddress string
	upsName    string
	targets    map[string]Target
	broadcast  *net.UDPAddr
}

func New(cfg Config) (*Client, error) {
	client := &Client{nutAddress: strings.TrimSpace(cfg.NUTAddress), upsName: strings.TrimSpace(cfg.UPSName), targets: map[string]Target{}}
	if client.upsName != "" && !safeName(client.upsName) {
		return nil, fmt.Errorf("invalid NUT UPS name")
	}
	for _, raw := range cfg.WOLTargets {
		id, mac, ok := strings.Cut(strings.TrimSpace(raw), "=")
		id, mac = strings.TrimSpace(id), strings.TrimSpace(mac)
		if !ok || !safeName(id) {
			return nil, fmt.Errorf("invalid WOL target %q", raw)
		}
		parsed, err := net.ParseMAC(mac)
		if err != nil || len(parsed) != 6 {
			return nil, fmt.Errorf("invalid WOL MAC for %s", id)
		}
		client.targets[id] = Target{ID: id, MAC: parsed.String()}
	}
	if len(client.targets) > 0 {
		address := strings.TrimSpace(cfg.Broadcast)
		if address == "" {
			address = "255.255.255.255:9"
		}
		var err error
		client.broadcast, err = net.ResolveUDPAddr("udp4", address)
		if err != nil {
			return nil, fmt.Errorf("parse WOL broadcast address: %w", err)
		}
	}
	return client, nil
}

func (c *Client) Targets() []Target {
	items := make([]Target, 0, len(c.targets))
	for _, target := range c.targets {
		items = append(items, target)
	}
	return items
}

func (c *Client) Status(ctx context.Context) Status {
	status := Status{Configured: c.nutAddress != "", CheckedAt: time.Now().UTC()}
	if c.nutAddress == "" || c.upsName == "" {
		status.Error = "NUT não configurado"
		return status
	}
	dialer := net.Dialer{Timeout: 6 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", c.nutAddress)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err = fmt.Fprintf(connection, "LIST VAR %s\n", c.upsName); err != nil {
		status.Error = err.Error()
		return status
	}
	values := map[string]string{}
	scanner := bufio.NewScanner(connection)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ERR ") {
			status.Error = line
			return status
		}
		if strings.HasPrefix(line, "END LIST VAR ") {
			break
		}
		key, value, ok := parseVariable(c.upsName, line)
		if ok {
			values[key] = value
		}
	}
	if err = scanner.Err(); err != nil {
		status.Error = err.Error()
		return status
	}
	status.Online = true
	status.UPSStatus = values["ups.status"]
	status.BatteryPercent = integerValue(values["battery.charge"])
	status.LoadPercent = integerValue(values["ups.load"])
	status.RuntimeSeconds = integerValue(values["battery.runtime"])
	return status
}

func (c *Client) Wake(ctx context.Context, targetID string) (Target, error) {
	target, ok := c.targets[targetID]
	if !ok || c.broadcast == nil {
		return Target{}, fmt.Errorf("WOL target is not allowed")
	}
	mac, err := net.ParseMAC(target.MAC)
	if err != nil {
		return Target{}, fmt.Errorf("parse target MAC: %w", err)
	}
	packet := make([]byte, 6+16*len(mac))
	for index := 0; index < 6; index++ {
		packet[index] = 0xff
	}
	for index := 6; index < len(packet); index += len(mac) {
		copy(packet[index:], mac)
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	connection, err := dialer.DialContext(ctx, "udp4", c.broadcast.String())
	if err != nil {
		return Target{}, err
	}
	defer connection.Close()
	if _, err = connection.Write(packet); err != nil {
		return Target{}, err
	}
	return target, nil
}

func parseVariable(upsName, line string) (string, string, bool) {
	prefix := "VAR " + upsName + " "
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	key, raw, ok := strings.Cut(strings.TrimPrefix(line, prefix), " ")
	if !ok {
		return "", "", false
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", "", false
	}
	return key, value, true
}

func integerValue(value string) *int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return &parsed
}

func safeName(value string) bool {
	if value == "" || len(value) > 120 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}
