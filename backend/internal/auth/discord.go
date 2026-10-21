package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DiscordTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email,omitempty"`
}

func GetDiscordAuthURL(clientID, redirectURI, state string) string {
	baseURL := "https://discord.com/api/oauth2/authorize"
	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("redirect_uri", redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", "identify email")
	if state != "" {
		params.Add("state", state)
	}
	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

func ExchangeDiscordCode(clientID, clientSecret, redirectURI, code string) (*DiscordTokenResponse, error) {
	tokenURL := "https://discord.com/api/oauth2/token"

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code with Discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp DiscordTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode discord token response: %w", err)
	}

	return &tokenResp, nil
}

func FetchDiscordUser(accessToken string) (*DiscordUser, error) {
	userURL := "https://discord.com/api/users/@me"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user from Discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord user endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var discordUser DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&discordUser); err != nil {
		return nil, fmt.Errorf("failed to decode discord user response: %w", err)
	}

	return &discordUser, nil
}
