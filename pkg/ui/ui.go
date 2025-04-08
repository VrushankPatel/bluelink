package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vrushank/bluelink/pkg/firebase"
)

// Styles for the TUI
var (
	borderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2774AE"))

	roomHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#2774AE")).
			PaddingLeft(1).
			PaddingRight(1).
			Width(100)

	participantsHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#2774AE")).
				PaddingLeft(1).
				PaddingRight(1)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2774AE")).
			PaddingLeft(1).
			Width(100)

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")).
			Width(10)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")).
			Italic(true).
			PaddingLeft(1)
)

// ChatModel is the main TUI model
type ChatModel struct {
	roomID       string
	userID       string
	username     string
	userColor    string
	msgViewport  viewport.Model
	inputArea    textarea.Model
	messages     []firebase.Message
	participants map[string]firebase.Participant
	firebase     *firebase.Client
	msgChan      chan firebase.Message
	partChan     chan map[string]firebase.Participant
	helpOpen     bool
	quitting     bool
}

// NewChatUI creates a new chat UI
func NewChatUI(roomID, username, userID, userColor string, fb *firebase.Client) (*ChatModel, error) {
	// Set up viewport for messages
	msgViewport := viewport.New(100, 30)
	msgViewport.Style = borderStyle

	// Set up textarea for input
	ta := textarea.New()
	ta.Placeholder = "Type a message and press Enter to send..."
	ta.CharLimit = 1000
	ta.SetWidth(98)
	ta.SetHeight(1)
	ta.Focus()

	// Set up channels for Firebase events
	msgChan := make(chan firebase.Message)
	partChan := make(chan map[string]firebase.Participant)

	m := &ChatModel{
		roomID:       roomID,
		username:     username,
		userID:       userID,
		userColor:    userColor,
		msgViewport:  msgViewport,
		inputArea:    ta,
		messages:     []firebase.Message{},
		firebase:     fb,
		msgChan:      msgChan,
		partChan:     partChan,
		participants: map[string]firebase.Participant{},
		helpOpen:     false,
	}

	// Join the room
	if err := fb.JoinRoom(roomID, userID, username, userColor); err != nil {
		return nil, fmt.Errorf("failed to join room: %w", err)
	}

	// Load initial messages
	initialMsgs, err := fb.GetInitialMessages(roomID)
	if err != nil {
		// Just log the error but continue - this isn't fatal
		fmt.Printf("Warning: Could not load initial messages: %v\n", err)
	} else {
		m.messages = initialMsgs
		m.updateMessages()
	}

	// Start listeners
	go fb.ListenForMessages(roomID, msgChan)
	go fb.ListenForParticipants(roomID, partChan)

	return m, nil
}

// Init initializes the chat UI
func (m ChatModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.waitForMessages(),
		m.waitForParticipants(),
		m.keepAlive(),
	)
}

// waitForMessages waits for new messages from Firebase
func (m ChatModel) waitForMessages() tea.Cmd {
	return func() tea.Msg {
		return <-m.msgChan
	}
}

// waitForParticipants waits for participant updates from Firebase
func (m ChatModel) waitForParticipants() tea.Cmd {
	return func() tea.Msg {
		return <-m.partChan
	}
}

// keepAlive periodically updates the user's activity status
func (m ChatModel) keepAlive() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		m.firebase.UpdateActivity(m.roomID, m.userID)
		return nil
	})
}

// Update handles UI updates
func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.firebase.LeaveRoom(m.roomID, m.userID)
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if !m.inputArea.Focused() {
				cmds = append(cmds, textarea.Blink)
				m.inputArea.Focus()
				break
			}

			messageText := strings.TrimSpace(m.inputArea.Value())
			if messageText == "" {
				break
			}

			// Handle commands
			if strings.HasPrefix(messageText, "/") {
				switch strings.ToLower(messageText) {
				case "/help":
					m.helpOpen = !m.helpOpen
				case "/clear":
					m.messages = []firebase.Message{}
					m.msgViewport.SetContent("")
				case "/exit":
					m.firebase.LeaveRoom(m.roomID, m.userID)
					m.quitting = true
					return m, tea.Quit
				default:
					// Add unknown command message
					m.messages = append(m.messages, firebase.Message{
						Sender:    "System",
						SenderID:  "system",
						Color:     "#888888",
						Text:      fmt.Sprintf("Unknown command: %s. Type /help for available commands.", messageText),
						Timestamp: time.Now().Unix(),
					})
				}
			} else {
				// Send regular message
				err := m.firebase.SendMessage(m.roomID, m.userID, m.username, m.userColor, messageText)
				if err != nil {
					m.messages = append(m.messages, firebase.Message{
						Sender:    "System",
						SenderID:  "system",
						Color:     "#888888",
						Text:      fmt.Sprintf("Error sending message: %v", err),
						Timestamp: time.Now().Unix(),
					})
				}
			}

			m.inputArea.Reset()
			m.updateMessages()

		case "tab":
			if m.inputArea.Focused() {
				m.inputArea.Blur()
			} else {
				m.inputArea.Focus()
				cmds = append(cmds, textarea.Blink)
			}
		}

	case firebase.Message:
		// New message received
		m.messages = append(m.messages, msg)
		m.updateMessages()
		cmds = append(cmds, m.waitForMessages())

	case map[string]firebase.Participant:
		// Participant list updated
		m.participants = msg
		m.updateMessages()
		cmds = append(cmds, m.waitForParticipants())
	}

	// Handle viewport update
	m.msgViewport, vpCmd = m.msgViewport.Update(msg)
	cmds = append(cmds, vpCmd)

	// Handle textarea update
	m.inputArea, tiCmd = m.inputArea.Update(msg)
	cmds = append(cmds, tiCmd)

	// Always ensure we keep listening for messages and participants
	// This is critical - we must always reattach these commands
	cmds = append(cmds, m.waitForMessages())
	cmds = append(cmds, m.waitForParticipants())

	// Keep updating activity status
	cmds = append(cmds, m.keepAlive())

	return m, tea.Batch(cmds...)
}

