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
func DeleteComments(c *Client, markers ...string) error {
	if !c.IsAvailable() {
		return nil
	}
	for _, marker := range markers {
		ids, err := listCommentIDs(c, marker)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := c.glab(
				"api",
				fmt.Sprintf("projects/%s/merge_requests/%s/notes/%s", EncodeProject(c.repo), c.mr, id),
				"-X", "DELETE",
			); err != nil {
				return err
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
	if err := json.Unmarshal([]byte(out), &notes); err != nil {
		if apiErr := checkAPIError([]byte(out)); apiErr != nil {
			return nil, apiErr
		}
		return nil, fmt.Errorf("parsing MR notes: %w", err)
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

func listCommentIDs(c *Client, marker string) ([]string, error) {
	notes, err := fetchNotes(c)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, n := range notes {
		if !n.System && strings.Contains(n.Body, marker) {
			ids = append(ids, fmt.Sprintf("%d", n.ID))
		}
	}
	return ids, nil
}

func postComment(c *Client, body string) error {
	_, err := c.glabStdin(
		body, "api",
		fmt.Sprintf("projects/%s/merge_requests/%s/notes", EncodeProject(c.repo), c.mr),
		"-X", "POST", "-F", "body=@-",
	)
	return err
}
