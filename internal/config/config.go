package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"CampusMonitorAPI/internal/logger"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	MQTT     MQTTConfig
	Security SecurityConfig
	Logging  LoggingConfig
	Auth     AuthConfig
}
type AuthConfig struct {
	OAuthProviders          map[string]OAuthProviderConfig
	JWTSecret               string
	FrontendURL             string
	JWTExpiry               time.Duration
	RefreshTokenExpiry      time.Duration
	EnableLocalLogin        bool
	EnableRegistration      bool
	Require2FA              bool
	EnableAdminRegistration bool
}

type OAuthProviderConfig struct {
	Scopes       []string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
}

type ServerConfig struct {
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	Host            string
	PublicURL       string
	CertDir         string
	Environment     string
	Port            int
	MaxHeaderBytes  int
}

type DatabaseConfig struct {
	Host            string
	User            string
	Password        string
	Database        string
	SSLMode         string
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	Port            int
	MaxOpenConns    int
	MaxIdleConns    int
}

type MQTTConfig struct {
	Broker         string
	ClientID       string
	Username       string
	Password       string
	TelemetryTopic string
	CommandTopic   string
	KeepAlive      time.Duration
	ConnectTimeout time.Duration
	Port           int
	QoS            byte
	RetainMessages bool
	AutoReconnect  bool
}

type SecurityConfig struct {
	CORSAllowedOrigins []string
	CORSAllowedMethods []string
	JWTSecret          string
	APIKeyHeader       string
	JWTExpirationHours int
	RateLimitPerMinute int
	EnableRateLimit    bool
}

type LoggingConfig struct {
	FilePath  string
	Level     logger.Level
	Mode      logger.Mode
	UseColors bool
}

