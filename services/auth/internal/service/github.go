package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type GitHubService struct {
	ClientID     string
	ClientSecret string
}

func NewGitHubService(clientID, clientSecret string) *GitHubService {
	return &GitHubService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

func (s *GitHubService) ExchangeCodeForToken(code string) (string, error) {
	url := "https://github.com/login/oauth/access_token"
	payload := map[string]string{
		"client_id":     s.ClientID,
		"client_secret": s.ClientSecret,
		"code":          code,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenRes TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		return "", err
	}

	if tokenRes.Error != "" {
		return "", fmt.Errorf("github error: %s", tokenRes.Error)
	}

	return tokenRes.AccessToken, nil
}

type GitHubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func (s *GitHubService) FetchUserProfile(token string) (*GitHubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// Fetch primary email if not public
	if user.Email == "" {
		emails, _ := s.FetchUserEmails(token)
		if len(emails) > 0 {
			user.Email = emails[0]
		}
	}

	return &user, nil
}

func (s *GitHubService) FetchUserEmails(token string) ([]string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email   string `json:"email"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return nil, err
	}

	var emailList []string
	for _, e := range emails {
		if e.Primary {
			emailList = append([]string{e.Email}, emailList...)
		} else {
			emailList = append(emailList, e.Email)
		}
	}
	return emailList, nil
}

type GitHubRepo struct {
	FullName string `json:"full_name"`
}

func (s *GitHubService) FetchUserRepos(token string) ([]string, error) {
	url := "https://api.github.com/user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=updated"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch repos, status: %d", resp.StatusCode)
	}

	var repos []GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	var repoNames []string
	for _, r := range repos {
		repoNames = append(repoNames, r.FullName)
	}
	return repoNames, nil
}
