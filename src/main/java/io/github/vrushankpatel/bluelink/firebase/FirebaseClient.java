package io.github.vrushankpatel.bluelink.firebase;

import com.google.auth.oauth2.GoogleCredentials;
import com.google.firebase.FirebaseApp;
import com.google.firebase.FirebaseOptions;
import com.google.firebase.database.*;
import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;

import java.io.*;
import java.lang.reflect.Type;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Wraps Firebase Realtime Database operations.
 *
 * Credential resolution order:
 *   1. Classpath resource "firebase-credentials.json" (bundled in JAR — shareable build)
 *   2. FIREBASE_CREDENTIALS env var (path to file)
 *   3. firebase-credentials.json in the working directory
 *
 * Database URL resolution order:
 *   1. FIREBASE_DATABASE_URL env var
 *   2. firebase.database.url in classpath "bluelink.properties" (bundled in JAR)
 */
public class FirebaseClient {

    private static final Gson   GSON    = new Gson();
    private static final String SYSTEM  = "system";
    private static final long   TIMEOUT = 10;

    private final FirebaseDatabase db;

    public FirebaseClient() throws Exception {
        GoogleCredentials credentials = resolveCredentials();
        String dbUrl = resolveDbUrl();

        FirebaseOptions options = FirebaseOptions.builder()
                .setCredentials(credentials)
                .setDatabaseUrl(dbUrl)
                .build();

        if (FirebaseApp.getApps().isEmpty()) {
            FirebaseApp.initializeApp(options);
        }

        this.db = FirebaseDatabase.getInstance();
    }

    // ── credential / config resolution ───────────────────────────────────────

    private static GoogleCredentials resolveCredentials() throws Exception {
        // 1. Bundled inside JAR (classpath)
        InputStream bundled = FirebaseClient.class.getClassLoader()
                .getResourceAsStream("firebase-credentials.json");
        if (bundled != null) {
            return GoogleCredentials.fromStream(bundled);
        }

        // 2. Env var pointing to a file path
        String envPath = System.getenv("FIREBASE_CREDENTIALS");
        if (envPath != null && !envPath.isBlank()) {
            return GoogleCredentials.fromStream(new FileInputStream(envPath));
        }

        // 3. Working directory fallback
        File local = new File("firebase-credentials.json");
        if (local.exists()) {
            return GoogleCredentials.fromStream(new FileInputStream(local));
        }

        throw new IllegalStateException(
            "Firebase credentials not found.\n" +
            "Either bundle firebase-credentials.json in src/main/resources/ before building,\n" +
            "or set the FIREBASE_CREDENTIALS environment variable."
        );
    }

    private static String resolveDbUrl() throws Exception {
        // 1. Env var (highest priority — useful for dev/CI)
        String envUrl = System.getenv("FIREBASE_DATABASE_URL");
        if (envUrl != null && !envUrl.isBlank()) return envUrl;

        // 2. Bundled properties file
        InputStream propsStream = FirebaseClient.class.getClassLoader()
                .getResourceAsStream("bluelink.properties");
        if (propsStream != null) {
            Properties props = new Properties();
            props.load(propsStream);
            String url = props.getProperty("firebase.database.url", "").trim();
            if (!url.isBlank()) return url;
        }

        throw new IllegalStateException(
            "Firebase Database URL not found.\n" +
            "Set firebase.database.url in src/main/resources/bluelink.properties before building,\n" +
            "or set the FIREBASE_DATABASE_URL environment variable."
        );
    }

    // ── room operations ───────────────────────────────────────────────────────

    public String createRoom(String userId, String username, String color) throws Exception {
        String roomId = String.valueOf(10_000_000 + new Random().nextInt(90_000_000));
        createRoomWithId(roomId, userId, username, color);
        return roomId;
    }

    public void createRoomWithId(String roomId, String userId, String username, String color) throws Exception {
        long now = Instant.now().getEpochSecond();
        set(roomRef(roomId).child("participants").child(userId),
                toMap(new Participant(username, color, now)));
        push(roomRef(roomId).child("messages"),
                toMap(new Message("System", SYSTEM, "#888888", username + " created the room", now)));
    }

    public void joinRoom(String roomId, String userId, String username, String color) throws Exception {
        long now = Instant.now().getEpochSecond();
        set(roomRef(roomId).child("participants").child(userId),
                toMap(new Participant(username, color, now)));
        push(roomRef(roomId).child("messages"),
                toMap(new Message("System", SYSTEM, "#888888", username + " joined the room", now)));
    }

    public void leaveRoom(String roomId, String userId) {
        try {
            Map<String, Object> pData = get(roomRef(roomId).child("participants").child(userId));
            String name = pData != null ? (String) pData.get("name") : "Someone";
            push(roomRef(roomId).child("messages"),
                    toMap(new Message("System", SYSTEM, "#888888", name + " left the room",
                            Instant.now().getEpochSecond())));
            delete(roomRef(roomId).child("participants").child(userId));
        } catch (Exception ignored) {}
    }

    public boolean checkRoomExists(String roomId) throws Exception {
        Map<String, Object> data = get(roomRef(roomId));
        return data != null && !data.isEmpty();
    }

