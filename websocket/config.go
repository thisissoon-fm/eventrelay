package websocket

import "github.com/spf13/viper"

const (
	viper_bind_key     = "websocket.bind"
	viper_username_key = "websocket.username"
	viper_password_key = "websocket.password"
)

func init() {
	viper.SetDefault(viper_bind_key, ":8000")
	viper.SetDefault(viper_username_key, "username")
	viper.SetDefault(viper_password_key, "password")
}

type Configurer interface {
	Bind() string
	Username() string
	Password() string
}

type Config struct{}

func (c Config) Bind() string {
	viper.BindEnv(viper_bind_key)
	return viper.GetString(viper_bind_key)
}

func (c Config) Username() string {
	viper.BindEnv(viper_username_key)
	return viper.GetString(viper_username_key)
}

func (c Config) Password() string {
	viper.BindEnv(viper_password_key)
	return viper.GetString(viper_password_key)
}

func NewServerConfig() Config {
	return Config{}
}
