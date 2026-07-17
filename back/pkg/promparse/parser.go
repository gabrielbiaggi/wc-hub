package promparse

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Sample struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

func Parse(reader io.Reader, allow func(string) bool) ([]Sample, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, 16<<20))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	result := []Sample{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		space := strings.LastIndexAny(line, " \t")
		if space < 1 {
			continue
		}
		left, valueText := line[:space], strings.TrimSpace(line[space+1:])
		name := left
		labels := map[string]string{}
		if brace := strings.IndexByte(left, '{'); brace >= 0 {
			name = left[:brace]
			raw := strings.TrimSuffix(left[brace+1:], "}")
			for _, part := range splitLabels(raw) {
				pair := strings.SplitN(part, "=", 2)
				if len(pair) == 2 {
					labels[strings.TrimSpace(pair[0])] = strings.Trim(strings.TrimSpace(pair[1]), `"`)
				}
			}
		}
		if !allow(name) {
			continue
		}
		value, err := strconv.ParseFloat(valueText, 64)
		if err != nil {
			continue
		}
		result = append(result, Sample{Name: name, Labels: labels, Value: value})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse prometheus metrics: %w", err)
	}
	return result, nil
}
func splitLabels(value string) []string {
	var result []string
	start := 0
	quoted := false
	escaped := false
	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quoted {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
		}
		if r == ',' && !quoted {
			result = append(result, value[start:i])
			start = i + 1
		}
	}
	if start < len(value) {
		result = append(result, value[start:])
	}
	return result
}
