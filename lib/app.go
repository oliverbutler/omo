package lib

import (
	"oliverbutler/lib/blog"
	"oliverbutler/lib/database"
	"oliverbutler/lib/environment"
	"oliverbutler/lib/mapping"
	"oliverbutler/lib/photos"
	"oliverbutler/lib/storage"
	"oliverbutler/lib/users"
	"oliverbutler/lib/workflow"
)

type App struct {
	Environment *environment.EnvironmentService
	Database    *database.DatabaseService
	Storage     *storage.StorageService
	Users       *users.UserService
	Blog        *blog.BlogService
	Photos      *photos.PhotoService
	Mapping     *mapping.MappingService
	Workflow    *workflow.WorkflowService
}

// Single place services are instantiated, and environment variables are read and passed to the services.
// Gives a birds eye view of module dependencies, both internal and external.
func NewApp() (*App, error) {
	env, err := environment.NewEnvironmentService()
	if err != nil {
		return nil, err
	}

	db, err := database.NewDatabaseService(env)
	if err != nil {
		return nil, err
	}

	storageService, err := storage.NewStorageService(env)
	if err != nil {
		return nil, err
	}

	workflowService, err := workflow.NewWorkflowService()
	if err != nil {
		return nil, err
	}

	userService := users.NewUserService(db, env)

	blogService := blog.NewBlogService()

	photoService := photos.NewPhotoService(storageService, db, workflowService)

	mappingService := mapping.NewMappingService()

	/// At the end, the background worker is started.
	workflowService.StartBackgroundWorker()

	return &App{
		Database:    db,
		Users:       userService,
		Blog:        blogService,
		Photos:      photoService,
		Storage:     storageService,
		Mapping:     mappingService,
		Environment: env,
		Workflow:    workflowService,
	}, nil
}

func (a *App) TearDown() {
	a.Database.TearDown()
	a.Workflow.TearDown()
}
