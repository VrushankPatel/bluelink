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
			PaddingRight(1)

	participantsHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#2774AE")).
				PaddingLeft(1).
				PaddingRight(1)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2774AE")).
			PaddingLeft(1)

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")).
			Width(10)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")).
			Italic(true).
			PaddingLeft(1)

	mainChatStyle = lipgloss.NewStyle().
			Width(85)

	participantsPanelStyle = lipgloss.NewStyle().
				Width(30).
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
	width        int
	height       int
}

// NewChatUI creates a new chat UI
func NewChatUI(roomID, username, userID, userColor string, fb *firebase.Client) (*ChatModel, error) {
	// Get terminal size - default to reasonable values if can't detect
	width := 120 // default width
	height := 40 // default height

	// Calculate dimensions
	mainWidth := int(float64(width) * 0.75) // Main chat takes 75% of width
	viewportHeight := height - 4            // Reserve space for header and input

	// Set up viewport for messages
	msgViewport := viewport.New(mainWidth, viewportHeight)
	msgViewport.Style = borderStyle

	// Set up textarea for input
	ta := textarea.New()
	ta.Placeholder = "Type a message and press Enter to send..."
	ta.CharLimit = 1000
	ta.SetWidth(mainWidth - 2) // Account for borders
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
		width:        width,
		height:       height,
	}

	// Join the room silently (no debug output)
	if err := fb.JoinRoom(roomID, userID, username, userColor); err != nil {
		return nil, fmt.Errorf("failed to join room: %w", err)
	}

	// Load initial messages silently
	initialMsgs, err := fb.GetInitialMessages(roomID)
	if err != nil {
		m.messages = []firebase.Message{}
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
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update dimensions when window is resized
		m.width = msg.Width
		m.height = msg.Height

		// Recalculate viewport and input dimensions
		mainWidth := int(float64(m.width) * 0.75)
		viewportHeight := m.height - 4

		m.msgViewport.Width = mainWidth
		m.msgViewport.Height = viewportHeight
		m.inputArea.SetWidth(mainWidth - 2)

		// Make sure viewport content is updated with new dimensions
		m.updateMessages()

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
					m.updateMessages()
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
				// Send regular message silently
				_ = m.firebase.SendMessage(m.roomID, m.userID, m.username, m.userColor, messageText)
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
		// New message received silently
		m.messages = append(m.messages, msg)
		m.updateMessages()
		cmds = append(cmds, m.waitForMessages())

	case map[string]firebase.Participant:
		// Participant list updated silently
		m.participants = msg
		cmds = append(cmds, m.waitForParticipants())
	}

	// Handle viewport and textarea updates
	var cmd tea.Cmd
	m.msgViewport, cmd = m.msgViewport.Update(msg)
	cmds = append(cmds, cmd)
	m.inputArea, cmd = m.inputArea.Update(msg)
	cmds = append(cmds, cmd)

	// Keep listening for messages and participants
	cmds = append(cmds, m.waitForMessages(), m.waitForParticipants(), m.keepAlive())

	return m, tea.Batch(cmds...)
}

// updateMessages refreshes the message viewport
func (m *ChatModel) updateMessages() {
	var sb strings.Builder

	for _, msg := range m.messages {
		// Format timestamp
		timestamp := time.Unix(msg.Timestamp, 0).Format("15:04:05")
		timeStr := timestampStyle.Render(timestamp)

		// Format sender with color
		senderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(msg.Color))
		senderStr := senderStyle.Render(msg.Sender)

		// Combine all parts
		messageText := fmt.Sprintf("%s %s: %s\n", timeStr, senderStr, msg.Text)
		sb.WriteString(messageText)
	}

	// Update content and ensure we're at the bottom to see new messages
	m.msgViewport.SetContent(sb.String())
	m.msgViewport.GotoBottom()
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

	// Calculate dimensions based on current terminal size
	mainWidth := int(float64(m.width) * 0.75)
	sideWidth := m.width - mainWidth - 2

	// Update styles with current dimensions
	roomHeaderStyle = roomHeaderStyle.Width(m.width)
	participantsHeaderStyle = participantsHeaderStyle.Width(sideWidth)
	inputStyle = inputStyle.Width(mainWidth)

	// Format header with room ID
	header := roomHeaderStyle.Render(fmt.Sprintf("BlueLink Chat - Room: %s", m.roomID))

	// Format participants list
	participants := lipgloss.NewStyle().
		Width(sideWidth).
		PaddingLeft(1).
		Render(m.formatParticipants())

	// Format main chat area
	mainChat := lipgloss.NewStyle().
		Width(mainWidth).
		Render(m.msgViewport.View())

	// Layout the UI with messages on left (75%), participants on right (25%)
	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		mainChat,
		participants,
	)

	// Format input area
	input := inputStyle.Render(m.inputArea.View())

	// Format help text if open
	help := m.formatHelp()

	// Combine all elements with proper spacing
	return fmt.Sprintf("%s\n%s\n%s\n%s", header, mainContent, input, help)
}

// Run starts the Bubble Tea TUI program
func (m *ChatModel) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
