package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func PostPRComment(ctx context.Context, repoURL string, prNumber int, markdown, token string) error {
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid repo URL")
	}
	repo := parts[len(parts)-1]
	owner := parts[len(parts)-2]
	repo = strings.TrimSuffix(repo, ".git")

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, prNumber)

	payload := map[string]string{"body": markdown}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("github api error: status %d", resp.StatusCode)
	}

	return nil
}
