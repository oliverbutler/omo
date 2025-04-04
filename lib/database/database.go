package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"oliverbutler/lib/environment"
	"oliverbutler/lib/logging"
	"oliverbutler/lib/tracing"
	"os"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type DatabaseService struct {
	Pool *pgxpool.Pool
}

func NewDatabaseService(ctx context.Context, env *environment.EnvironmentService) (*DatabaseService, error) {
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", env.DbUser, env.DbPassword, env.DbHost, env.DbPort, env.DbName)

	cfg, err := pgxpool.ParseConfig(dbUrl)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithIncludeQueryParameters(), otelpgx.WithAttributes(
		semconv.DBSystemPostgreSQL,
		semconv.ServiceName("pg"),
	), otelpgx.WithTracerProvider(tracing.DBTraceProvider))

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		logging.OmoLogger.Error("Failed to convert pgxpool to *sql.DB", "error", err)
		return nil, err
	}

	logging.OmoLogger.Info(fmt.Sprintf("Connected to database: %s at %s:%s", env.DbName, env.DbHost, env.DbPort))

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
