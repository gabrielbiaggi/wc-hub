package docker

// Client intentionally accepts an explicit endpoint. Never mount the host
// Docker socket into the public API container; use a restricted socket proxy.
type Client struct{ endpoint string }

func New(endpoint string) *Client { return &Client{endpoint: endpoint} }
