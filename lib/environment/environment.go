package environment

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type EnvironmentService struct {
	baseURL        string
	dbHost         string
	dbPort         string
	dbName         string
	dbUser         string
	dbPassword     string
	minioEndpoint  string
	minioAccessKey string
	minioSecretKey string
	minioUseSSL    bool
	env            Environment
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
	e.baseURL = e.getEnvOrDefault("BASE_URL", "http://localhost:6900")
	e.dbHost = e.getEnvOrDefault("DB_HOST", "localhost")
	e.dbName = e.getEnvOrDefault("DB_NAME", "mydb")
	e.dbUser = e.getEnvOrDefault("DB_USER", "user")
	e.dbPassword = e.getEnvOrDefault("DB_PASSWORD", "")

	e.dbPort = e.getEnvOrDefault("DB_PORT", "5432")

	e.minioEndpoint = e.getEnvOrDefault("MINIO_ENDPOINT", "")
	e.minioAccessKey = e.getEnvOrDefault("MINIO_ACCESS_KEY", "")
	e.minioSecretKey = e.getEnvOrDefault("MINIO_SECRET_KEY", "")
	e.minioUseSSL = e.getEnvAsBool("MINIO_USE_SSL", false)

	envString := e.getEnvOrDefault("ENV", "local")
	if envString == "production" {
		e.env = Production
	} else {
		e.env = Local
	}

	return nil
}

func (e *EnvironmentService) getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func (e *EnvironmentService) getEnvAsInt(key string, defaultValue int) (int, error) {
	if value, exists := os.LookupEnv(key); exists {
		return strconv.Atoi(value)
	}
	return defaultValue, nil
}

func (e *EnvironmentService) getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1"
	}
	return defaultValue
}

// Getter methods
func (e *EnvironmentService) GetBaseURL() string {
	return e.baseURL
}

func (e *EnvironmentService) GetDomain() string {
	parsedURL, err := url.Parse(e.baseURL)
	if err != nil {
		return ""
	}
	return parsedURL.Hostname()
}

func (e *EnvironmentService) GetDbHost() string {
	return e.dbHost
}

func (e *EnvironmentService) GetDbPort() string {
	return e.dbPort
}

func (e *EnvironmentService) GetDbName() string {
	return e.dbName
}

func (e *EnvironmentService) GetDbUser() string {
	return e.dbUser
}

func (e *EnvironmentService) GetDbPassword() string {
	return e.dbPassword
}

func (e *EnvironmentService) GetMinioEndpoint() string {
	return e.minioEndpoint
}

func (e *EnvironmentService) GetMinioAccessKey() string {
	return e.minioAccessKey
}

func (e *EnvironmentService) GetMinioSecretKey() string {
	return e.minioSecretKey
}

func (e *EnvironmentService) GetMinioUseSSL() bool {
	return e.minioUseSSL
}

func (e *EnvironmentService) GetEnv() Environment {
	return e.env
}
