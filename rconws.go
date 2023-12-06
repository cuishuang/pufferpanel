package pufferpanel

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/spf13/cast"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type RCONWSConnection struct {
	io.WriteCloser
	IP               string
	Port             string
	Password         string
	Reconnect        bool
	Logger           *log.Logger
	connection       *websocket.Conn
	ready            bool
	identifier       int
	identifierLocker sync.Mutex
}

type rconwsMessage struct {
	Message    string `json:"Message"`
	Identifier int    `json:"Identifier"`
	Stacktrace string `json:"Stacktrace"`
}

func (tc *RCONWSConnection) Write(p []byte) (n int, err error) {
	if !tc.ready {
		time.Sleep(1 * time.Second)
		if !tc.ready {
			return 0, errors.New("rconws not available")
		}
	}
	if tc.connection != nil {
		tc.identifierLocker.Lock()
		defer tc.identifierLocker.Unlock()
		tc.identifier++
		return len(p), tc.connection.WriteJSON(&rconwsMessage{
			Message:    string(p),
			Identifier: tc.identifier,
		})
	}
	return 0, errors.New("rconws not available")
}

func (tc *RCONWSConnection) Start() {
	tc.Reconnect = true
	if tc.IP == "" {
		tc.IP = "127.0.0.1"
	}

	go tc.reconnector()
}

func (tc *RCONWSConnection) Close() error {
	tc.Reconnect = false
	if tc.connection == nil {
		return nil
	}
	return tc.connection.Close()
}

func (tc *RCONWSConnection) reconnector() {
	init := true
	for tc.Reconnect {
		tc.ready = false
		if !init {
			time.Sleep(5 * time.Second)
		} else {
			init = false
		}

		ipAddr := &net.TCPAddr{
			IP:   net.ParseIP(tc.IP),
			Port: cast.ToInt(tc.Port),
		}

		conn, _, err := websocket.DefaultDialer.Dial("ws://"+ipAddr.String()+"/"+tc.Password, nil)
		if err != nil {
			logging.Debug.Printf("Error waiting for RCON WS socket: %s", err.Error())
			continue
		}

		//wait a second for the prompt for passwords/other delays
		time.Sleep(1 * time.Second)

		tc.connection = conn
		tc.ready = true
		listening := true
		for listening {
			var data []byte
			_, data, err = conn.ReadMessage()
			if err != nil {
				listening = false
			} else if len(data) > 0 {
				tc.Logger.Printf("[RCON-WS] " + string(data))
			}
		}
	}
}