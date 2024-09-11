package users

import (
	"encoding/json"
	"fmt"
	"oliverbutler/lib/utils"
	"os"
	"time"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

const (
	RedirectPath = "/api/auth/github/callback"
)

// GetOAuthAuthorizationUrl returns the URL to let the user authorize the app to access their GitHub account.
// If redirectToApp is true, the user will be redirected to the app, otherwise they will be redirected to the API.
func GetOAuthAuthorizationUrl(redirectToApp bool) string {
	redirectUri := utils.GetBaseUrl() + RedirectPath

	if redirectToApp {
		redirectUri += "/app"
	} else {
		redirectUri += "/api"
	}

	return "https://github.com/login/oauth/authorize?client_id=" + os.Getenv("GITHUB_CLIENT_ID") + "&redirect_uri=" + redirectUri + "&scope=user:email"
}

func ExchangeOAuthCodeForAccessToken(code string) (*TokenResponse, error) {
	data := map[string]string{
		"client_id":     os.Getenv("GITHUB_CLIENT_ID"),
		"client_secret": os.Getenv("GITHUB_CLIENT_SECRET"),
		"code":          code,
		"redirect_uri":  utils.GetBaseUrl() + RedirectPath,
	}
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}

	resp, err := utils.JSONRequest(utils.POST, "https://github.com/login/oauth/access_token", data, headers)
	if err != nil {
		return nil, err
	}

	tokenResponse := &TokenResponse{}
	err = json.Unmarshal(resp, tokenResponse)
	if err != nil {
		return nil, err
	}

	if tokenResponse.Error != "" {
		return nil, fmt.Errorf("error from GitHub: %s", tokenResponse.Error)
	}

	return tokenResponse, nil
}

func GetGitHubUser(accessToken string) (GitHubUser, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
	}

	resp, err := utils.JSONRequest(utils.GET, "https://api.github.com/user", nil, headers)
	if err != nil {
		return GitHubUser{}, err
	}

	var user GitHubUser
	err = json.Unmarshal(resp, &user)
	if err != nil {
		return GitHubUser{}, err
	}

	return user, nil
}

type GitHubUser struct {
	Login                   string    `json:"login"`
	Id                      int       `json:"id"`
	NodeId                  string    `json:"node_id"`
	AvatarUrl               string    `json:"avatar_url"`
	GravatarId              string    `json:"gravatar_id"`
	Url                     string    `json:"url"`
	HtmlUrl                 string    `json:"html_url"`
	FollowersUrl            string    `json:"followers_url"`
	FollowingUrl            string    `json:"following_url"`
	GistsUrl                string    `json:"gists_url"`
	StarredUrl              string    `json:"starred_url"`
	SubscriptionsUrl        string    `json:"subscriptions_url"`
	OrganizationsUrl        string    `json:"organizations_url"`
	ReposUrl                string    `json:"repos_url"`
	EventsUrl               string    `json:"events_url"`
	ReceivedEventsUrl       string    `json:"received_events_url"`
	Type                    string    `json:"type"`
	SiteAdmin               bool      `json:"site_admin"`
	Name                    string    `json:"name"`
	Company                 string    `json:"company"`
	Blog                    string    `json:"blog"`
	Location                string    `json:"location"`
	Email                   string    `json:"email"`
	Hireable                bool      `json:"hireable"`
	Bio                     string    `json:"bio"`
	TwitterUsername         string    `json:"twitter_username"`
	PublicRepos             int       `json:"public_repos"`
	PublicGists             int       `json:"public_gists"`
	Followers               int       `json:"followers"`
	Following               int       `json:"following"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	PrivateGists            int       `json:"private_gists"`
	TotalPrivateRepos       int       `json:"total_private_repos"`
	OwnedPrivateRepos       int       `json:"owned_private_repos"`
	DiskUsage               int       `json:"disk_usage"`
	Collaborators           int       `json:"collaborators"`
	TwoFactorAuthentication bool      `json:"two_factor_authentication"`
	Plan                    struct {
		Name          string `json:"name"`
		Space         int    `json:"space"`
		PrivateRepos  int    `json:"private_repos"`
		Collaborators int    `json:"collaborators"`
	} `json:"plan"`
}
