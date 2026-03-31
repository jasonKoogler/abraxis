package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// UserInfo represents standardized user information across different providers
type UserInfo struct {
	RawData           map[string]interface{} `json:"-"` // raw data from the provider
	Provider          string                 `json:"provider"`
	Email             string                 `json:"email"`
	Name              string                 `json:"name"`
	FirstName         string                 `json:"first_name"`
	LastName          string                 `json:"last_name"`
	NickName          string                 `json:"nick_name"`
	Description       string                 `json:"description"`
	UserID            string                 `json:"user_id"`
	AvatarURL         string                 `json:"avatar_url"`
	Location          string                 `json:"location"`
	AccessToken       string                 `json:"access_token"`
	AccessTokenSecret string                 `json:"access_token_secret"`
	RefreshToken      string                 `json:"refresh_token"`
	ExpiresAt         time.Time              `json:"expires_at"`
	IDToken           string                 `json:"id_token"`
}

// Provider-specific user info endpoints
const (
	googleUserInfoURL   = "https://www.googleapis.com/oauth2/v3/userinfo"
	twitterUserInfoURL  = "https://api.twitter.com/1.1/account/verify_credentials.json"
	facebookUserInfoURL = "https://graph.facebook.com/v18.0/me"
)

// GetUserInfo fetches user information from the specified provider
func (m *OAuthManager) GetUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*UserInfo, error) {
	if !token.Valid() {
		return nil, fmt.Errorf("invalid token")
	}

	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))

	switch provider {
	case "google":
		return m.getGoogleUserInfo(client, token.AccessToken)
	case "twitter":
		return m.getTwitterUserInfo(client, token.AccessToken)
	case "facebook":
		return m.getFacebookUserInfo(client, token.AccessToken)
	// Add more providers here
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (m *OAuthManager) getGoogleUserInfo(client *http.Client, accessToken string) (*UserInfo, error) {
	resp, err := client.Get(googleUserInfoURL + "?access_token=" + url.QueryEscape(accessToken))
	if err != nil {
		return nil, fmt.Errorf("failed to get Google user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google API error: %s, status: %d", string(body), resp.StatusCode)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Google user info: %w", err)
	}

	// Google specific user info to be decoded
	var googUser struct {
		Email      string `json:"email"`
		FamilyName string `json:"family_name"`
		GivenName  string `json:"given_name"`
		ID         string `json:"id"`
		Locale     string `json:"locale"`
		Name       string `json:"name"`
		Picture    string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googUser); err != nil {
		return nil, fmt.Errorf("failed to decode Google user info: %w", err)
	}

	user := &UserInfo{
		Email:     googUser.Email,
		FirstName: googUser.GivenName,
		LastName:  googUser.FamilyName,
		AvatarURL: googUser.Picture,
		Provider:  "google",
		UserID:    googUser.ID,
	}

	if err := json.Unmarshal(respBytes, &user.RawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Google user info: %w", err)
	}

	return user, nil
}

func (m *OAuthManager) getTwitterUserInfo(client *http.Client, accessToken string) (*UserInfo, error) {
	resp, err := client.Get(twitterUserInfoURL + "?access_token=" + url.QueryEscape(accessToken))
	if err != nil {
		return nil, fmt.Errorf("failed to get Twitter user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Twitter API error: %s, status: %d", string(body), resp.StatusCode)
	}

	// todo: finish this
	return nil, nil
}

func (m *OAuthManager) getFacebookUserInfo(client *http.Client, accessToken string) (*UserInfo, error) {
	resp, err := client.Get(facebookUserInfoURL + "?access_token=" + url.QueryEscape(accessToken))
	if err != nil {
		return nil, fmt.Errorf("failed to get Facebook user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Facebook API error: %s, status: %d", string(body), resp.StatusCode)
	}

	// todo: finish this
	return nil, nil
}

// Helper function to make HTTP requests with error handling
func makeRequest(client *http.Client, url string, target interface{}) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s, status: %d", string(body), resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}
