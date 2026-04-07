package io.github.vrushankpatel.bluelink.config;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;

import java.io.*;
import java.nio.file.*;
import java.util.UUID;

/**
 * Persists user identity (id, username, color) in ~/.bluelink/config.json.
 */
public class UserConfig {

    private static final String CONFIG_DIR  = ".bluelink";
    private static final String CONFIG_FILE = "config.json";
    private static final Gson   GSON        = new GsonBuilder().setPrettyPrinting().create();

    private String userId;
    private String username;
    private String color;

    // Gson needs a no-arg constructor
    public UserConfig() {}

    private UserConfig(String userId, String username, String color) {
        this.userId   = userId;
        this.username = username;
        this.color    = color;
    }

    // ── factory ──────────────────────────────────────────────────────────────

    public static UserConfig loadOrCreate() throws IOException {
        Path configPath = configPath();
        Files.createDirectories(configPath.getParent());

        if (Files.exists(configPath)) {
            try (Reader r = Files.newBufferedReader(configPath)) {
                return GSON.fromJson(r, UserConfig.class);
            }
        }

        return createNew(configPath);
    }

    private static UserConfig createNew(Path configPath) throws IOException {
        System.out.print("Enter your name: ");
        BufferedReader br = new BufferedReader(new InputStreamReader(System.in));
        String name = br.readLine();
        if (name == null || name.isBlank()) {
            throw new IllegalArgumentException("Name cannot be empty.");
        }
        name = name.trim();

        String userId = "user_" + UUID.randomUUID().toString().replace("-", "").substring(0, 8);
        String color  = randomHexColor();

        UserConfig cfg = new UserConfig(userId, name, color);
        try (Writer w = Files.newBufferedWriter(configPath)) {
            GSON.toJson(cfg, w);
        }
        return cfg;
    }

    // ── helpers ───────────────────────────────────────────────────────────────

    private static Path configPath() {
        return Paths.get(System.getProperty("user.home"), CONFIG_DIR, CONFIG_FILE);
    }

    /** Returns a random bright hex color suitable for terminal display. */
    private static String randomHexColor() {
        // Pick a random hue, high saturation & value → always a vivid color
        float hue = (float) Math.random();
        java.awt.Color c = java.awt.Color.getHSBColor(hue, 0.7f, 0.9f);
        return String.format("#%02X%02X%02X", c.getRed(), c.getGreen(), c.getBlue());
    }

    // ── getters ───────────────────────────────────────────────────────────────

    public String getUserId()   { return userId; }
    public String getUsername() { return username; }
    public String getColor()    { return color; }
}
