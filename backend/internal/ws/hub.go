package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"skribbl-dare-backend/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow extension and skribbl origins
		return true
	},
}

type EventType string

const (
	EventPlayerJoined     EventType = "player_joined"
	EventPlayerLeft       EventType = "player_left"
	EventDareCompleted    EventType = "dare_completed"
	EventRoomPlayersSync  EventType = "room_players_sync"
	EventActiveDareUpdate EventType = "active_dare_update"
)

type WSMessage struct {
	Type     EventType       `json:"type"`
	RoomCode string          `json:"room_code"`
	Payload  json.RawMessage `json:"payload"`
}

type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	Send      chan []byte
	DiscordID string
	RoomCode  string
	Nickname  string
	Tier      models.Tier
	Points    int
	AvatarURL string
}

type Hub struct {
	clients    map[*Client]bool
	rooms      map[string]map[*Client]bool
	broadcast  chan WSMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan WSMessage, 100),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.RoomCode != "" {
				if h.rooms[client.RoomCode] == nil {
					h.rooms[client.RoomCode] = make(map[*Client]bool)
				}
				h.rooms[client.RoomCode][client] = true
			}
			h.mu.Unlock()

			// Broadcast room players sync if room assigned
			if client.RoomCode != "" {
				h.broadcastRoomSync(client.RoomCode)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
				if client.RoomCode != "" && h.rooms[client.RoomCode] != nil {
					delete(h.rooms[client.RoomCode], client)
					if len(h.rooms[client.RoomCode]) == 0 {
						delete(h.rooms, client.RoomCode)
					}
				}
			}
			h.mu.Unlock()

			if client.RoomCode != "" {
				h.broadcastRoomSync(client.RoomCode)
			}

		case msg := <-h.broadcast:
			h.mu.RLock()
			roomClients, exists := h.rooms[msg.RoomCode]
			if exists {
				data, err := json.Marshal(msg)
				if err == nil {
					for c := range roomClients {
						select {
						case c.Send <- data:
						default:
							close(c.Send)
							delete(h.clients, c)
							delete(roomClients, c)
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) broadcastRoomSync(roomCode string) {
	h.mu.RLock()
	clients, ok := h.rooms[roomCode]
	if !ok {
		h.mu.RUnlock()
		return
	}

	var players []models.PlayerBadgeInfo
	for c := range clients {
		players = append(players, models.PlayerBadgeInfo{
			DiscordID:      c.DiscordID,
			Username:       c.DiscordID,
			InGameNickname: c.Nickname,
			Points:         c.Points,
			Tier:           c.Tier,
			AvatarURL:      c.AvatarURL,
		})
	}
	h.mu.RUnlock()

	payload, err := json.Marshal(players)
	if err == nil {
		h.BroadcastToRoom(roomCode, EventRoomPlayersSync, payload)
	}
}

func (h *Hub) BroadcastToRoom(roomCode string, eventType EventType, payload []byte) {
	h.broadcast <- WSMessage{
		Type:     eventType,
		RoomCode: roomCode,
		Payload:  payload,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break
		}

		var incoming WSMessage
		if err := json.Unmarshal(message, &incoming); err == nil {
			if incoming.RoomCode != "" && incoming.RoomCode != c.RoomCode {
				// Player switched room
				c.Hub.mu.Lock()
				if c.RoomCode != "" && c.Hub.rooms[c.RoomCode] != nil {
					delete(c.Hub.rooms[c.RoomCode], c)
				}
				c.RoomCode = incoming.RoomCode
				if c.Hub.rooms[c.RoomCode] == nil {
					c.Hub.rooms[c.RoomCode] = make(map[*Client]bool)
				}
				c.Hub.rooms[c.RoomCode][c] = true
				c.Hub.mu.Unlock()

				c.Hub.broadcastRoomSync(c.RoomCode)
			}
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, discordID, roomCode, nickname string, tier models.Tier, points int, avatarURL string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &Client{
		Hub:       hub,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		DiscordID: discordID,
		RoomCode:  roomCode,
		Nickname:  nickname,
		Tier:      tier,
		Points:    points,
		AvatarURL: avatarURL,
	}

	client.Hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}
