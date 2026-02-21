package config

import (
	"errors"
	"fmt"
	"strings"

	"git.nonahob.net/jacob/golibs/datastores/sql/postgres"
	"github.com/spf13/viper"
)

type Config struct {
	ListenAddr   string   `mapstructure:"listen_addr"`
	DB           DBConfig `mapstructure:"db"`
	ArtifactPath string   `mapstructure:"artifact_path"`
	KubeConfig   string   `mapstructure:"kubeconfig"`
	LogLevel     string   `mapstructure:"log_level"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

func (db *DBConfig) PostgresConfig() *postgres.Config {
	return &postgres.Config{
		Host:     db.Host,
		Port:     db.Port,
		User:     db.User,
		Password: db.Password,
		DBName:   db.Name,
		SSLMode:  db.SSLMode,
	}
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetDefault("listen_addr", ":8080")
	v.SetDefault("artifact_path", "/var/lib/shipinator/artifacts")
	v.SetDefault("log_level", "info")
	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", "5432")
	v.SetDefault("db.user", "postgres")
	v.SetDefault("db.password", "postgres")
	v.SetDefault("db.ssl_mode", "disable")

	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/shipinator")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	v.SetEnvPrefix("SHIPINATOR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Explicitly bind nested db keys so SHIPINATOR_DB_HOST etc. work.
	for _, key := range []string{
		"db.host", "db.port", "db.user", "db.password", "db.name", "db.ssl_mode",
		"kubeconfig",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("binding env for %s: %w", key, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if cfg.DB.Name == "" {
		return nil, fmt.Errorf("db.name is required")
	}

	return &cfg, nil
}
