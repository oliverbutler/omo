package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"oliverbutler/lib/database"
	"oliverbutler/lib/environment"
	"oliverbutler/lib/tracing"
	"oliverbutler/lib/utils"
	"strings"
	"time"

	"github.com/lucsky/cuid"
)

type UserService struct {
	repo   UserRepository
	github *GitHubService
	env    *environment.EnvironmentService
}

func NewUserService(database *database.DatabaseService, env *environment.EnvironmentService) *UserService {
	repo := PgNewUserRepository(database.Pool)
	github := NewGitHubService(env)

	return &UserService{repo: repo, github: github, env: env}
}

type UserContext struct {
	User       *User
	IsLoggedIn bool
}

type User struct {
	ID                string    `json:"id"`
	GivenName         string    `json:"givenName"`
	FamilyName        string    `json:"familyName"`
	Email             string    `json:"email"`
	ProfilePictureUrl *string   `json:"profilePictureUrl"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type GithubConnection struct {
	GithubId          int       `json:"githubId"`
	UserId            string    `json:"userId"`
	GithubAccessToken string    `json:"githubAccessToken"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type UserSession struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userID"`
	RefreshTokenHash string     `json:"refreshTokenHash"`
	FamilyID         string     `json:"familyID"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	InvalidatedAt    *time.Time `json:"invalidatedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type UserSessionResponse struct {
	AccessToken   string `json:"accessToken"`
	RefreshToken  string `json:"refreshToken"`
	UserSessionId string `json:"userSessionId"`
}

const (
	AccessTokenExpiresIn  = time.Minute * 15
	RefreshTokenExpiresIn = time.Hour * 24 * 7
)

type RefreshTokenRequest struct {
	RefreshToken  string `json:"refreshToken"`
	UserSessionId string `json:"userSessionId"`
}

func (s *UserService) GetById(id string) (*User, error) {
	return s.repo.GetById(id)
}

func (s *UserService) GetByEmail(email string) (*User, error) {
	return s.repo.GetByEmail(email)
}

func (s *UserService) HandleGithubAuthCallback(ctx context.Context, code string) (*UserSessionResponse, error) {
	ctx, span := tracing.Tracer.Start(ctx, "UserService.HandleGithubAuthCallback")
	defer span.End()

	tokenResponse, err := s.github.ExchangeOAuthCodeForAccessToken(ctx, code)
	if err != nil {
		slog.Error("ExchangeOAuthCodeForAccessToken failed", "error", err)
		return nil, err
	}

	gitHubUser, err := s.github.GetGitHubUser(ctx, tokenResponse.AccessToken)
	if err != nil {
		slog.Error("GetGitHubUser failed", "error", err)
		return nil, err
	}

	user, err := s.UpsertUserFromGitHub(ctx, gitHubUser, tokenResponse.AccessToken)
	if err != nil {
		slog.Error("UpsertUserFromGitHub failed", "error", err)
		return nil, err
	}

	userSession, err := s.CreateUserSession(ctx, user)
	if err != nil {
		slog.Error("CreateUserSession failed", "error", err)
		return nil, err
	}

	return userSession, nil
}

func (s *UserService) UpsertUserFromGitHub(ctx context.Context, gitHubUser GitHubUser, gitHubAccessToken string) (*User, error) {
	ctx, span := tracing.Tracer.Start(ctx, "UserService.UpsertUserFromGitHub")
	defer span.End()

	user, err := s.GetByEmail(gitHubUser.Email)
	if err != nil {
		if errors.Is(err, utils.RowNotFound) {
			nameParts := strings.Split(gitHubUser.Name, " ")
			givenName := nameParts[0]
			familyName := nameParts[1]

			slog.Info("creating new user from github user", gitHubUser)

			return s.repo.CreateGitHubUser(User{
				ID:                cuid.New(),
				GivenName:         givenName,
				FamilyName:        familyName,
				Email:             gitHubUser.Email,
				ProfilePictureUrl: gitHubUser.AvatarUrl,
			}, gitHubUser.Id, gitHubAccessToken)
		}

		slog.Error("failed to get user by email", err)

		return nil, err
	}

	return user, nil
}

