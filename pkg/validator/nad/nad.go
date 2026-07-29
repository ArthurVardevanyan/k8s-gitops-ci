package nad

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidationError records NAD structure issues.
type ValidationError struct {
	File, Message string
}

func (e ValidationError) String() string { return fmt.Sprintf("%s: %s", e.File, e.Message) }

// IsNADFile reports whether a path looks like a NetworkAttachmentDefinition file.
func IsNADFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// ValidateFiles validates a list of files for NAD config emptiness.
func ValidateFiles(files []string) []ValidationError {
	var errs []ValidationError
	for _, f := range files {
		if !IsNADFile(f) {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		errs = append(errs, validateBytes(data, f)...)
	}
	return errs
}

// ValidateDir walks a directory and validates NAD files.
func ValidateDir(dir string) []ValidationError {
	var errs []ValidationError
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs = append(errs, ValidationError{File: path, Message: fmt.Sprintf("walking directory: %v", err)})
			return nil
		}
		if info.IsDir() || !IsNADFile(path) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			errs = append(errs, ValidationError{File: path, Message: fmt.Sprintf("read error: %v", rerr)})
			return nil
		}
		errs = append(errs, validateBytes(data, path)...)
		return nil
	})
	if err != nil {
		errs = append(errs, ValidationError{File: dir, Message: fmt.Sprintf("walking directory: %v", err)})
	}
	return errs
}

func validateBytes(data []byte, source string) []ValidationError {
	var errs []ValidationError
	if !strings.Contains(string(data), "kind: NetworkAttachmentDefinition") {
		return nil
	}
	var found bool
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		key := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
		if key == "config" {
			found = true
			value := ""
			if idx := strings.Index(line, ":"); idx != -1 {
				value = strings.TrimSpace(line[idx+1:])
			}
			if value == "" || value == "''" || value == `""` {
				errs = append(errs, ValidationError{File: source, Message: fmt.Sprintf("line %d: spec.config is empty", lineNo)})
			}
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		errs = append(errs, ValidationError{File: source, Message: fmt.Sprintf("scan error: %v", scanErr)})
	}
	if !found {
		errs = append(errs, ValidationError{File: source, Message: "spec.config field not found in NetworkAttachmentDefinition"})
	}
	return errs
}
