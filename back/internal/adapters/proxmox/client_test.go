package proxmox

import "testing"

func TestRequiresHTTPS(t *testing.T) {
	if _, err := New("http://pve.local:8006", "user@pam!hub", []byte("secret"), ""); err == nil {
		t.Fatal("insecure Proxmox URL accepted")
	}
}
