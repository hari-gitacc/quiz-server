package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"quiz-system/internal/models"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Message represents the standard message format exchanged over WebSocket.
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// upgrader configures the WebSocket connection upgrade.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins. Adjust this in production!
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type UserInfo struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Hub struct in hub.go
type Hub struct {
	clients       map[*Client]bool
	quizRooms     map[string]map[*Client]bool
	participants  map[string]int
	register      chan *Client
	unregister    chan *Client
	mu            sync.RWMutex
	quizService   QuizServiceInterface // Existing interface
	clientsByUser map[uint]*Client     // For non-host participants
	hosts         map[uint]*Client     // NEW: for hosts (quiz creators)
}

func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		quizRooms:     make(map[string]map[*Client]bool),
		participants:  make(map[string]int),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		clientsByUser: make(map[uint]*Client),
		hosts:         make(map[uint]*Client),
	}
}

// Add method to register services
// func (h *Hub) RegisterService(name string, service interface{}) {
//     h.services[name] = service
// }

func (h *Hub) SetQuizService(service QuizServiceInterface) {
	h.quizService = service
}

type QuizServiceInterface interface {
    HandleNextQuestion(quizCode string, currentIndex int) error
    GetQuizByCode(quizCode string) (*models.Quiz, error)
    RemoveParticipant(quizCode string, userID uint) error
    JoinQuiz(quizCode string, userID uint) error
    HandleNextQuestionForUser(userID uint, quizCode string, nextIndex int) error
    GetLeaderboard(quizCode string) ([]models.LeaderboardEntry, error)
    StartQuiz(quizCode string, userID uint) error
}

func (h *Hub) checkIfHost(quizCode string, userID uint) (bool, error) {
	// Use your quiz service to retrieve quiz details.
	quiz, err := h.quizService.GetQuizByCode(quizCode)
	if err != nil {
		return false, err
	}
	return quiz.CreatorID == userID, nil
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	quizCode string
	user     *UserInfo
	isHost   bool          // NEW: indicates if this client is the host
	done     chan struct{} // New done channel to signal shutdown
}

func (h *Hub) BroadcastToQuiz(quizCode string, message []byte) {
	// Use RLock() for reading only
	h.mu.RLock()
	clients := h.quizRooms[quizCode]
	h.mu.RUnlock() // Release the lock immediately after reading

	log.Printf("BroadcastToQuiz: Starting broadcast to quiz %s", quizCode)

	if len(clients) == 0 {
		log.Printf("No clients found for quiz room: %s", quizCode)
		return
	}

	log.Printf("Found %d clients in room %s", len(clients), quizCode)

	// Create a copy of clients to avoid concurrent map access
	clientsCopy := make([]*Client, 0, len(clients))
	for client := range clients {
		if client != nil {
			clientsCopy = append(clientsCopy, client)
		}
	}

	// Send messages via each client's send channel
	for _, client := range clientsCopy {
		func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic while sending message to client %p: %v", c, r)
					h.unregister <- c
				}
			}()

			// Instead of writing directly to the connection, send the message through the channel.
			select {
			case c.send <- message:
				log.Printf("Queued message for client %p", c)
			default:
				log.Printf("Send channel full for client %p; unregistering client", c)
				h.unregister <- c
			}
		}(client)
	}

	log.Printf("Completed broadcasting message to all clients in room %s", quizCode)
}

// BroadcastMessage marshals the message and then broadcasts it.
func (h *Hub) BroadcastMessage(quizCode string, messageType string, data interface{}) {
	log.Printf("BroadcastMessage called for quiz %s with type %s", quizCode, messageType)

	msg := Message{
		Type: messageType,
		Data: data,
	}

	messageBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	log.Printf("Marshaled message: %s", string(messageBytes))
	h.BroadcastToQuiz(quizCode, messageBytes)
}

func (h *Hub) SendMessageToUser(userID uint, messageType string, data interface{}) {
	h.mu.RLock()
	client, exists := h.clientsByUser[userID] // Now this field exists
	h.mu.RUnlock()
	if !exists || client == nil {
		log.Printf("No active client found for user %d", userID)
		return
	}

	msg := Message{
		Type: messageType,
		Data: data,
	}
	messageBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message for user %d: %v", userID, err)
		return
	}

	// Send the message bytes through the client's send channel.
	select {
	case client.send <- messageBytes:
		log.Printf("Queued message for user %d", userID)
	default:
		log.Printf("Send channel full for user %d; unregistering client", userID)
		h.unregister <- client
	}
}