var requiredEnvVars = []string{
	"DB_HOST",
	"DB_PORT",
	"DB_USER",
	"DB_PASSWORD",
	"DB_NAME",
	"MQTT_BROKER",
	"MQTT_PORT",
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("loading .env: %w", err)
	}
	if err := validateRequired(); err != nil {
		return nil, err
	}
	cfg := &Config{
		Server:   loadServerConfig(),
		Database: loadDatabaseConfig(),
		MQTT:     loadMQTTConfig(),
		Security: loadSecurityConfig(),
		Logging:  loadLoggingConfig(),
		Auth:     loadAuthConfig(),
	}

	return cfg, nil
}
func loadAuthConfig() AuthConfig {
	jwtSecret := getEnv("JWT_SECRET", "change_me_in_production")
	if jwtSecret == "change_me_in_production" {
		logger.Warn("Please remember to change the jwt secret from default value")
	}
	jwtExpiry := getEnvAsDuration("JWT_EXPIRY", "24h")
	refreshExpiry := getEnvAsDuration("REFRESH_TOKEN_EXPIRY", "720h")

	providers := make(map[string]OAuthProviderConfig)
	// TODO: CONSIDER THIS TO BE A PART OF ENVS
	// Google OAuth
	if id := getEnv("GOOGLE_CLIENT_ID", ""); id != "" {
		providers["google"] = OAuthProviderConfig{
			ClientID:     id,
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			AuthURL:      "https://accounts.google.com/o/oauth2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		}
	}

	// GitHub OAuth
	if id := getEnv("GITHUB_CLIENT_ID", ""); id != "" {
		providers["github"] = OAuthProviderConfig{
			ClientID:     id,
			ClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scopes:       []string{"user:email"},
		}
	}

	// PocketID (example custom OAuth)
	if id := getEnv("POCKETID_CLIENT_ID", ""); id != "" {
		providers["pocketid"] = OAuthProviderConfig{
			ClientID:     id,
			ClientSecret: getEnv("POCKETID_CLIENT_SECRET", ""),
			AuthURL:      getEnv("POCKETID_AUTH_URL", ""),
			TokenURL:     getEnv("POCKETID_TOKEN_URL", ""),
			UserInfoURL:  getEnv("POCKETID_USERINFO_URL", ""),
			Scopes:       strings.Split(getEnv("POCKETID_SCOPES", "openid profile email"), ","),
		}
	}

	return AuthConfig{
		JWTSecret:               jwtSecret,
		JWTExpiry:               jwtExpiry,
		RefreshTokenExpiry:      refreshExpiry,
		EnableLocalLogin:        getEnvAsBool("ENABLE_LOCAL_LOGIN", true),
		EnableRegistration:      getEnvAsBool("ENABLE_REGISTRATION", true),
		Require2FA:              getEnvAsBool("REQUIRE_2FA", false),
		OAuthProviders:          providers,
		FrontendURL:             getEnv("FRONTEND_URL", "http://localhost:5173"),
		EnableAdminRegistration: getEnvAsBool("ENABLE_ADMIN_REGISTRATION", false),
	}
}
func validateRequired() error {
	var missing []string

	for _, key := range requiredEnvVars {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

func loadServerConfig() ServerConfig {
	return ServerConfig{
		Host:            getEnv("SERVER_HOST", "0.0.0.0"),
		Port:            getEnvAsInt("SERVER_PORT", 8080),
		Environment:     getEnv("ENVIRONMENT", "development"),
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", "15s"),
		ReadTimeout:     getEnvAsDuration("READ_TIMEOUT", "10s"),
		WriteTimeout:    getEnvAsDuration("WRITE_TIMEOUT", "10s"),
		MaxHeaderBytes:  getEnvAsInt("MAX_HEADER_BYTES", 1048576),
		PublicURL:       getEnv("PUBLIC_URL", "http://localhost:9080"),
		CertDir:         getEnv("CERT_DIR", "certs"),
	}
}

func loadDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnvAsInt("DB_PORT", 5432),
		User:            getEnv("DB_USER", "campus_admin"),
		Password:        getEnv("DB_PASSWORD", ""),
		Database:        getEnv("DB_NAME", "campus_monitor"),
		SSLMode:         getEnv("DB_SSL_MODE", "disable"),
		MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", "5m"),
		ConnMaxIdleTime: getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", "5m"),
	}
}

func loadMQTTConfig() MQTTConfig {
	return MQTTConfig{
		Broker:         getEnv("MQTT_BROKER", "localhost"),
		Port:           getEnvAsInt("MQTT_PORT", 1883),
		ClientID:       getEnv("MQTT_CLIENT_ID", "campus-backend"),
		Username:       getEnv("MQTT_USERNAME", ""),
		Password:       getEnv("MQTT_PASSWORD", ""),
		TelemetryTopic: getEnv("MQTT_TELEMETRY_TOPIC", "campus/probes/telemetry"),
		CommandTopic:   getEnv("MQTT_COMMAND_TOPIC", "campus/probes/+/cmd"),
		QoS:            byte(getEnvAsInt("MQTT_QOS", 1)),
		RetainMessages: getEnvAsBool("MQTT_RETAIN", false),
		KeepAlive:      getEnvAsDuration("MQTT_KEEP_ALIVE", "60s"),
		ConnectTimeout: getEnvAsDuration("MQTT_CONNECT_TIMEOUT", "10s"),
		AutoReconnect:  getEnvAsBool("MQTT_AUTO_RECONNECT", true),
	}
}

func loadSecurityConfig() SecurityConfig {
	origins := getEnv("CORS_ALLOWED_ORIGINS", "*")
	methods := getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS")

	return SecurityConfig{
		JWTSecret:          getEnv("JWT_SECRET", "campus_monitor_secret_change_in_production"),
		JWTExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		APIKeyHeader:       getEnv("API_KEY_HEADER", "X-API-Key"),
		CORSAllowedOrigins: strings.Split(origins, ","),
		CORSAllowedMethods: strings.Split(methods, ","),
		RateLimitPerMinute: getEnvAsInt("RATE_LIMIT_PER_MINUTE", 100),
		EnableRateLimit:    getEnvAsBool("ENABLE_RATE_LIMIT", true),
	}
}

func loadLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:     logger.ParseLevel(getEnv("LOG_LEVEL", "info")),
		Mode:      logger.ParseMode(getEnv("LOG_MODE", "normal")),
		FilePath:  getEnv("LOG_FILE_PATH", ""),
		UseColors: getEnvAsBool("LOG_USE_COLORS", true),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	duration, _ := time.ParseDuration(defaultValue)
	return duration
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Database,
		c.Database.SSLMode,
	)
}

func (c *Config) GetMQTTBroker() string {
	return fmt.Sprintf("tcp://%s:%d", c.MQTT.Broker, c.MQTT.Port)
}

func (c *Config) Validate() error {
	var errors []string

	if c.Database.Password == "" {
		errors = append(errors, "DB_PASSWORD cannot be empty")
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errors = append(errors, "SERVER_PORT must be between 1 and 65535")
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		errors = append(errors, "DB_PORT must be between 1 and 65535")
	}

	if c.MQTT.Port < 1 || c.MQTT.Port > 65535 {
		errors = append(errors, "MQTT_PORT must be between 1 and 65535")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

func (c *Config) Print() {
	fmt.Println("Campus Monitor config summary \n {")
	fmt.Printf("\n Environment:     %s\n", c.Server.Environment)
	fmt.Printf("Server:          %s:%d\n", c.Server.Host, c.Server.Port)
	fmt.Printf("Database:        %s:%d/%s\n", c.Database.Host, c.Database.Port, c.Database.Database)
	fmt.Printf("MQTT Broker:     %s:%d\n", c.MQTT.Broker, c.MQTT.Port)
	fmt.Printf("\n } \n ")
}