    // ── messaging ─────────────────────────────────────────────────────────────

    public void sendMessage(String roomId, String userId, String username,
                            String color, String text) throws Exception {
        long now = Instant.now().getEpochSecond();
        push(roomRef(roomId).child("messages"),
                toMap(new Message(username, userId, color, Crypto.encrypt(text, roomId), now)));
        update(roomRef(roomId).child("participants").child(userId), Map.of("lastActive", now));
    }

    public List<Message> pollMessages(String roomId, long afterTimestamp) throws Exception {
        Map<String, Object> raw = get(roomRef(roomId).child("messages"));
        if (raw == null || raw.isEmpty()) return List.of();

        List<Message> result = new ArrayList<>();
        for (Map.Entry<String, Object> entry : raw.entrySet()) {
            Message msg = toMessage(entry.getValue());
            if (msg != null && msg.getTimestamp() > afterTimestamp) {
                result.add(decryptMsg(msg, roomId));
            }
        }
        result.sort(Comparator.comparingLong(Message::getTimestamp));
        return result;
    }

    public List<Message> getInitialMessages(String roomId) throws Exception {
        return pollMessages(roomId, 0);
    }

    public void updateActivity(String roomId, String userId) throws Exception {
        update(roomRef(roomId).child("participants").child(userId),
                Map.of("lastActive", Instant.now().getEpochSecond()));
    }

    // ── Firebase sync helpers ─────────────────────────────────────────────────

    @SuppressWarnings("unchecked")
    private Map<String, Object> get(DatabaseReference ref) throws Exception {
        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Map<String, Object>> result = new AtomicReference<>();
        AtomicReference<Exception> error = new AtomicReference<>();
        ref.addListenerForSingleValueEvent(new ValueEventListener() {
            @Override public void onDataChange(DataSnapshot s) { result.set((Map<String,Object>) s.getValue()); latch.countDown(); }
            @Override public void onCancelled(DatabaseError e) { error.set(e.toException()); latch.countDown(); }
        });
        if (!latch.await(TIMEOUT, TimeUnit.SECONDS)) throw new Exception("Firebase read timed out");
        if (error.get() != null) throw error.get();
        return result.get();
    }

    private void set(DatabaseReference ref, Map<String, Object> value) throws Exception {
        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Exception> error = new AtomicReference<>();
        ref.setValue(value, (e, r) -> { if (e != null) error.set(e.toException()); latch.countDown(); });
        if (!latch.await(TIMEOUT, TimeUnit.SECONDS)) throw new Exception("Firebase write timed out");
        if (error.get() != null) throw error.get();
    }

    private void push(DatabaseReference ref, Map<String, Object> value) throws Exception {
        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Exception> error = new AtomicReference<>();
        ref.push().setValue(value, (e, r) -> { if (e != null) error.set(e.toException()); latch.countDown(); });
        if (!latch.await(TIMEOUT, TimeUnit.SECONDS)) throw new Exception("Firebase push timed out");
        if (error.get() != null) throw error.get();
    }

    private void update(DatabaseReference ref, Map<String, Object> value) throws Exception {
        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Exception> error = new AtomicReference<>();
        ref.updateChildren(value, (e, r) -> { if (e != null) error.set(e.toException()); latch.countDown(); });
        if (!latch.await(TIMEOUT, TimeUnit.SECONDS)) throw new Exception("Firebase update timed out");
        if (error.get() != null) throw error.get();
    }

    private void delete(DatabaseReference ref) throws Exception {
        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Exception> error = new AtomicReference<>();
        ref.removeValue((e, r) -> { if (e != null) error.set(e.toException()); latch.countDown(); });
        if (!latch.await(TIMEOUT, TimeUnit.SECONDS)) throw new Exception("Firebase delete timed out");
        if (error.get() != null) throw error.get();
    }

    // ── conversion helpers ────────────────────────────────────────────────────

    private DatabaseReference roomRef(String roomId) {
        return db.getReference("rooms").child(roomId);
    }

    @SuppressWarnings("unchecked")
    private Message toMessage(Object raw) {
        if (!(raw instanceof Map)) return null;
        Map<String, Object> map = (Map<String, Object>) raw;
        return new Message(
            (String) map.getOrDefault("sender", ""),
            (String) map.getOrDefault("senderId", ""),
            (String) map.getOrDefault("color", "#888888"),
            (String) map.getOrDefault("text", ""),
            toLong(map.get("timestamp"))
        );
    }

    private long toLong(Object v) {
        if (v instanceof Long)    return (Long) v;
        if (v instanceof Integer) return ((Integer) v).longValue();
        if (v instanceof Double)  return ((Double) v).longValue();
        return 0L;
    }

    private Map<String, Object> toMap(Object obj) {
        Type type = new TypeToken<Map<String, Object>>(){}.getType();
        return GSON.fromJson(GSON.toJson(obj), type);
    }

    private Message decryptMsg(Message msg, String roomId) {
        if (SYSTEM.equals(msg.getSenderId())) return msg;
        try {
            msg.setText(Crypto.decrypt(msg.getText(), roomId));
        } catch (Exception e) {
            msg.setText("[Failed to decrypt message]");
        }
        return msg;
    }
}
