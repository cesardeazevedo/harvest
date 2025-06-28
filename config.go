package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/nbd-wtf/go-nostr"
	"gopkg.in/yaml.v3"
)

type RelayConfig struct {
	URL      string `yaml:"url"`
	Until    string `yaml:"until"`
	Interval int    `yaml:"interval"`
}

type AppConfig struct {
	Relays []RelayConfig `yaml:"relays"`
	Filter nostr.Filter  `yaml:"filter"`
	path   string
}

func Load(path string) (*AppConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file not found")
	}
	var app AppConfig

	err = yaml.Unmarshal(file, &app)
	if err != nil {
		return nil, err
	}
	app.path = path
	return &app, nil
}

func (config AppConfig) UpdateUntil(url string, until time.Time) {
	eval := yqlib.NewStringEvaluator()
	encoder := yqlib.NewYamlEncoder(yqlib.YamlPreferences{})
	decoder := yqlib.NewYamlDecoder(yqlib.YamlPreferences{})
	for i := range config.Relays {
		if config.Relays[i].URL == url {
			formatted := until.Format("January 2, 2006 15:04:05")
			config.Relays[i].Until = formatted

			file, err := os.ReadFile(config.path)
			if err != nil {
				continue
			}
			res, _ := eval.Evaluate(
				fmt.Sprintf(".relays[%d].until = \"%s\"", i, formatted),
				string(file),
				encoder,
				decoder,
			)
			os.WriteFile(config.path, []byte(res), 0644)
		}
	}
}
