package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

var (
	cfg  *Config
	once sync.Once
)

// Config holds all configuration for the gateway server
type Config struct {
	ServerConfig  ServerConfig
	RedisConfig   RedisConfig
	ResourceConfig ResourceConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// ResourceConfig holds system resource limits
type ResourceConfig struct {
	MaxFDLimit      float64
	FDDangerRatio   float64
	MaxSessions     int
	MaxSessionsPerUser int
}

// Load loads configuration from file and environment variables
func Load() error {
	var err error
	once.Do(func() {
		// Set defaults
		viper.SetDefault("server.port", 8080)
		viper.SetDefault("server.read_timeout", 15)
		viper.SetDefault("server.write_timeout", 15)
		viper.SetDefault("server.idle_timeout", 60)
		viper.SetDefault("redis.addr", "localhost:6379")
		viper.SetDefault("redis.password", "")
		viper.SetDefault("redis.db", 0)
		viper.SetDefault("resource.max_fd_limit", 4096)
		viper.SetDefault("resource.fd_danger_ratio", 0.85)
		viper.SetDefault("resource.max_sessions", 1000)
		viper.SetDefault("resource.max_sessions_per_user", 20)

		// Allow environment variables to override
		viper.AutomaticEnv()

		// Read config file if present
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")

		if err = viper.ReadInConfig(); err != nil {
			// Config file is optional, continue with defaults
			err = nil
		}

		cfg = &Config{
			ServerConfig: ServerConfig{
				Port:         viper.GetInt("server.port"),
				ReadTimeout:  viper.GetInt("server.read_timeout"),
				WriteTimeout: viper.GetInt("server.write_timeout"),
				IdleTimeout:  viper.GetInt("server.idle_timeout"),
			},
			RedisConfig: RedisConfig{
				Addr:     viper.GetString("redis.addr"),
				Password: viper.GetString("redis.password"),
				DB:       viper.GetInt("redis.db"),
			},
			ResourceConfig: ResourceConfig{
				MaxFDLimit:          viper.GetFloat64("resource.max_fd_limit"),
				FDDangerRatio:       viper.GetFloat64("resource.fd_danger_ratio"),
				MaxSessions:         viper.GetInt("resource.max_sessions"),
				MaxSessionsPerUser:  viper.GetInt("resource.max_sessions_per_user"),
			},
		}
	})
	return err
}

// Get returns the current configuration
func Get() *Config {
	if cfg == nil {
		panic("config not loaded, call Load() first")
	}
	return cfg
}

// String returns a human-readable representation of the config
func (c *Config) String() string {
	return fmt.Sprintf("Server{Port:%d}, Redis{%s}, Resource{MaxSessions:%d, MaxSessionsPerUser:%d}",
		c.ServerConfig.Port,
		c.RedisConfig.Addr,
		c.ResourceConfig.MaxSessions,
		c.ResourceConfig.MaxSessionsPerUser,
	)
}
