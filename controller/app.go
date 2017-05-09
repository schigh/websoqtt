package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"./cfg"
	"./mqtt"
	"./ws"
	"syscall"
)

var configPath string
var basicAuthUser string
var basicAuthPass string

func init() {
	flag.StringVar(&configPath, "c", "config.json", "Path to config file")
	flag.StringVar(&basicAuthUser, "u", "", "HTTP Basic auth user name")
	flag.StringVar(&basicAuthPass, "p", "", "HTTP Basic auth password")
}

func main() {
	flag.Parse()
	// set basic username and password if passed as params
	// ---------------------------------------------------
	if basicAuthUser != "" {
		os.Setenv("WEBSOCKET_USERNAME", basicAuthUser)
	}

	if basicAuthPass != "" {
		os.Setenv("WEBSOCKET_PASSWORD", basicAuthPass)
	}

	// set up config first
	// ---------------------------------------------------
	if err := cfg.Setup(configPath); err != nil {
		log.Fatalln("Error setting up config: ", err.Error())
	}

	// set up websockets
	// ---------------------------------------------------
	ws.AmbientTempHandler = func(temp int64) error {
		log.Println("handling ambient temp")
		mqtt.SendAmbientTemp(temp)
		return nil
	}
	ws.SetpointHandler = func(setpoint int64) error {
		log.Println("handling setpoint")
		mqtt.SendSetpoint(setpoint)
		return nil
	}
	if err := ws.Setup(); err != nil {
		log.Fatalln("Error setting up websockets: ", err.Error())
	}

	// set up mqtt
	// ---------------------------------------------------
	mqtt.DemandHandler = func(demand string) error {
		ws.SendDemand(demand)
		return nil
	}
	if err := mqtt.Setup(); err != nil {
		log.Fatalln("Error setting up MQTT: ", err.Error())
	}

	// create signal handlers for graceful shutdown
	// ---------------------------------------------------
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan)

	for {
		s := <-sigChan
		switch s {
		case syscall.SIGINT:
			fallthrough
		case syscall.SIGTERM:
			log.Println("Closing gracefully...")
			mqtt.Teardown()
			ws.Teardown()
			os.Exit(0)
		}
	}
}
