package dimulai

import "github.com/dimframework/dim"

type AppConfig struct {
	AppName string
	dim.Config
}

func LoadAppConfig() (*AppConfig, error) {
	dcfg, err := dim.LoadConfig()

	if err != nil {
		return nil, err
	}

	cfg := &AppConfig{
		AppName: dim.GetEnvOrDefault("APP_NAME", "Dimulai Starter Kit"),
		Config:  *dcfg,
	}

	return cfg, nil
}
