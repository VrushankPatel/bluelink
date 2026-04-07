package io.github.vrushankpatel.bluelink;

import io.github.vrushankpatel.bluelink.config.UserConfig;
import io.github.vrushankpatel.bluelink.firebase.FirebaseClient;

import java.util.Scanner;

public class Main {

    public static void main(String[] args) throws Exception {
        printBanner();

        UserConfig config = UserConfig.loadOrCreate();
        FirebaseClient firebase = new FirebaseClient();

        String roomId;
        Scanner scanner = new Scanner(System.in);

        if (args.length > 0) {
            roomId = args[0];
            boolean exists = firebase.checkRoomExists(roomId);
            if (!exists) {
                System.out.printf("Room %s does not exist. Create it? (y/N): ", roomId);
                String response = scanner.nextLine().trim().toLowerCase();
                if (response.equals("y") || response.equals("yes")) {
                    firebase.createRoomWithId(roomId, config.getUserId(), config.getUsername(), config.getColor());
                    System.out.printf("Room %s created.%n", roomId);
                } else {
                    System.out.println("Exiting.");
                    System.exit(0);
                }
            }
        } else {
            roomId = firebase.createRoom(config.getUserId(), config.getUsername(), config.getColor());
        }

        System.out.printf("Connecting to room: %s%n", roomId);
        System.out.println("Type a message and press Enter to send. Commands: /help, /clear, /exit");
        System.out.println("─".repeat(60));

        ChatSession session = new ChatSession(roomId, config, firebase, scanner);

        // Graceful shutdown on Ctrl+C
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            System.out.println("\nDisconnecting...");
            session.stop();
        }));

        session.run();
    }

    private static void printBanner() {
        System.out.println("""
                ╔══════════════════════════════════╗
                ║   BlueLink — encrypted CLI chat  ║
                ╚══════════════════════════════════╝
                """);
    }
}
