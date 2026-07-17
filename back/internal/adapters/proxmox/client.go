package proxmox

import "net/http"

// Client is the Proxmox adapter boundary. Authentication uses API tokens only;
// root passwords and TLS verification bypasses are deliberately unsupported.
type Client struct {
	baseURL string
	tokenID string
	secret  []byte
	http    *http.Client
}

func New(baseURL, tokenID string, secret []byte, transport *http.Client) *Client {
	return &Client{baseURL: baseURL, tokenID: tokenID, secret: secret, http: transport}
}
