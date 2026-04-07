package io.github.vrushankpatel.bluelink;

import io.github.vrushankpatel.bluelink.config.UserConfig;
import io.github.vrushankpatel.bluelink.firebase.FirebaseClient;
import io.github.vrushankpatel.bluelink.firebase.Message;

import java.util.List;
import java.util.Scanner;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Manages the active chat session: polling for new messages and reading user input.
 */
public class ChatSession {

    private final String roomId;
    private final UserConfig config;
    private final FirebaseClient firebase;
    private final Scanner scanner;

    private final AtomicBoolean running = new AtomicBoolean(true);
    private final AtomicLong lastTimestamp = new AtomicLong(0);
    private final ScheduledExecutorService scheduler = Executors.newScheduledThreadPool(2);

    public ChatSession(String roomId, UserConfig config, FirebaseClient firebase, Scanner scanner) {
        this.roomId = roomId;
        this.config = config;
        this.firebase = firebase;
        this.scanner = scanner;
    }

    public void run() {
        // Join the room
        try {
            firebase.joinRoom(roomId, config.getUserId(), config.getUsername(), config.getColor());
        } catch (Exception e) {
            System.err.println("Failed to join room: " + e.getMessage());
            return;
        }

        // Load and display history
        try {
            List<Message> history = firebase.getInitialMessages(roomId);
            long maxTs = 0;
            for (Message msg : history) {
                printMessage(msg);
                if (msg.getTimestamp() > maxTs) maxTs = msg.getTimestamp();
            }
            lastTimestamp.set(maxTs);
        } catch (Exception e) {
            // Non-fatal — just start with no history
        }

        // Poll for new messages every 500 ms
        scheduler.scheduleAtFixedRate(this::pollMessages, 500, 500, TimeUnit.MILLISECONDS);

        // Keep-alive heartbeat every 30 s
        scheduler.scheduleAtFixedRate(
                () -> {
                    try { firebase.updateActivity(roomId, config.getUserId()); } catch (Exception ignored) {}
                },
                30, 30, TimeUnit.SECONDS
        );

        // Read input loop (blocking, on main thread)
        while (running.get()) {
            String line = scanner.nextLine();
            if (!running.get()) break;
            handleInput(line.trim());
        }
    }

    public void stop() {
        if (running.compareAndSet(true, false)) {
            scheduler.shutdownNow();
            try { firebase.leaveRoom(roomId, config.getUserId()); } catch (Exception ignored) {}
        }
    }

    // ── private helpers ──────────────────────────────────────────────────────

    private void pollMessages() {
        try {
            List<Message> newMsgs = firebase.pollMessages(roomId, lastTimestamp.get());
            for (Message msg : newMsgs) {
                printMessage(msg);
                if (msg.getTimestamp() > lastTimestamp.get()) {
                    lastTimestamp.set(msg.getTimestamp());
                }
            }
        } catch (Exception ignored) {}
    }

    private void handleInput(String input) {
        if (input.isEmpty()) return;

        if (input.startsWith("/")) {
            switch (input.toLowerCase()) {
                case "/help" -> printHelp();
                case "/exit" -> {
                    stop();
                    System.exit(0);
                }
                case "/clear" -> System.out.print("\033[H\033[2J");
                default -> System.out.println("[System] Unknown command: " + input + ". Type /help.");
            }
        } else {
            try {
                firebase.sendMessage(roomId, config.getUserId(), config.getUsername(), config.getColor(), input);
            } catch (Exception e) {
                System.err.println("[Error] Failed to send message: " + e.getMessage());
            }
        }
    }

    private void printMessage(Message msg) {
        String time = new java.text.SimpleDateFormat("HH:mm:ss").format(
                new java.util.Date(msg.getTimestamp() * 1000));
        System.out.printf("[%s] %s: %s%n", time, msg.getSender(), msg.getText());
    }

    private void printHelp() {
        System.out.println("""
                Commands:
                  /help   — show this help
                  /clear  — clear the screen
                  /exit   — leave the room and quit
                """);
    }
}
