package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

type Config struct {
	MQTT struct {
		Address   string `json:"address"`
		Port      string `json:"port"`
		Transport string `json:"transport"`
		ClientID  string `json:"client_id"`
		Topics    struct {
			Temperature string `json:"temperature"`
			Demand      string `json:"demand"`
			Setpoint    string `json:"setpoint"`
			BurnerTemp  string `json:"burner_temp"`
		} `json:"topics"`
	} `json:"mqtt"`
	Websocket struct {
		Address  string `json:"address"`
		Username string `json:"-"`
		Password string `json:"-"`
	} `json:"websocket"`
}

var config *Config

// parse - parses the config file
func parse(cfgPath string) error {
	fileinfo, err := os.Stat(cfgPath)
	if err != nil {
		return err
	}

	if fileinfo.IsDir() {
		return fmt.Errorf("%s is a directory", cfgPath)
	}

	fileData, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	config = &Config{}
	if err = json.Unmarshal(fileData, config); err != nil {
		return err
	}

	config.Websocket.Username = os.Getenv("WEBSOCKET_USERNAME")
	config.Websocket.Password = os.Getenv("WEBSOCKET_PASSWORD")

	return nil
}

// Setup - Sets up the application config by loading json file
func Setup(cfgPath string) error {
	if os.Getenv("WEBSOCKET_USERNAME") == "" || os.Getenv("WEBSOCKET_PASSWORD") == "" {
		return errors.New("HTTP Basic auth requires a user name and password")
	}
	if err := parse(cfgPath); err != nil {
		return err
	}

	return nil
}

// SharedConfig - gets the singleton config instance
func SharedConfig() *Config {
	return config
}
