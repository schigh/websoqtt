package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mq "github.com/eclipse/paho.mqtt.golang"
	"github.com/fatih/color"
	"github.com/felixge/pidctrl"
)

var client mq.Client
var logger *log.Logger
var pid *pidctrl.PIDController
var proportional float64
var integral float64
var derivative float64
var setpoint float64
var ambient float64

func init() {
	logger = log.New(os.Stderr, color.BlueString("burner: "), log.Lmicroseconds)
	proportional = 1
	integral = 0.1
	derivative = 1
	setpoint = 72.0
	ambient = 72.0
}

// Setup - set up mqtt handler
func Setup() error {
	options := mq.NewClientOptions()
	options.AddBroker(fmt.Sprintf("%s://%s:%s",
		"tcp",
		"127.0.0.1",
		"1883",
	))
	options.SetClientID("BURNER")
	client = mq.NewClient(options)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		logger.Println("MQTT client connected")
	}

	if token := client.Subscribe("thermostat/setpoint", 0, handleSetpoint); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	if token := client.Subscribe("thermostat/temp", 0, handleTemperature); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// SendDemand - send demand back to the broker
func SendDemand(demand float64) {
	token := client.Publish("burner/demand", 0, false, fmt.Sprintf("%1.2f", demand))
	token.Wait()
}

// Teardown - close mqtt client
func Teardown() {
	client.Disconnect(250)
}

// handleSetpoint -
func handleSetpoint(c mq.Client, msg mq.Message) {
	logger.Println("setpoint: ", string(msg.Payload()))
	setpt, err := strconv.ParseFloat(string(msg.Payload()), 64)
	if err == nil {
		setpoint = setpt
		pid.Set(setpoint)
	}
}

// handleTemperature - handle updates to ambient temperature
func handleTemperature(c mq.Client, msg mq.Message) {
	logger.Println("temp: ", string(msg.Payload()))
	amb, err := strconv.ParseFloat(string(msg.Payload()), 64)
	if err == nil {
		ambient = amb
	}
}

func main() {
	if err := Setup(); err != nil {
		logger.Fatalln(err.Error())
	}
	defer Teardown()

	pid = pidctrl.NewPIDController(proportional, integral, derivative)
	pid.Set(setpoint)

	ticker := time.NewTicker(250 * time.Millisecond)
	go func() {
		for range ticker.C {
			delta := pid.Update(ambient)
			SendDemand(delta)
		}
	}()

	// create signal handlers for graceful shutdown
	// ---------------------------------------------------
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan)

	for {
		s := <-sigChan
		switch s {
		case syscall.SIGINT:
			fallthrough
		case syscall.SIGTERM:
			logger.Println("Closing gracefully...")
			Teardown()
			os.Exit(0)
		}
	}
}
