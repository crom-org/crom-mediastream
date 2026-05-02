package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	StreamURL      string `mapstructure:"stream_url"`
	StreamKey      string `mapstructure:"stream_key"`
	VideoDir       string `mapstructure:"video_dir"`
	Resolution     string `mapstructure:"resolution"`
	FPS            int    `mapstructure:"fps"`
	TwitchClientID string `mapstructure:"twitch_client_id"`
	TwitchToken    string `mapstructure:"twitch_token"`
	TwitchUserID   string `mapstructure:"twitch_user_id"`

	AutoDJEnabled bool `mapstructure:"auto_dj"`
	LoopEnabled   bool `mapstructure:"loop"`
	ChatEnabled   bool `mapstructure:"chat"`
	ScrollEnabled bool `mapstructure:"scroll"`
	Port          string `mapstructure:"port"`
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	// Defaults
	viper.SetDefault("video_dir", "./videos")
	viper.SetDefault("resolution", "1280x720")
	viper.SetDefault("fps", 30)
	viper.SetDefault("port", "8080")
	viper.SetDefault("auto_dj", false)
	viper.SetDefault("loop", true)
	viper.SetDefault("chat", false)
	viper.SetDefault("scroll", true)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// If config file not found, we expect environment variables or flags
	}

	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) GetFullStreamURL() string {
	return fmt.Sprintf("%s/%s", c.StreamURL, c.StreamKey)
}
