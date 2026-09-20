package config

import (
	"log"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	S3       S3Config
}

type AppConfig struct {
	Name  string `env:"APP_NAME" envDefault:"nwstep-api"`
	Env   string `env:"APP_ENV" envDefault:"development"`
	Debug bool   `env:"APP_DEBUG" envDefault:"true"`
}

type ServerConfig struct {
	Port         int           `env:"SERVER_PORT" envDefault:"8080"`
	ReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"0s"`
	WriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"0s"`
	RateLimitMax int           `env:"RATE_LIMIT_MAX" envDefault:"600"`
}

type DatabaseConfig struct {
	Host         string `env:"DB_HOST" envDefault:"localhost"`
	Port         int    `env:"DB_PORT" envDefault:"5432"`
	User         string `env:"DB_USER" envDefault:"postgres"`
	Password     string `env:"DB_PASSWORD" envDefault:"postgres"`
	DBName       string `env:"DB_NAME" envDefault:"nwstep"`
	SSLMode      string `env:"DB_SSLMODE" envDefault:"disable"`
	MaxOpenConns int    `env:"DB_MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns int    `env:"DB_MAX_IDLE_CONNS" envDefault:"25"`
}

type RedisConfig struct {
	Host     string `env:"REDIS_HOST" envDefault:"localhost"`
	Port     int    `env:"REDIS_PORT" envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

type JWTConfig struct {
	Secret        string        `env:"JWT_SECRET" envDefault:"your-super-secret-key-change-in-production"`
	AccessExpiry  time.Duration `env:"JWT_ACCESS_EXPIRY" envDefault:"15m"`
	RefreshExpiry time.Duration `env:"JWT_REFRESH_EXPIRY" envDefault:"168h"`
}

type S3Config struct {
	Endpoint  string `env:"S3_ENDPOINT" envDefault:"localhost:9000"`
	AccessKey string `env:"S3_ACCESS_KEY" envDefault:"minioadmin"`
	SecretKey string `env:"S3_SECRET_KEY" envDefault:"minioadmin"`
	Bucket    string `env:"S3_BUCKET" envDefault:"uploads"`
	UseSSL    bool   `env:"S3_USE_SSL" envDefault:"false"`
}

func Load() (*Config, error) {
	_ = godotenv.Load() // Ignore error if .env is missing, fallback to env vars
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Printf("Failed to parse config: %v", err)
		return nil, err
	}
	return &cfg, nil
}
