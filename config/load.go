package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Configuration defaults
func init() {
	viper.SetTypeByDefaultValue(true)
	viper.SetConfigType("toml")
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/sfm/eventrelay")
	viper.AddConfigPath("$HOME/.config/sfm/eventrelay")
	viper.SetEnvPrefix("SFM")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

// Load configuration
func Load(path string) error {
	if _, err := os.Stat(path); err != nil {
		viper.SetConfigFile(path)
	}
	return viper.ReadInConfig()
}
