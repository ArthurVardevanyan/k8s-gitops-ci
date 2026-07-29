package github

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// UpsertComment posts or updates the single CI report comment.
func UpsertComment(c *Client, marker, body string) error {
	if !c.IsAvailable() {
		return nil
	}
	id, err := findCommentID(c, marker)
	if err != nil {
		return err
	}
	if id != "" {
		_, err := c.gh("pr", "comment", c.pr, "--edit-last", "--body-file", "-")
		if err == nil {
			return nil
		}
		_, err = c.gh("api", fmt.Sprintf("repos/%s/issues/comments/%s", c.repo, id), "-X", "PATCH", "-F", "body="+body)
		return err
	}
	return postComment(c, body)
}

// DeleteComments deletes comments whose bodies contain any of the markers.
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
			if _, err := c.gh("api", fmt.Sprintf("repos/%s/issues/comments/%s", c.repo, id), "-X", "DELETE"); err != nil {
				return err
			}
		}
	}
	return nil
}

func findCommentID(c *Client, marker string) (string, error) {
	out, err := c.gh("api", fmt.Sprintf("repos/%s/issues/%s/comments", c.repo, c.pr), "--jq", fmt.Sprintf(".[] | select(.body | contains(%q)) | .id", marker))
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}
	return strings.Split(out, "\n")[0], nil
}

func listCommentIDs(c *Client, marker string) ([]string, error) {
	out, err := exec.CommandContext(context.Background(), "gh", "api", fmt.Sprintf("repos/%s/issues/%s/comments", c.repo, c.pr), "--jq", fmt.Sprintf(".[] | select(.body | contains(%q)) | .id", marker)).Output()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids, nil
}

func postComment(c *Client, body string) error {
	_, err := c.gh("pr", "comment", c.pr, "--body", body)
	return err
}
