package mqtt

import (
	"fmt"
	"log"
	"os"

	"../cfg"
	mq "github.com/eclipse/paho.mqtt.golang"
	"github.com/fatih/color"
	"github.com/schigh/str"
)

var client mq.Client
var logger *log.Logger
var DemandHandler func(demand string) error
var RoomTempHandler func(temp int64) error

func init() {
	logger = log.New(os.Stderr, color.BlueString(str.Pad("mqtt: ", " ", 12, str.PadLeft)), log.Lmicroseconds)
}

// Setup - set up all mqtt functionality
func Setup() error {
	config := cfg.SharedConfig()
	options := mq.NewClientOptions()
	options.AddBroker(fmt.Sprintf("%s://%s:%s",
		config.MQTT.Transport,
		config.MQTT.Address,
		config.MQTT.Port,
	))
	options.SetClientID(config.MQTT.ClientID)
	client = mq.NewClient(options)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	} else {
		logger.Println("MQTT client connected")
	}

	if token := client.Subscribe(config.MQTT.Topics.Demand, 0, handleDemand); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// SendAmbientTemp - Send the received temperature along to the MQTT broker
func SendAmbientTemp(temp int64) {
	config := cfg.SharedConfig()
	token := client.Publish(config.MQTT.Topics.Temperature, 0, false, fmt.Sprintf("%d", temp))
	token.Wait()
}

// SendSetpoint - Send the received setpoint along to the MQTT broker
func SendSetpoint(setpoint int64) {
	config := cfg.SharedConfig()
	token := client.Publish(config.MQTT.Topics.Setpoint, 0, false, fmt.Sprintf("%d", setpoint))
	token.Wait()
}

// handleDemand - handle messages for demand coming off MQTT
func handleDemand(c mq.Client, msg mq.Message) {
	DemandHandler(string(msg.Payload()))
}

// Teardown - clean up
func Teardown() {
	client.Disconnect(250)
}
