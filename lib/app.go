package lib

import (
	"context"
	"log/slog"
	"oliverbutler/lib/blog"
	"oliverbutler/lib/database"
	"oliverbutler/lib/environment"
	"oliverbutler/lib/logging"
	"oliverbutler/lib/mapping"
	"oliverbutler/lib/photos"
	"oliverbutler/lib/storage"
	"oliverbutler/lib/tracing"
	"oliverbutler/lib/users"
	"os"
)

type App struct {
	Environment *environment.EnvironmentService
	Database    *database.DatabaseService
	Storage     *storage.StorageService
	Users       *users.UserService
	Blog        *blog.BlogService
	Photos      *photos.PhotoService
	Mapping     *mapping.MappingService
}

// Single place services are instantiated, and environment variables are read and passed to the services.
// Gives a birds eye view of module dependencies, both internal and external.
func NewApp(ctx context.Context) (*App, error) {
	logging.OmoLogger = logging.NewOmoLogger(slog.NewJSONHandler(os.Stdout, nil))

	env, err := environment.NewEnvironmentService()
	if err != nil {
		return nil, err
	}

	err = tracing.InitTracing(ctx, env)

	db, err := database.NewDatabaseService(ctx, env)
	if err != nil {
		return nil, err
	}

	storageService, err := storage.NewStorageService(env)
	if err != nil {
		return nil, err
	}

	userService := users.NewUserService(db, env)

	blogService := blog.NewBlogService()

	photoService := photos.NewPhotoService(storageService, db)

	mappingService := mapping.NewMappingService()

	return &App{
		Database:    db,
		Users:       userService,
		Blog:        blogService,
		Photos:      photoService,
		Storage:     storageService,
		Mapping:     mappingService,
		Environment: env,
	}, nil
}

func (a *App) TearDown() {
	a.Database.TearDown()
}
