package yamlsyntax

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Violation records a YAML syntax error.
type Violation struct {
	File    string
	Line    int
	Column  int
	Message string
}

// Filter returns YAML files.
func Filter(files []string) []string {
	var out []string
	for _, f := range files {
		l := strings.ToLower(f)
		if strings.HasSuffix(l, ".yaml") || strings.HasSuffix(l, ".yml") {
			out = append(out, f)
		}
	}
	return out
}

// CheckFile parses a YAML file and returns syntax violations.
func CheckFile(path string) ([]Violation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return CheckBytes(path, data), nil
}

// CheckBytes parses YAML bytes and returns syntax violations.
func CheckBytes(filename string, data []byte) []Violation {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return []Violation{{File: filename, Message: err.Error()}}
		}
	}
}

// CheckFiles checks all YAML files; returns concatenated violations.
func CheckFiles(files []string) ([]Violation, error) {
	files = Filter(files)
	var all []Violation
	for _, f := range files {
		v, err := CheckFile(f)
		if err != nil {
			return nil, fmt.Errorf("checking %s: %w", f, err)
		}
		all = append(all, v...)
	}
	return all, nil
}
