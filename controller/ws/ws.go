package ws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"../cfg"
	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/schigh/str"
)

type ThermostatMessage struct {
	MessageType    string `json:"type"`
	MessagePayload int64  `json:"payload"`
}

type DemandMessage struct {
	MessageType   string  `json:"type"`
	MessageDemand float64 `json:"demand"`
}

var conn *websocket.Conn
var logger *log.Logger
var AmbientTempHandler func(temp int64) error
var SetpointHandler func(setpoint int64) error

func init() {
	logger = log.New(os.Stderr, color.GreenString(str.Pad("websocket: ", " ", 12, str.PadLeft)), log.Lmicroseconds)
}

// Setup - set up websocket connection
func Setup() error {
	var err error

	config := cfg.SharedConfig()
	header := http.Header{}
	auth := base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s:%s", config.Websocket.Username, config.Websocket.Password)),
	)
	header.Add("Authorization", "Basic "+auth)
	conn, _, err = websocket.DefaultDialer.Dial(config.Websocket.Address, header)
	if err != nil {
		return err
	}

	logger.Println("Websocket client connected")

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				logger.Println(err.Error())
				break
			}
			if len(msg) == 0 {
				continue
			}

			tmsg := &ThermostatMessage{}
			if err := json.Unmarshal(msg, tmsg); err != nil {
				logger.Println(err.Error())
				break
			}

			if tmsg.MessageType == "temp" {
				if err := AmbientTempHandler(tmsg.MessagePayload); err != nil {
					logger.Println(err.Error())
					break
				}
			} else if tmsg.MessageType == "setpoint" {
				if err := SetpointHandler(tmsg.MessagePayload); err != nil {
					logger.Println(err.Error())
					break
				}
			}
		}
	}()

	return err
}

// SendDemand - Send demand down to subscribers
func SendDemand(demand string) {
	fdemand, err := strconv.ParseFloat(demand, 64)
	if err == nil {
		msg := &DemandMessage{
			MessageType:   "demand",
			MessageDemand: fdemand,
		}

		data, err := json.Marshal(msg)
		if err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

// Teardown - gracefully close websocket
func Teardown() {
	if conn != nil {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}
}
