package main

import (
	"encoding/base64"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"goji.io"
	"goji.io/pat"
)

type wsclient struct {
	sync.Mutex
	sock *websocket.Conn
	dead bool
}

const (
	socketBufferSize = 1024
)

var publisher *wsclient
var subscribers []wsclient
var upgrader websocket.Upgrader
var username string
var password string
var logger *log.Logger

func init() {
	upgrader = websocket.Upgrader{
		ReadBufferSize:  socketBufferSize,
		WriteBufferSize: socketBufferSize,
	}
	subscribers = []wsclient{}

	flag.StringVar(&username, "u", "", "basic auth user name")
	flag.StringVar(&password, "p", "", "basic auth password")

	logger = log.New(os.Stderr, color.CyanString("server: "), log.Lmicroseconds)
}

// broadcastToSubscribers - send a message to all subscribers
func broadcastToSubscribers(msg []byte) {
	if len(msg) == 0 {
		return
	}
	for _, client := range subscribers {
		if !client.dead {
			if err := client.sock.WriteMessage(websocket.TextMessage, msg); err != nil {
				client.dead = true
				client.sock.Close()
			}
		}
	}
}

// broadcastToPublisher - send a message back to the monitor
func broadcastToPublisher(msg []byte) {
	if len(msg) == 0 || publisher == nil {
		return
	}
	logger.Println("Sending message to controller: ", string(msg))
	publisher.Lock()
	if err := publisher.sock.WriteMessage(websocket.TextMessage, msg); err != nil {
		publisher.sock.Close()
		publisher.Unlock()
		publisher = nil
		return
	}
	publisher.Unlock()
}

// joinPublisher - Join the publisher (controller)
func joinPublisher(writer http.ResponseWriter, req *http.Request) {
	socket, err := upgrader.Upgrade(writer, req, nil)
	if err != nil {
		logger.Println("joinPublisher: ", err.Error())
		return
	}

	if publisher != nil {
		http.Error(writer, "Publisher already attached", 409)
	}
	logger.Println("Publisher joined")

	publisher = &wsclient{
		sock: socket,
	}

	for {
		if publisher == nil {
			break
		}

		_, msg, err := publisher.sock.ReadMessage()
		if err != nil {
			// this is most likely a disconnect
			logger.Println("unable to read publisher message: ", err.Error())
			publisher = nil
			break
		}
		logger.Println(string(msg))

		broadcastToSubscribers(msg)
	}
}

// joinSubscriber - join a subscriber (web client)
func joinSubscriber(writer http.ResponseWriter, req *http.Request) {
	socket, err := upgrader.Upgrade(writer, req, nil)
	if err != nil {
		logger.Println("joinSubscriber: ", err.Error())
		return
	}

	sub := wsclient{
		sock: socket,
	}
	logger.Println("Joining subscriber: ", sub)

	subscribers = append(subscribers, sub)

	for {
		_, msg, err := sub.sock.ReadMessage()
		logger.Println("Received temperature update: ", string(msg))
		if err != nil {
			// this is most likely a disconnect
			logger.Println("unable to read subscriber message: ", err.Error())
			sub.dead = true
			sub.sock.Close()
			break
		}
		broadcastToPublisher(msg)
	}
}

// serveHTML - serve the index.html file
func serveHTML(writer http.ResponseWriter, req *http.Request) {
	http.ServeFile(writer, req, "index.html")
}

// accessDenied - the handler for failed basic auth middleware
func accessDenied(writer http.ResponseWriter) {
	writer.Header().Set("WWW-Authenticate", `Basic realm="gophertown"`)
	writer.WriteHeader(401)
	writer.Write([]byte("401 Unauthorized\n"))
}

// basicAuthMiddleware - handle HTTP Basic authentication for all requests
func basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		tokens := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
		if len(tokens) != 2 {
			accessDenied(writer)
			return
		}

		b, err := base64.StdEncoding.DecodeString(tokens[1])
		if err != nil {
			accessDenied(writer)
			return
		}

		pair := strings.SplitN(string(b), ":", 2)
		if len(pair) != 2 {
			accessDenied(writer)
			return
		}

		if pair[0] != username || pair[1] != password {
			accessDenied(writer)
			return
		}

		next.ServeHTTP(writer, req)
	})
}

func main() {
	flag.Parse()

	if username == "" || password == "" {
		logger.Fatalln("both the username (-u) and password (-p) parameters are required")
	}

	mux := goji.NewMux()
	mux.Use(basicAuthMiddleware)
	mux.HandleFunc(pat.Get("/"), serveHTML)
	mux.HandleFunc(pat.Get("/publisher/join"), joinPublisher)
	mux.HandleFunc(pat.Get("/subscriber/join"), joinSubscriber)

	logger.Fatal(http.ListenAndServe("0.0.0.0:8888", mux))
}
