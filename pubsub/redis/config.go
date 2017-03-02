// Redis Pub/Sub Configuration

package redis

import "github.com/spf13/viper"

const (
	viper_redis_host_key   = "redis.host"
	viper_redis_topics_key = "redis.topics"
)

func init() {
	viper.SetDefault(viper_redis_host_key, "localhost:6379")
	viper.BindEnv(viper_redis_host_key)
}

type Configurer interface {
	Host() string
	Topics() []string
}

type Config struct{}

func (c Config) Host() string {
	return viper.GetString(viper_redis_host_key)
}

func (c Config) Topics() []string {
	return viper.GetStringSlice(viper_redis_topics_key)
}

func NewConfig() Config {
	return Config{}
}