func (s *UserService) RefreshUserSession(refresh RefreshTokenRequest) (*UserSessionResponse, error) {
	session, err := s.repo.GetUserSessionById(refresh.UserSessionId)
	if err != nil {
		return nil, err
	}

	if session.InvalidatedAt != nil {
		return nil, errors.New("invalid user session")
	}

	if session.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("invalid user session")
	}

	user, err := s.GetById(session.UserID)

	latestSession, err := s.repo.GetLatestUserSessionByFamilyId(session.FamilyID)
	if err != nil {
		return nil, err
	}

	if latestSession.ID != session.ID {
		slog.Warn("detected potential session hijacking", session.ID, latestSession.ID)

		err := s.repo.InvalidateUserSessionsByFamilyId(session.FamilyID)
		if err != nil {
			slog.Error("failed to invalidate user sessions by family id", err)
			return nil, err
		}

		return nil, errors.New("invalid user session")
	}

	if err != nil {
		return nil, err
	}

	err = CompareHashAndPassword(session.RefreshTokenHash, refresh.RefreshToken)
	if err != nil {
		return nil, errors.New("invalid user session")
	}

	tokens, err := CreateAccessAndRefreshToken(user)
	if err != nil {
		return nil, err
	}

	err = s.repo.InvalidateUserSessionById(session.ID)
	if err != nil {
		return nil, err
	}

	newSession, err := s.repo.CreateSession(UserSession{
		ID:               cuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokens.RefreshTokenHash,
		FamilyID:         session.FamilyID,
		ExpiresAt:        time.Now().Add(time.Hour * 24 * 7),
	})
	if err != nil {
		return nil, err
	}

	return &UserSessionResponse{
		AccessToken:   tokens.AccessToken,
		RefreshToken:  tokens.RefreshToken,
		UserSessionId: newSession.ID,
	}, nil
}

func (s *UserService) CreateUserSession(ctx context.Context, user *User) (*UserSessionResponse, error) {
	ctx, span := tracing.Tracer.Start(ctx, "UserService.CreateUserSession")
	defer span.End()

	tokens, err := CreateAccessAndRefreshToken(user)

	familyId := cuid.New()

	newSession, err := s.repo.CreateSession(UserSession{
		ID:               cuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: tokens.RefreshTokenHash,
		FamilyID:         familyId,
		ExpiresAt:        time.Now().Add(time.Hour * 24 * 7),
	})
	if err != nil {
		return nil, err
	}

	return &UserSessionResponse{
		AccessToken:   tokens.AccessToken,
		RefreshToken:  tokens.RefreshToken,
		UserSessionId: newSession.ID,
	}, nil
}

func (s *UserService) ExtractUserClaimsFromCookies(w http.ResponseWriter, r *http.Request) (AccessTokenClaims, error) {
	accessTokenCookie, err := r.Cookie("AccessToken")
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("failed to extract AccessToken cookie: %v", err)
	}
	accessToken := accessTokenCookie.Value

	refreshTokenCookie, err := r.Cookie("RefreshToken")
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("failed to extract RefreshToken cookie: %v", err)
	}
	refreshToken := refreshTokenCookie.Value

	userSessionIdCookie, err := r.Cookie("UserSessionId")
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("failed to extract UserSessionId cookie: %v", err)
	}
	userSessionId := userSessionIdCookie.Value

	claims, err := ParseAccessToken(accessToken)
	if err != nil {
		refreshResult, err := s.RefreshUserSession(RefreshTokenRequest{
			RefreshToken:  refreshToken,
			UserSessionId: userSessionId,
		})
		if err != nil {
			return AccessTokenClaims{}, fmt.Errorf("failed to refresh token: %v", err)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "AccessToken",
			Value:    refreshResult.AccessToken,
			MaxAge:   int(AccessTokenExpiresIn.Seconds()),
			Path:     "/",
			Domain:   utils.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "RefreshToken",
			Value:    refreshResult.RefreshToken,
			MaxAge:   int(RefreshTokenExpiresIn.Seconds()),
			Path:     "/",
			Domain:   utils.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "UserSessionId",
			Value:    refreshResult.UserSessionId,
			MaxAge:   int(RefreshTokenExpiresIn.Seconds()),
			Path:     "/",
			Domain:   utils.GetDomain(),
			Secure:   true,
			HttpOnly: true,
		})

		claims, err = ParseAccessToken(refreshResult.AccessToken)
		if err != nil {
			return AccessTokenClaims{}, fmt.Errorf("failed to parse refreshed access token: %v", err)
		}
	}

	return claims, nil
}

func (s *UserService) ExtractUserFromCookies(ctx context.Context, w http.ResponseWriter, r *http.Request) (*UserContext, error) {
	_, span := tracing.Tracer.Start(ctx, "UserService.ExtractUserFromCookies")
	defer span.End()

	claims, err := s.ExtractUserClaimsFromCookies(w, r)
	if err != nil {
		return &UserContext{IsLoggedIn: false, User: nil}, fmt.Errorf("failed to extract user claims: %v", err)
	}

	user, err := s.repo.GetById(claims.UserId)
	if err != nil {
		return &UserContext{IsLoggedIn: false, User: nil}, fmt.Errorf("failed to get user by id: %v", err)
	}

	return &UserContext{IsLoggedIn: true, User: user}, nil
}

func (s *UserService) GetOAuthAuthorizationUrl() string {
	return s.github.GetOAuthAuthorizationUrl()
}
