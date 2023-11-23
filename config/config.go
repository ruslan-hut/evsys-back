package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"sync"
)

type Config struct {
	IsDebug     bool   `yaml:"is_debug" env-default:"false"`
	LogRecords  int64  `yaml:"log_records" env-default:"0"`
	FirebaseKey string `yaml:"firebase_key" env-default:""`
	Listen      struct {
		Type     string `yaml:"type" env-default:"port"`
		BindIP   string `yaml:"bind_ip" env-default:"0.0.0.0"`
		Port     string `yaml:"port" env-default:"5000"`
		TLS      bool   `yaml:"tls_enabled" env-default:"false"`
		CertFile string `yaml:"cert_file" env-default:""`
		KeyFile  string `yaml:"key_file" env-default:""`
	} `yaml:"listen"`
	CentralSystem struct {
		Enabled bool   `yaml:"enabled" env-default:"false"`
		Url     string `yaml:"url" env-default:""`
		Token   string `yaml:"token" env-default:""`
	} `yaml:"central_system"`
	Pusher struct {
		Enabled bool   `yaml:"enabled" env-default:"false"`
		AppID   string `yaml:"app_id" env-default:""`
		Key     string `yaml:"key" env-default:""`
		Secret  string `yaml:"secret" env-default:""`
		Cluster string `yaml:"cluster" env-default:"eu"`
	} `yaml:"pusher"`
	Mongo struct {
		Enabled  bool   `yaml:"enabled" env-default:"false"`
		Host     string `yaml:"host" env-default:"127.0.0.1"`
		Port     string `yaml:"port" env-default:"27017"`
		User     string `yaml:"user" env-default:"admin"`
		Password string `yaml:"password" env-default:"pass"`
		Database string `yaml:"database" env-default:""`
	} `yaml:"mongo"`
}

var instance *Config
var once sync.Once

func GetConfig(path string) (*Config, error) {
	var err error
	once.Do(func() {
		instance = &Config{}
		if err = cleanenv.ReadConfig(path, instance); err != nil {
			desc, _ := cleanenv.GetDescription(instance, nil)
			err = fmt.Errorf("%s; %s", err, desc)
			instance = nil
		}
	})
	return instance, err
}
