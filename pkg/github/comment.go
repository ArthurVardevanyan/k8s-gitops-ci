package github

import (
	"fmt"
	"strings"
)

// UpsertComment posts or updates the single CI report comment. Updating an
// existing comment PATCHes it directly by ID (found via findCommentID)
// rather than relying on gh's --edit-last (which edits whatever the
// authenticated user's most recent comment happens to be, not necessarily
// the one matching marker). The body is passed via stdin (-F body=@-), not
// as a command-line argument, to avoid both argv length limits for large
// reports and shell-argument-escaping concerns.
func UpsertComment(c *Client, marker, body string) error {
	if !c.IsAvailable() {
		return nil
	}
	id, err := findCommentID(c, marker)
	if err != nil {
		return err
	}
	if id != "" {
		_, err := c.ghStdin(body, "api", fmt.Sprintf("repos/%s/issues/comments/%s", c.repo, id), "-X", "PATCH", "-F", "body=@-")
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
	// Route through c.gh (not a bare exec.Command) so GH_REPO gets set the
	// same way findCommentID's lookup does.
	out, err := c.gh("api", fmt.Sprintf("repos/%s/issues/%s/comments", c.repo, c.pr), "--jq", fmt.Sprintf(".[] | select(.body | contains(%q)) | .id", marker))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids, nil
}

func postComment(c *Client, body string) error {
	_, err := c.ghStdin(body, "pr", "comment", c.pr, "--body-file", "-")
	return err
}
