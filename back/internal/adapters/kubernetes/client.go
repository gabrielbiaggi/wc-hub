package kubernetes

type Client struct{ kubeconfigPath string }

func New(kubeconfigPath string) *Client { return &Client{kubeconfigPath: kubeconfigPath} }