func (h *Hub) RegisterClient(client *Client, quizCode string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Printf("Registering client %p for quiz %s", client, quizCode)

	// Initialize room if it doesn't exist.
	if _, ok := h.quizRooms[quizCode]; !ok {
		h.quizRooms[quizCode] = make(map[*Client]bool)
		log.Printf("Created new room for quiz %s", quizCode)
	}

	// Add client to room and to the overall clients map.
	h.quizRooms[quizCode][client] = true
	h.clients[client] = true

	// Recalculate participant count excluding hosts.
	count := 0
	for c := range h.quizRooms[quizCode] {
		if c.user != nil {
			if _, isHost := h.hosts[c.user.UserID]; !isHost {
				count++
			}
		}
	}
	h.participants[quizCode] = count
	log.Printf("Client %p registered for quiz %s. Participant count (excluding hosts): %d", client, quizCode, count)

	// Broadcast updated participant count.
	go h.BroadcastMessage(quizCode, "participant_update", map[string]interface{}{
		"count": count,
	})
	// Also broadcast the updated participant list.
    go h.SendParticipantList(quizCode)
}

func (h *Hub) UnregisterClient(client *Client) {
    h.mu.Lock()
    quizCode := client.quizCode
    
    // Check if client exists in the quiz room
    if room, exists := h.quizRooms[quizCode]; exists {
        delete(room, client)
        log.Printf("Client %p removed from quiz %s", client, quizCode)
        
        // Remove from global maps
        delete(h.clients, client)
        if client.user != nil {
            delete(h.clientsByUser, client.user.UserID)
            delete(h.hosts, client.user.UserID)
        }
        
        // Recalculate participant count
        count := 0
        participants := make([]UserInfo, 0)
        var hostInfo *UserInfo
        
        for c := range room {
            if c.user != nil {
                if c.isHost {
                    hostInfo = c.user
                } else {
                    count++
                    participants = append(participants, *c.user)
                }
            }
        }
        
        h.participants[quizCode] = count
        
        // Update database if participant leaves
        if !client.isHost && client.user != nil {
            go h.removeParticipantFromDB(quizCode, client.user.UserID)
        }
        
        // Close client channels
        close(client.send)
        close(client.done)
        
        // Release the lock before broadcasting
        h.mu.Unlock()
        
        // Broadcast updated participant information
        h.BroadcastMessage(quizCode, "participant_list", map[string]interface{}{
            "participants": participants,
            "count":       count,
            "host":        hostInfo,
        })
        
        h.BroadcastMessage(quizCode, "participant_update", map[string]interface{}{
            "count": count,
        })
        
        log.Printf("Participant updates broadcast for quiz %s", quizCode)
    } else {
        h.mu.Unlock()
    }
}

// Helper method to remove participant from database
func (h *Hub) removeParticipantFromDB(quizCode string, userID uint) {
    if h.quizService != nil {
        if err := h.quizService.RemoveParticipant(quizCode, userID); err != nil {
            log.Printf("Error removing participant from database: %v", err)
        }
    }
}

func (h *Hub) SendParticipantList(quizCode string) {
    h.mu.RLock()
    room, exists := h.quizRooms[quizCode]
    if !exists {
        h.mu.RUnlock()
        return
    }

    participants := make([]UserInfo, 0)
    var hostInfo *UserInfo

    for client := range room {
        if client.user != nil {
            if client.isHost {
                hostInfo = client.user
            } else {
                participants = append(participants, *client.user)
            }
        }
    }
    h.mu.RUnlock()

    // Send both participant list and participant update
    h.BroadcastMessage(quizCode, "participant_list", map[string]interface{}{
        "participants": participants,
        "count":       len(participants),
        "host":        hostInfo,
    })

    h.BroadcastMessage(quizCode, "participant_update", map[string]interface{}{
        "count": len(participants),
    })
}


