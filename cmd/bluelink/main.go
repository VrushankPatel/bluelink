package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/vrushank/bluelink/pkg/config"
	"github.com/vrushank/bluelink/pkg/firebase"
	"github.com/vrushank/bluelink/pkg/ui"
)

func main() {
	// Parse command line arguments
	flag.Parse()
	args := flag.Args()

	// Load or create user configuration
	cfg, err := config.LoadOrCreate()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize Firebase connection
	fb, err := firebase.NewClient()
	if err != nil {
		fmt.Printf("Error connecting to Firebase: %v\n", err)
		os.Exit(1)
	}

	// Determine room ID (create new or join existing)
	roomID := ""
	if len(args) > 0 {
		// Try to join existing room
		roomID = args[0]

		// First check if the room exists
		roomExists, err := fb.CheckRoomExists(roomID)
		if err != nil {
			fmt.Printf("Error checking room: %v\n", err)
			os.Exit(1)
		}

		// If room doesn't exist, ask user if they want to create it
		if !roomExists {
			fmt.Printf("Room %s does not exist. Create it? (y/N): ", roomID)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "y" || response == "yes" {
				// Create the room with the specified ID
				err = fb.CreateRoomWithID(roomID, cfg.UserID, cfg.Username, cfg.Color)
				if err != nil {
					fmt.Printf("Error creating room: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Room %s created successfully.\n", roomID)
			} else {
				fmt.Println("Exiting.")
				os.Exit(0)
			}
		}
	} else {
		// Create new room with random ID
		roomID, err = fb.CreateRoom(cfg.UserID, cfg.Username, cfg.Color)
		if err != nil {
			fmt.Printf("Error creating new room: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Connecting to room: %s\n", roomID)

	// Initialize UI
	chatUI, err := ui.NewChatUI(roomID, cfg.Username, cfg.UserID, cfg.Color, fb)
	if err != nil {
		fmt.Printf("Error initializing UI: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nDisconnecting from chat...")
		fb.LeaveRoom(roomID, cfg.UserID)
		os.Exit(0)
	}()

	// Run the UI
	if err := chatUI.Run(); err != nil {
		fmt.Printf("Error running UI: %v\n", err)
		os.Exit(1)
	}
}
