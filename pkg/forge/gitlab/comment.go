package gitlab

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UpsertComment posts or updates the single CI report comment on a GitLab MR.
// Updating an existing comment updates the note directly by ID via PUT API
// with body streamed via stdin (-F body=@-).
func UpsertComment(c *Client, marker, body string) error {
	if !c.IsAvailable() {
		return nil
	}
	id, err := findCommentID(c, marker)
	if err != nil {
		return err
	}
	if id != "" {
		_, err := c.glabStdin(
			body, "api",
			fmt.Sprintf("projects/%s/merge_requests/%s/notes/%s", EncodeProject(c.repo), c.mr, id),
			"-X", "PUT", "-F", "body=@-",
		)
		return err
	}
	return postComment(c, body)
}

// DeleteComments deletes MR notes whose bodies contain any of the markers.
// It fetches notes in a single pass across all markers, deduping note IDs
// and ignoring 404 errors if a note was already deleted.
func DeleteComments(c *Client, markers ...string) error {
	if !c.IsAvailable() || len(markers) == 0 {
		return nil
	}
	notes, err := fetchNotes(c)
	if err != nil {
		return err
	}
	seen := make(map[int]bool)
	for _, n := range notes {
		if n.System || seen[n.ID] {
			continue
		}
		for _, marker := range markers {
			if strings.Contains(n.Body, marker) {
				seen[n.ID] = true
				if _, err := c.glab(
					"api",
					fmt.Sprintf("projects/%s/merge_requests/%s/notes/%d", EncodeProject(c.repo), c.mr, n.ID),
					"-X", "DELETE",
				); err != nil {
					errStr := err.Error()
					if !strings.Contains(errStr, "404") && !strings.Contains(errStr, "Not Found") {
						return err
					}
				}
				break
			}
		}
	}
	return nil
}

type mrNote struct {
	ID     int    `json:"id"`
	Body   string `json:"body"`
	System bool   `json:"system"`
}

func fetchNotes(c *Client) ([]mrNote, error) {
	out, err := c.glab(
		"api", "--paginate",
		fmt.Sprintf("projects/%s/merge_requests/%s/notes", EncodeProject(c.repo), c.mr),
	)
	if err != nil {
		return nil, err
	}
	var notes []mrNote
	dec := json.NewDecoder(strings.NewReader(out))
	for dec.More() {
		var page []mrNote
		if err := dec.Decode(&page); err != nil {
			if apiErr := checkAPIError([]byte(out)); apiErr != nil {
				return nil, apiErr
			}
			return nil, fmt.Errorf("parsing MR notes: %w", err)
		}
		notes = append(notes, page...)
	}
	return notes, nil
}

func findCommentID(c *Client, marker string) (string, error) {
	notes, err := fetchNotes(c)
	if err != nil {
		return "", err
	}
	for _, n := range notes {
		if !n.System && strings.Contains(n.Body, marker) {
			return fmt.Sprintf("%d", n.ID), nil
		}
	}
	return "", nil
}

func postComment(c *Client, body string) error {
	_, err := c.glabStdin(
		body, "api",
		fmt.Sprintf("projects/%s/merge_requests/%s/notes", EncodeProject(c.repo), c.mr),
		"-X", "POST", "-F", "body=@-",
	)
	return err
}
