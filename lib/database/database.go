package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"oliverbutler/lib/environment"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

type DatabaseService struct {
	Pool *pgxpool.Pool
}

func NewDatabaseService(env *environment.EnvironmentService) (*DatabaseService, error) {
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", env.GetDbUser(), env.GetDbPassword(), env.GetDbHost(), env.GetDbPort(), env.GetDbName())

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		slog.Error("Failed to convert pgxpool to *sql.DB", "error", err)
		return nil, err
	}

	slog.Info(fmt.Sprintf("Connected to database: %s at %s:%s", env.GetDbName(), env.GetDbHost(), env.GetDbPort()))

	defer db.Close()

	gooseProvider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("./migrations"))

	res, err := gooseProvider.Up(context.Background())
	if err != nil {
		slog.Error("Failed to run migrations", "error", err)
		panic(err)
	}

	if res != nil {
		slog.Info("Migrations ran successfully")

		for _, r := range res {
			slog.Info(fmt.Sprintf("Migration: %s in %s", r.String(), r.Duration.String()))
		}
	}

	return &DatabaseService{Pool: pool}, nil
}

func (d *DatabaseService) TearDown() {
	d.Pool.Close()
}