// Run listens on the register and unregister channels and updates the hub state accordingly.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.quizCode != "" {
				// Create the quiz room if it doesn't exist.
				if _, exists := h.quizRooms[client.quizCode]; !exists {
					h.quizRooms[client.quizCode] = make(map[*Client]bool)
					log.Printf("Created room for quiz %s", client.quizCode)
				}
				// Add the client to the room.
				h.quizRooms[client.quizCode][client] = true
				h.participants[client.quizCode]++
				log.Printf("Client %p added to quiz %s. Total: %d", client, client.quizCode, h.participants[client.quizCode])
				// Broadcast updated participant count.
				count := h.participants[client.quizCode]
				go h.BroadcastMessage(client.quizCode, "participant_update", map[string]interface{}{
					"count": count,
				})
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				if client.quizCode != "" {
					if room, exists := h.quizRooms[client.quizCode]; exists {
						delete(room, client)
						h.participants[client.quizCode]--
						log.Printf("Client %p left quiz %s. Remaining: %d", client, client.quizCode, h.participants[client.quizCode])
						count := h.participants[client.quizCode]
						go h.BroadcastMessage(client.quizCode, "participant_update", map[string]interface{}{
							"count": count,
						})
					}
				}
				delete(h.clients, client)
				close(client.send)
				close(client.done)
			}
			h.mu.Unlock()
		}
	}
}

// NewClient creates a new Client instance.
func NewClient(hub *Hub, conn *websocket.Conn, quizCode string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		quizCode: quizCode,
		done:     make(chan struct{}),
	}
}

// HandleWebSocket upgrades the HTTP connection to a WebSocket and registers the client.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	quizCode := vars["quizCode"]
	if quizCode == "" {
		http.Error(w, "Missing quiz code", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := NewClient(h, conn, quizCode)
	log.Printf("Created new WebSocket client %p for quiz %s", client, quizCode)

	h.RegisterClient(client, quizCode)

	// Start the pumps in separate goroutines
	go client.writePump()
	go client.readPump()
}

// readPump continuously reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Unexpected close: %v", err)
			}
			break
		}
		log.Printf("Received from client %p: %s", c, string(message))
		c.handleMessage(message)
	}
}

func (c *Client) handleMessage(message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	log.Printf("Client %p handling message type: %s", c, msg.Type)

	switch msg.Type {
	case "join_quiz":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			if user, ok := data["user"].(map[string]interface{}); ok {
				c.user = &UserInfo{
					UserID:   uint(user["userId"].(float64)),
					Username: user["username"].(string),
					Email: func() string {
						if email, ok := user["email"].(string); ok {
							return email
						}
						return ""
					}(),
				}
				log.Printf("User joined: %+v", c.user)

				// Determine host status using quiz service
				isHost, err := c.hub.checkIfHost(c.quizCode, c.user.UserID)
				if err != nil {
					log.Printf("Error checking host status for quiz %s: %v", c.quizCode, err)
					isHost = false // default to non-host on error
				}
				c.isHost = isHost

				c.hub.mu.Lock()
				if isHost {
					// Host: add to a dedicated hosts map if desired (or simply mark the client)
					if c.hub.hosts == nil {
						c.hub.hosts = make(map[uint]*Client)
					}
					c.hub.hosts[c.user.UserID] = c
					log.Printf("User %d identified as host; will not receive participant events.", c.user.UserID)
				} else {
					// Regular participant: add to clientsByUser map.
					c.hub.clientsByUser[c.user.UserID] = c
				}
				c.hub.mu.Unlock()
				go c.hub.SendParticipantList(c.quizCode)

			}
		}

	case "start_quiz":
		log.Printf("Quiz start message received for quiz %s", c.quizCode)

	case "answer_submitted":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			quizCode := data["quizCode"].(string)
			questionId := uint(data["questionId"].(float64))
			answer := data["answer"].(string)
			userId := uint(data["userId"].(float64))

			log.Printf("Answer submitted for quiz %s: user %d, question %d, answer: %s",
				quizCode, userId, questionId, answer)

			// Broadcast answer submission to all participants (both hosts and players may receive this if needed)
			c.hub.BroadcastMessage(quizCode, "answer_update", map[string]interface{}{
				"userId":     userId,
				"questionId": questionId,
			})
		}

	case "next_question":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			quizCode := data["quizCode"].(string)
			currentIndex := int(data["currentIndex"].(float64))

			if c.hub.quizService != nil {
				log.Printf("Processing next question request for quiz %s, current index: %d", quizCode, currentIndex)
				if err := c.hub.quizService.HandleNextQuestion(quizCode, currentIndex); err != nil {
					log.Printf("Error handling next question: %v", err)
				}
			} else {
				log.Printf("Quiz service not initialized")
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Error getting writer for client %p: %v", c, err)
				return
			}

			log.Printf("Writing message to client %p: %s", c, string(message))
			_, err = w.Write(message)
			if err != nil {
				log.Printf("Error writing message to client %p: %v", c, err)
				return
			}

			if err := w.Close(); err != nil {
				log.Printf("Error closing writer for client %p: %v", c, err)
				return
			}
			log.Printf("Successfully wrote message to client %p", c)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}
