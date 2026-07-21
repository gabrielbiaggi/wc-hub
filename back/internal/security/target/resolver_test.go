package target

import (
	"testing"
)

func TestTargetResolver(t *testing.T) {
	resolver := NewResolver()
	resolver.containerIDs = append(resolver.containerIDs, "02843a22a2af")

	tests := []struct {
		name     string
		check    func() bool
		expected bool
	}{
		{"container wc-hub", func() bool { return resolver.IsSelfProtectedContainer("wc-hub") }, true},
		{"canonical docker target", func() bool { return resolver.IsSelfProtectedContainer("docker/container/wc-hub") }, true},
		{"full docker id from hostname prefix", func() bool { return resolver.IsSelfProtectedContainer("docker/container/02843a22a2af8e46dd7d83ed5f4a7850") }, true},
		{"container prefix false positive", func() bool { return resolver.IsSelfProtectedContainer("hubspot") }, false},
		{"container my-app", func() bool { return resolver.IsSelfProtectedContainer("my-app") }, false},
		{"pod wc-hub-api", func() bool { return resolver.IsSelfProtectedPod("wc-hub-api-79d8f") }, true},
		{"canonical kubernetes target", func() bool { return resolver.IsSelfProtectedPod("k8s/default/pod/wc-hub-api-79d8f") }, true},
		{"pod nginx", func() bool { return resolver.IsSelfProtectedPod("nginx-deployment") }, false},
		{"workspace hub-infrastructure", func() bool { return resolver.IsSelfProtectedWorkspace("hub-infrastructure") }, true},
		{"canonical terraform target", func() bool { return resolver.IsSelfProtectedWorkspace("terraform/workspace/hub-infrastructure") }, true},
		{"workspace staging-app", func() bool { return resolver.IsSelfProtectedWorkspace("staging-app") }, false},
		{"host 127.0.0.1", func() bool { return resolver.IsSelfProtectedHost("127.0.0.1") }, true},
		{"host 10.99.99.99", func() bool { return resolver.IsSelfProtectedHost("10.99.99.99") }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.check(); got != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.expected, got)
			}
		})
	}
}
