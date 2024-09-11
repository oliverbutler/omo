package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"oliverbutler/lib/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	GetById(id string) (*User, error)
	GetByEmail(email string) (*User, error)
	CreateGitHubUser(user User, gitHubUserId int, gitHubAccessToken string) (*User, error)
	GetUserSessionById(id string) (*UserSession, error)
	GetLatestUserSessionByFamilyId(familyId string) (*UserSession, error)
	InvalidateUserSessionsByFamilyId(familyId string) error
	InvalidateUserSessionById(id string) error
	CreateSession(session UserSession) (*UserSession, error)
}

type PgUserRepository struct {
	db *pgxpool.Pool
}

func PgNewUserRepository(db *pgxpool.Pool) *PgUserRepository {
	return &PgUserRepository{db: db}
}

func (r *PgUserRepository) GetById(id string) (*User, error) {
	row := r.db.QueryRow(
		context.Background(),
		"SELECT * FROM users WHERE id = $1",
		id,
	)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.RowNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (r *PgUserRepository) GetByEmail(email string) (*User, error) {
	row := r.db.QueryRow(
		context.Background(),
		"SELECT * FROM users WHERE email = $1",
		email,
	)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.RowNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (r *PgUserRepository) CreateGitHubUser(user User, gitHubUserId int, gitHubAccessToken string) (*User, error) {
	tx, err := r.db.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		slog.Error("failed to begin transaction", err)
		return nil, fmt.Errorf("failed to begin transaction")
	}

	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			slog.Error("failed to rollback transaction", err)
		}
	}(tx, context.Background())

	row := tx.QueryRow(
		context.Background(),
		"INSERT INTO users (id, given_name, family_name, email, profile_picture_url) VALUES ($1, $2, $3, $4, $5) RETURNING *",
		user.ID,
		user.GivenName,
		user.FamilyName,
		user.Email,
		user.ProfilePictureUrl,
	)

	newUser, err := scanUser(row)
	if err != nil {
		slog.Error("failed to scan user", err)
		return nil, fmt.Errorf("failed to scan user")
	}

	_, err = tx.Exec(context.Background(),
		"INSERT INTO github_connections (github_id, user_id, github_access_token) VALUES ($1, $2, $3) RETURNING *",
		gitHubUserId,
		user.ID,
		gitHubAccessToken,
	)
	if err != nil {
		slog.Error("failed to insert github connection", err)
		return nil, fmt.Errorf("failed to insert github connection")
	}

	err = tx.Commit(context.Background())

	slog.Info("Added new user from github", newUser)

	if err != nil {
		slog.Error("failed to commit transaction", err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	return &newUser, nil
}

func (r *PgUserRepository) GetUserSessionById(id string) (*UserSession, error) {
	row := r.db.QueryRow(
		context.Background(),
		"SELECT * FROM user_sessions WHERE id = $1",
		id,
	)
	session, err := scanUserSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.RowNotFound
		}
		return nil, err
	}

	return &session, nil
}

func (r *PgUserRepository) GetLatestUserSessionByFamilyId(familyId string) (*UserSession, error) {
	row := r.db.QueryRow(
		context.Background(),
		"SELECT * FROM user_sessions WHERE family_id = $1 ORDER BY created_at DESC LIMIT 1",
		familyId,
	)
	session, err := scanUserSession(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.RowNotFound
		}
		return nil, err
	}

	return &session, nil
}

func (r *PgUserRepository) InvalidateUserSessionsByFamilyId(familyId string) error {
	_, err := r.db.Exec(
		context.Background(),
		"UPDATE user_sessions SET invalidated_at = $1 WHERE family_id = $2",
		time.Now(),
		familyId,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *PgUserRepository) InvalidateUserSessionById(id string) error {
	_, err := r.db.Exec(
		context.Background(),
		"UPDATE user_sessions SET invalidated_at = $1 WHERE id = $2",
		time.Now(),
		id,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *PgUserRepository) CreateSession(session UserSession) (*UserSession, error) {
	row := r.db.QueryRow(
		context.Background(),
		"INSERT INTO user_sessions (id, user_id, refresh_token_hash, expires_at, family_id) VALUES ($1, $2, $3, $4, $5) RETURNING *",
		session.ID,
		session.UserID,
		session.RefreshTokenHash,
		session.ExpiresAt,
		session.FamilyID,
	)

	newSession, err := scanUserSession(row)
	if err != nil {
		return nil, err
	}

	return &newSession, nil
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.GivenName, &user.FamilyName, &user.Email, &user.ProfilePictureUrl, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	if user == (User{}) {
		return User{}, fmt.Errorf("no user found")
	}
	return user, nil
}

func scanUserSession(row pgx.Row) (UserSession, error) {
	var session UserSession
	err := row.Scan(&session.ID, &session.UserID, &session.RefreshTokenHash, &session.ExpiresAt, &session.InvalidatedAt, &session.FamilyID, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return UserSession{}, err
	}
	if session == (UserSession{}) {
		return UserSession{}, fmt.Errorf("failed to parse user session")
	}
	return session, nil
}
