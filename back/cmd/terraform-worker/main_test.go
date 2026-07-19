package main

import "testing"

func TestSummarizePlan(t *testing.T) {
	input := []byte(`{"resource_changes":[{"change":{"actions":["create"]}},{"change":{"actions":["update"]}},{"change":{"actions":["delete"]}},{"change":{"actions":["delete","create"]}}]}`)
	got := summarizePlan(input)
	if got.Add != 1 || got.Change != 2 || got.Destroy != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

func TestRedact(t *testing.T) {
	got := redact("token=abc password: xyz safe=value")
	if got != "token=[REDACTED] password: [REDACTED] safe=value" {
		t.Fatalf("unexpected redaction: %s", got)
	}
}
