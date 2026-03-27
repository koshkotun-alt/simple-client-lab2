package client

import (
	"fmt"
	"net/http"
	"os"
	"simple-web-server/src/tracer"

	"github.com/gorilla/websocket"
	"encoding/json"
)

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

type Client struct {
	socket   *websocket.Conn
	send     chan []byte
	room     *Room
	username string
}

var upgrader = &websocket.Upgrader{
	WriteBufferSize: socketBufferSize,
	ReadBufferSize:  socketBufferSize,
	// Можно добавить разрешения, если нужно
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (c *Client) Read() {
	defer func() {
		c.room.Leave <- c
		c.socket.Close()
	}()

	for {
		_, message, err := c.socket.ReadMessage()
		if err != nil {
			break
		}

		// Формируем сообщение с именем пользователя
		data := map[string]string{
			"username": c.username,
			"message":  string(message),
		}
		jsonMsg, err := json.Marshal(data)
		if err != nil {
			continue
		}

		c.room.Forward <- jsonMsg
	}
	// После разрыва соединения отправляем клиента в канал ухода
	c.room.Leave <- c
}

func (c *Client) Write() {
	for msg := range c.send {
		if err := c.socket.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
	c.socket.Close()
}

type Room struct {
	Forward chan []byte
	Join    chan *Client
	Leave   chan *Client
	Clients map[*Client]bool
	Tracer  tracer.Tracer
}

func NewRoom() *Room {
	return &Room{
		Forward: make(chan []byte),
		Join:    make(chan *Client),
		Leave:   make(chan *Client),
		Clients: make(map[*Client]bool),
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.Join:
			r.Clients[client] = true
			r.Tracer.Trace("New client joined")
		case client := <-r.Leave:
			delete(r.Clients, client)
			r.Tracer.Trace("Client left")
		case msg := <-r.Forward:
			for client := range r.Clients {
				select {
				case client.send <- msg:
					r.Tracer.Trace(" -- sent to client")
				default:
					delete(r.Clients, client)
					close(client.send)
					r.Tracer.Trace(" -- failed to send, cleaned up client")
				}
			}
		}
	}
}

// Обработка входящего HTTP-запроса для WebSocket
func (r *Room) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	// Получение параметров
	username := req.URL.Query().Get("username")

	if username == "" {
		username = "Аноним"
	}

	// Можно добавить проверку токена, если нужно
	// например:
	// if !isValidToken(token) { /* отказ */ }

	socket, err := upgrader.Upgrade(writer, req, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ServeHTTP: %v", err)
		return
	}

	client := &Client{
		socket:   socket,
		send:     make(chan []byte, messageBufferSize),
		room:     r,
		username: username,
	}

	go client.Write()
	go client.Read()

	// Добавляем клиента в комнату
	r.Join <- client
}
