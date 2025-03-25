package environment

import (
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type EnvironmentService struct {
	BaseURL            string
	DbHost             string
	DbPort             string
	DbName             string
	DbUser             string
	DbPassword         string
	GithubClientId     string
	GithubClientSecret string

	StorageAccessKeyID     string
	StorageSecretAccessKey string
	StorageEndpoint        string

	Env Environment
}

type Environment int

const (
	Local Environment = iota
	Production
)

func (e Environment) String() string {
	switch e {
	case Local:
		return "Local"
	case Production:
		return "Production"
	default:
		return "Unknown"
	}
}

func NewEnvironmentService() (*EnvironmentService, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	env := &EnvironmentService{}
	if err := env.load(); err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)
	}
	return env, nil
}

func (e *EnvironmentService) load() error {
	e.BaseURL = e.getEnvOrDefault("BASE_URL", "http://localhost:6900")
	e.DbHost = e.getEnvOrDefault("DB_HOST", "localhost")
	e.DbName = e.getEnvOrDefault("DB_NAME", "oliverbutler")
	e.DbUser = e.getEnvOrDefault("DB_USER", "postgres")
	e.DbPassword = e.getEnvOrDefault("DB_PASSWORD", "password")

	e.DbPort = e.getEnvOrDefault("DB_PORT", "5432")

	e.GithubClientId = e.getEnvOrDefault("GITHUB_CLIENT_ID", "")
	e.GithubClientSecret = e.getEnvOrDefault("GITHUB_CLIENT_SECRET", "")

	e.StorageAccessKeyID = e.getEnvOrDefault("STORAGE_ACCESS_KEY_ID", "")
	e.StorageSecretAccessKey = e.getEnvOrDefault("STORAGE_SECRET_ACCESS_KEY", "")
	e.StorageEndpoint = e.getEnvOrDefault("STORAGE_ENDPOINT", "")

	envString := e.getEnvOrDefault("ENV", "local")
	if envString == "production" {
		e.Env = Production
	} else {
		e.Env = Local
	}

	return nil
}

func (e *EnvironmentService) getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func (e *EnvironmentService) GetDomain() string {
	parsedURL, err := url.Parse(e.BaseURL)
	if err != nil {
		return ""
	}
	return parsedURL.Hostname()
}
