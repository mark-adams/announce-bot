package announcebot

import (
	"github.com/kelseyhightower/envconfig"

	"gopkg.in/redis.v3"
)

// Configuration represents the configuration variables needed by the bot
type Configuration struct {
	Port            int    `default:"8080" required:"true"`
	HipchatAPIHost  string `envconfig:"HIPCHAT_API_HOST" default:"api.hipchat.com"`
	HipchatXMPPHost string `envconfig:"HIPCHAT_XMPP_HOST" default:"chat.hipchat.com:5222"`
	HipchatUser     string `envconfig:"HIPCHAT_USER"`
	HipchatPassword string `envconfig:"HIPCHAT_PASSWORD"`
	HipchatAPIToken string `envconfig:"HIPCHAT_API_TOKEN" required:"true"`
	AnnounceRoom    string `envconfig:"ANNOUNCE_ROOM" default:"-1"`
	TestRoom        string `envconfig:"TEST_ROOM" required:"true"`
	RedisAddress    string `envconfig:"REDIS_ADDRESS" default:"localhost:6379"`
	RedisPassword   string `envconfig:"REDIS_PASSWORD"`
	RedisDB         int64  `envconfig:"REDIS_DB" default:"0"`
}

// GetRedisOptions generates Redis options from the configuration
func (c *Configuration) GetRedisOptions() *redis.Options {
	return &redis.Options{
		Addr:     c.RedisAddress,
		Password: c.RedisPassword,
		DB:       c.RedisDB,
	}
}

// LoadConfigFromEnv loads a new Configuration instance from environment variables
func LoadConfigFromEnv(appname string) (*Configuration, error) {
	var config Configuration

	err := envconfig.Process(appname, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
