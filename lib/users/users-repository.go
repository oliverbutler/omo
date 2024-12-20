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
	GetById(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	CreateGitHubUser(ctx context.Context, user User, gitHubUserId int, gitHubAccessToken string) (*User, error)
	GetUserSessionById(ctx context.Context, id string) (*UserSession, error)
	GetLatestUserSessionByFamilyId(ctx context.Context, familyId string) (*UserSession, error)
	InvalidateUserSessionsByFamilyId(ctx context.Context, familyId string) error
	InvalidateUserSessionById(ctx context.Context, id string) error
	CreateSession(ctx context.Context, csession UserSession) (*UserSession, error)
	IncrementVisitorCount(ctx context.Context) (int, error)
}

type PgUserRepository struct {
	db *pgxpool.Pool
}

func PgNewUserRepository(db *pgxpool.Pool) *PgUserRepository {
	return &PgUserRepository{db: db}
}

func (r *PgUserRepository) GetById(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRow(
		ctx,
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

func (r *PgUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.QueryRow(
		ctx,
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

func (r *PgUserRepository) CreateGitHubUser(ctx context.Context, user User, gitHubUserId int, gitHubAccessToken string) (*User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		slog.Error("failed to begin transaction", err)
		return nil, fmt.Errorf("failed to begin transaction")
	}

	defer func(tx pgx.Tx, ctx context.Context) {
		err := tx.Rollback(ctx)
		if err != nil {
			slog.Error("failed to rollback transaction", err)
		}
	}(tx, ctx)

	row := tx.QueryRow(
		ctx,
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

	_, err = tx.Exec(ctx,
		"INSERT INTO github_connections (github_id, user_id, github_access_token) VALUES ($1, $2, $3) RETURNING *",
		gitHubUserId,
		user.ID,
		gitHubAccessToken,
	)
	if err != nil {
		slog.Error("failed to insert github connection", err)
		return nil, fmt.Errorf("failed to insert github connection")
	}

	err = tx.Commit(ctx)

	slog.Info("Added new user from github", newUser)

	if err != nil {
		slog.Error("failed to commit transaction", err)
		return nil, fmt.Errorf("failed to commit transaction")
	}

	return &newUser, nil
}

func (r *PgUserRepository) GetUserSessionById(ctx context.Context, id string) (*UserSession, error) {
	row := r.db.QueryRow(
		ctx,
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

func (r *PgUserRepository) GetLatestUserSessionByFamilyId(ctx context.Context, familyId string) (*UserSession, error) {
	row := r.db.QueryRow(
		ctx,
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

func (r *PgUserRepository) InvalidateUserSessionsByFamilyId(ctx context.Context, familyId string) error {
	_, err := r.db.Exec(
		ctx,
		"UPDATE user_sessions SET invalidated_at = $1 WHERE family_id = $2",
		time.Now(),
		familyId,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *PgUserRepository) InvalidateUserSessionById(ctx context.Context, id string) error {
	_, err := r.db.Exec(
		ctx,
		"UPDATE user_sessions SET invalidated_at = $1 WHERE id = $2",
		time.Now(),
		id,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *PgUserRepository) CreateSession(ctx context.Context, session UserSession) (*UserSession, error) {
	row := r.db.QueryRow(
		ctx,
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

func (r *PgUserRepository) IncrementVisitorCount(ctx context.Context) (int, error) {
	var count int
	row := r.db.QueryRow(
		ctx,
		"UPDATE visits SET visits = visits + 1 WHERE page = 'home' RETURNING visits",
	)
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
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
