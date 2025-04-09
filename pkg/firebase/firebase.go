package firebase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"google.golang.org/api/option"
)

// Message represents a chat message
type Message struct {
	Sender    string `json:"sender"`
	SenderID  string `json:"senderId"`
	Color     string `json:"color"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}

// Participant represents a chat room participant
type Participant struct {
	Name       string `json:"name"`
	Color      string `json:"color"`
	LastActive int64  `json:"lastActive"`
}

// Client handles Firebase interactions
type Client struct {
	app *firebase.App
	db  *db.Client
	ctx context.Context
}

// NewClient creates a new Firebase client
func NewClient() (*Client, error) {
	ctx := context.Background()

	// Look for Firebase credentials
	credFile := os.Getenv("FIREBASE_CREDENTIALS")
	if credFile == "" {
		// For development, try to find in current directory
		credFile = "firebase-credentials.json"
		if _, err := os.Stat(credFile); os.IsNotExist(err) {
			return nil, errors.New("Firebase credentials not found. Set FIREBASE_CREDENTIALS environment variable to point to your credentials file")
		}
	}

	// Initialize Firebase app
	opt := option.WithCredentialsFile(credFile)
	config := &firebase.Config{
		DatabaseURL: os.Getenv("FIREBASE_DATABASE_URL"),
	}

	app, err := firebase.NewApp(ctx, config, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing Firebase app: %w", err)
	}

	// Get Firebase Realtime Database client
	db, err := app.Database(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing Firebase database: %w", err)
	}

	return &Client{
		app: app,
		db:  db,
		ctx: ctx,
	}, nil
}

// CreateRoom creates a new chat room and returns the room ID
func (c *Client) CreateRoom(userID, username, color string) (string, error) {
	// Generate a random 8-digit room ID
	rand.Seed(time.Now().UnixNano())
	roomID := strconv.Itoa(10000000 + rand.Intn(90000000))

	// Create room with initial participant
	roomRef := c.db.NewRef("rooms").Child(roomID)

	// Add creator as first participant
	participant := Participant{
		Name:       username,
		Color:      color,
		LastActive: time.Now().Unix(),
	}

	err := roomRef.Child("participants").Child(userID).Set(c.ctx, participant)
	if err != nil {
		return "", fmt.Errorf("failed to create room: %w", err)
	}

	// Add system message
	welcomeMsg := Message{
		Sender:    "System",
		SenderID:  "system",
		Color:     "#888888",
		Text:      fmt.Sprintf("%s created the room", username),
		Timestamp: time.Now().Unix(),
	}

	_, err = roomRef.Child("messages").Push(c.ctx, welcomeMsg)
	if err != nil {
		return "", fmt.Errorf("failed to add system message: %w", err)
	}

	return roomID, nil
}

// JoinRoom adds the user to an existing room
func (c *Client) JoinRoom(roomID, userID, username, color string) error {
	// Check if room exists
	roomRef := c.db.NewRef("rooms").Child(roomID)
	var roomData map[string]interface{}
	if err := roomRef.Get(c.ctx, &roomData); err != nil || roomData == nil {
		return errors.New("room does not exist")
	}

	// Add user to participants
	participant := Participant{
		Name:       username,
		Color:      color,
		LastActive: time.Now().Unix(),
	}

	if err := roomRef.Child("participants").Child(userID).Set(c.ctx, participant); err != nil {
		return fmt.Errorf("failed to join room: %w", err)
	}

	// Add system message
	joinMsg := Message{
		Sender:    "System",
		SenderID:  "system",
		Color:     "#888888",
		Text:      fmt.Sprintf("%s joined the room", username),
		Timestamp: time.Now().Unix(),
	}

	_, err := roomRef.Child("messages").Push(c.ctx, joinMsg)
	if err != nil {
		return fmt.Errorf("failed to add system message: %w", err)
	}

	return nil
}

// LeaveRoom removes the user from a room
func (c *Client) LeaveRoom(roomID, userID string) error {
	// Get username for the leave message
	var participant Participant
	participantRef := c.db.NewRef("rooms").Child(roomID).Child("participants").Child(userID)
	if err := participantRef.Get(c.ctx, &participant); err != nil {
		return fmt.Errorf("failed to get participant data: %w", err)
	}

	// Add system message about leaving
	leaveMsg := Message{
		Sender:    "System",
		SenderID:  "system",
		Color:     "#888888",
		Text:      fmt.Sprintf("%s left the room", participant.Name),
		Timestamp: time.Now().Unix(),
	}

	_, err := c.db.NewRef("rooms").Child(roomID).Child("messages").Push(c.ctx, leaveMsg)
	if err != nil {
		return fmt.Errorf("failed to add system message: %w", err)
	}

	// Remove user from participants
	if err := participantRef.Delete(c.ctx); err != nil {
		return fmt.Errorf("failed to leave room: %w", err)
	}

	return nil
}

// SendMessage sends a message to the chat room
func (c *Client) SendMessage(roomID, userID, username, color, text string) error {
	// Encrypt the message text
	encryptedText, err := encrypt(text, roomID)
	if err != nil {
		return fmt.Errorf("failed to encrypt message: %w", err)
	}

	message := Message{
		Sender:    username,
		SenderID:  userID,
		Color:     color,
		Text:      encryptedText, // Store encrypted text
		Timestamp: time.Now().Unix(),
	}

	_, err = c.db.NewRef("rooms").Child(roomID).Child("messages").Push(c.ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Update user's last active timestamp
	lastActive := map[string]interface{}{
		"lastActive": time.Now().Unix(),
	}

	err = c.db.NewRef("rooms").Child(roomID).Child("participants").Child(userID).Update(c.ctx, lastActive)
	if err != nil {
		return fmt.Errorf("failed to update activity: %w", err)
	}

	return nil
}

// decryptMessage decrypts a message's text field
func (c *Client) decryptMessage(msg Message, roomID string) Message {
	// Don't decrypt system messages
	if msg.SenderID == "system" {
		return msg
	}

	decryptedText, err := decrypt(msg.Text, roomID)
	if err != nil {
		// If decryption fails, return a message indicating the error
		msg.Text = "[Failed to decrypt message]"
		return msg
	}

	msg.Text = decryptedText
	return msg
}

// ListenForMessages sets up a polling mechanism for messages
func (c *Client) ListenForMessages(roomID string, msgChan chan Message) {
	go func() {
		var lastTimestamp int64 = 0
		var processedMsgIDs = make(map[string]bool)

		// Get all messages initially to find the most recent timestamp
		messagesRef := c.db.NewRef("rooms").Child(roomID).Child("messages")
		var initialMessages map[string]Message
		if err := messagesRef.Get(c.ctx, &initialMessages); err == nil {
			// Process all initial messages to find the latest timestamp
			for msgID, msg := range initialMessages {
				if msg.Timestamp > lastTimestamp {
					lastTimestamp = msg.Timestamp
				}
				// Mark this message as processed
				processedMsgIDs[msgID] = true
			}
		}

		for {
			// Get all messages and filter in memory
			var messages map[string]Message
			if err := messagesRef.Get(c.ctx, &messages); err == nil {
				if len(messages) > 0 {
					// Convert map to slice for sorting
					var orderedMsgs []Message
					var maxTimestamp int64 = lastTimestamp

					// Process messages and track message IDs to avoid duplicates
					for msgID, msg := range messages {
						// Skip already processed messages
						if processedMsgIDs[msgID] {
							continue
						}

						// Skip messages with timestamps older than our last processed timestamp
						if msg.Timestamp <= lastTimestamp && lastTimestamp > 0 {
							continue
						}

						// Track this message as processed
						processedMsgIDs[msgID] = true

						// Decrypt the message before adding it
						msg = c.decryptMessage(msg, roomID)
						orderedMsgs = append(orderedMsgs, msg)

						// Update max timestamp
						if msg.Timestamp > maxTimestamp {
							maxTimestamp = msg.Timestamp
						}
					}

					// Sort messages by timestamp
					sort.Slice(orderedMsgs, func(i, j int) bool {
						return orderedMsgs[i].Timestamp < orderedMsgs[j].Timestamp
					})

					// Send each new message to the channel
					for _, msg := range orderedMsgs {
						msgChan <- msg
					}

					// Update lastTimestamp for next query
					lastTimestamp = maxTimestamp

					// Periodically clean up the processedMsgIDs map to prevent memory leaks
					if len(processedMsgIDs) > 1000 {
						processedMsgIDs = make(map[string]bool)
					}
				}
			}

			// Sleep before polling again
			time.Sleep(500 * time.Millisecond)
		}
	}()
}

// ListenForParticipants sets up a polling mechanism for participants
func (c *Client) ListenForParticipants(roomID string, partChan chan map[string]Participant) {
	go func() {
		var lastUpdate int64 = 0
		for {
			// Query participants
			participantsRef := c.db.NewRef("rooms").Child(roomID).Child("participants")
			var participants map[string]Participant
			if err := participantsRef.Get(c.ctx, &participants); err == nil {
				// Check if anything has changed
				var maxTimestamp int64 = 0
				for _, p := range participants {
					if p.LastActive > maxTimestamp {
						maxTimestamp = p.LastActive
					}
				}

				// If we have new activity, send update
				if maxTimestamp > lastUpdate || lastUpdate == 0 {
					lastUpdate = maxTimestamp
					partChan <- participants
				}
			}

			// Sleep before polling again
			time.Sleep(1 * time.Second)
		}
	}()
}

// UpdateActivity updates the user's last active timestamp
func (c *Client) UpdateActivity(roomID, userID string) error {
	lastActive := map[string]interface{}{
		"lastActive": time.Now().Unix(),
	}

	err := c.db.NewRef("rooms").Child(roomID).Child("participants").Child(userID).Update(c.ctx, lastActive)
	if err != nil {
		return fmt.Errorf("failed to update activity: %w", err)
	}

	return nil
}

// CheckRoomExists checks if a room with the given ID exists
func (c *Client) CheckRoomExists(roomID string) (bool, error) {
	roomRef := c.db.NewRef("rooms").Child(roomID)
	var roomData map[string]interface{}
	if err := roomRef.Get(c.ctx, &roomData); err != nil {
		return false, fmt.Errorf("failed to check room: %w", err)
	}
	return roomData != nil, nil
}

// CreateRoomWithID creates a new chat room with the specified ID
func (c *Client) CreateRoomWithID(roomID, userID, username, color string) error {
	// Create room with initial participant
	roomRef := c.db.NewRef("rooms").Child(roomID)

	// Add creator as first participant
	participant := Participant{
		Name:       username,
		Color:      color,
		LastActive: time.Now().Unix(),
	}

	err := roomRef.Child("participants").Child(userID).Set(c.ctx, participant)
	if err != nil {
		return fmt.Errorf("failed to create room: %w", err)
	}

	// Add system message
	welcomeMsg := Message{
		Sender:    "System",
		SenderID:  "system",
		Color:     "#888888",
		Text:      fmt.Sprintf("%s created the room", username),
		Timestamp: time.Now().Unix(),
	}

	_, err = roomRef.Child("messages").Push(c.ctx, welcomeMsg)
	if err != nil {
		return fmt.Errorf("failed to add system message: %w", err)
	}

	return nil
}

// GetInitialMessages gets the most recent messages from a room
func (c *Client) GetInitialMessages(roomID string) ([]Message, error) {
	messagesRef := c.db.NewRef("rooms").Child(roomID).Child("messages")

	// Instead of querying by timestamp which requires an index,
	// just get all messages and sort them in memory
	var messagesMap map[string]Message
	if err := messagesRef.Get(c.ctx, &messagesMap); err != nil {
		return nil, fmt.Errorf("failed to get initial messages: %w", err)
	}

	// If no messages found, return empty slice instead of nil
	if len(messagesMap) == 0 {
		return []Message{}, nil
	}

	var messages []Message
	for _, msg := range messagesMap {
		// Decrypt message before adding it to the list
		msg = c.decryptMessage(msg, roomID)
		messages = append(messages, msg)
	}

	// Sort messages by timestamp
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp < messages[j].Timestamp
	})

	return messages, nil
}
