package github

import "net/http"

type Client struct {
	token []byte
	http  *http.Client
}

func New(token []byte, transport *http.Client) *Client { return &Client{token: token, http: transport} }