// updateMessages refreshes the message viewport
func (m *ChatModel) updateMessages() {
	var sb strings.Builder

	// Debug output to help diagnose issues
	fmt.Printf("Updating messages view with %d messages\n", len(m.messages))

	for i, msg := range m.messages {
		// Format timestamp
		timestamp := time.Unix(msg.Timestamp, 0).Format("15:04:05")
		timeStr := timestampStyle.Render(timestamp)

		// Format sender with color
		senderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(msg.Color))
		senderStr := senderStyle.Render(msg.Sender)

		// Combine all parts
		messageText := fmt.Sprintf("%s %s: %s\n", timeStr, senderStr, msg.Text)
		sb.WriteString(messageText)

		// Debug output for the first and last few messages
		if i < 3 || i >= len(m.messages)-3 {
			fmt.Printf("Message %d: %s %s: %s\n", i, timestamp, msg.Sender, msg.Text)
		}
	}

	// Update content and ensure we're at the bottom to see new messages
	content := sb.String()
	m.msgViewport.SetContent(content)
	m.msgViewport.GotoBottom()
	
	// Force viewport to update immediately
	m.msgViewport.Update(nil)
	
	// Force viewport to update immediately
	m.msgViewport.Update(nil)
}

// formatParticipants returns a formatted string of participants
func (m *ChatModel) formatParticipants() string {
	var sb strings.Builder
	sb.WriteString(participantsHeaderStyle.Render("Participants"))
	sb.WriteString("\n")

	for _, p := range m.participants {
		// Format timestamp as "5m ago"
		timeSince := time.Since(time.Unix(p.LastActive, 0)).Round(time.Minute)
		timeStr := fmt.Sprintf("%dm", int(timeSince.Minutes()))
		if timeSince.Minutes() < 1 {
			timeStr = "now"
		}

		// Format participant with color
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Color))
		nameStr := nameStyle.Render(p.Name)

		// Show active status
		active := "●"
		activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
		if timeSince.Minutes() > 5 {
			activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
		}
		if timeSince.Minutes() > 15 {
			activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
		}

		participantText := fmt.Sprintf("%s %s (%s)\n", activeStyle.Render(active), nameStr, timeStr)
		sb.WriteString(participantText)
	}

	return sb.String()
}

// formatHelp returns help text
func (m *ChatModel) formatHelp() string {
	if !m.helpOpen {
		return ""
	}

	return helpStyle.Render(`
Commands:
  /help   - Show this help
  /clear  - Clear the chat history
  /exit   - Leave the room
  
Navigation:
  Tab     - Toggle between chat and input
  Ctrl+C  - Exit the application
	`)
}

// View renders the UI
func (m ChatModel) View() string {
	if m.quitting {
		return "Disconnected from chat. Goodbye!\n"
	}

	// Format header with room ID
	header := roomHeaderStyle.Render(fmt.Sprintf("BlueLink Chat - Room: %s", m.roomID))

	// Format participants list
	participants := m.formatParticipants()

	// Layout the UI with messages on left, participants on right
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.msgViewport.View(),
		participants,
	)

	// Format input area
	input := inputStyle.Render(m.inputArea.View())

	// Format help text if open
	help := m.formatHelp()

	// Combine all elements
	return fmt.Sprintf("%s\n%s\n%s\n%s", header, mainContent, input, help)
}

// Run starts the Bubble Tea TUI program
func (m *ChatModel) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
