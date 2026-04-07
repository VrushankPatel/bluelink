package io.github.vrushankpatel.bluelink.firebase;

import com.google.auth.oauth2.GoogleCredentials;
import com.google.firebase.FirebaseApp;
import com.google.firebase.FirebaseOptions;
import com.google.firebase.database.*;
import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;

import java.io.FileInputStream;
import java.io.InputStream;
import java.lang.reflect.Type;
import java.time.Instant;
import java.util.*;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Wraps Firebase Realtime Database operations.
 *
 * Credentials are resolved in order:
 *   1. FIREBASE_CREDENTIALS env var (path to service-account JSON)
 *   2. firebase-credentials.json in the working directory
 *
 * Database URL is read from FIREBASE_DATABASE_URL env var.
 */
public class FirebaseClient {

    private static final Gson   GSON    = new Gson();
    private static final String SYSTEM  = "system";
    private static final long   TIMEOUT = 10; // seconds for sync calls

    private final FirebaseDatabase db;

    public FirebaseClient() throws Exception {
        String credFile = System.getenv("FIREBASE_CREDENTIALS");
        if (credFile == null || credFile.isBlank()) {
            credFile = "firebase-credentials.json";
        }

        InputStream credStream = new FileInputStream(credFile);
        GoogleCredentials credentials = GoogleCredentials.fromStream(credStream);

        String dbUrl = System.getenv("FIREBASE_DATABASE_URL");
        if (dbUrl == null || dbUrl.isBlank()) {
            throw new IllegalStateException("FIREBASE_DATABASE_URL environment variable is not set.");
        }

        FirebaseOptions options = FirebaseOptions.builder()
                .setCredentials(credentials)
                .setDatabaseUrl(dbUrl)
                .build();

        if (FirebaseApp.getApps().isEmpty()) {
            FirebaseApp.initializeApp(options);
        }

        this.db = FirebaseDatabase.getInstance();
    }

    // ── room operations ───────────────────────────────────────────────────────

    public String createRoom(String userId, String username, String color) throws Exception {
        String roomId = String.valueOf(10_000_000 + new Random().nextInt(90_000_000));
        createRoomWithId(roomId, userId, username, color);
        return roomId;
    }

    public void createRoomWithId(String roomId, String userId, String username, String color) throws Exception {
        long now = Instant.now().getEpochSecond();

        Participant participant = new Participant(username, color, now);
        set(roomRef(roomId).child("participants").child(userId), toMap(participant));

        Message welcome = new Message("System", SYSTEM, "#888888",
                username + " created the room", now);
        push(roomRef(roomId).child("messages"), toMap(welcome));
    }

    public void joinRoom(String roomId, String userId, String username, String color) throws Exception {
        long now = Instant.now().getEpochSecond();

        Participant participant = new Participant(username, color, now);
        set(roomRef(roomId).child("participants").child(userId), toMap(participant));

        Message joinMsg = new Message("System", SYSTEM, "#888888",
                username + " joined the room", now);
        push(roomRef(roomId).child("messages"), toMap(joinMsg));
    }

    public void leaveRoom(String roomId, String userId) throws Exception {
        // Best-effort: get username then post leave message and remove participant
        try {
            Map<String, Object> pData = get(roomRef(roomId).child("participants").child(userId));
            String name = pData != null ? (String) pData.get("name") : "Someone";

            Message leaveMsg = new Message("System", SYSTEM, "#888888",
                    name + " left the room", Instant.now().getEpochSecond());
            push(roomRef(roomId).child("messages"), toMap(leaveMsg));

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
        String encrypted = Crypto.encrypt(text, roomId);
        long   now       = Instant.now().getEpochSecond();

        Message msg = new Message(username, userId, color, encrypted, now);
        push(roomRef(roomId).child("messages"), toMap(msg));

        // Update last-active
        Map<String, Object> activity = Map.of("lastActive", now);
        update(roomRef(roomId).child("participants").child(userId), activity);
    }

    /**
     * Returns messages with timestamp > afterTimestamp, sorted ascending.
     * Used by the polling loop in ChatSession.
     */
    public List<Message> pollMessages(String roomId, long afterTimestamp) throws Exception {
        Map<String, Object> raw = get(roomRef(roomId).child("messages"));
        if (raw == null || raw.isEmpty()) return List.of();

        List<Message> result = new ArrayList<>();
        for (Map.Entry<String, Object> entry : raw.entrySet()) {
            Message msg = toMessage(entry.getValue());
            if (msg != null && msg.getTimestamp() > afterTimestamp) {
                result.add(decrypt(msg, roomId));
            }
        }
        result.sort(Comparator.comparingLong(Message::getTimestamp));
        return result;
    }

    /** Fetches all messages for initial history display. */
    public List<Message> getInitialMessages(String roomId) throws Exception {
        return pollMessages(roomId, 0);
    }

    public void updateActivity(String roomId, String userId) throws Exception {
        Map<String, Object> activity = Map.of("lastActive", Instant.now().getEpochSecond());
        update(roomRef(roomId).child("participants").child(userId), activity);
    }

    // ── Firebase sync helpers ─────────────────────────────────────────────────

    @SuppressWarnings("unchecked")
    private Map<String, Object> get(DatabaseReference ref) throws Exception {
        CountDownLatch latch = new CountDownLatch(1);
        AtomicReference<Map<String, Object>> result = new AtomicReference<>();
        AtomicReference<Exception> error = new AtomicReference<>();

        ref.addListenerForSingleValueEvent(new ValueEventListener() {
            @Override public void onDataChange(DataSnapshot snapshot) {
                result.set((Map<String, Object>) snapshot.getValue());
                latch.countDown();
            }
            @Override public void onCancelled(DatabaseError e) {
                error.set(e.toException());
                latch.countDown();
            }
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
        String sender    = (String) map.getOrDefault("sender", "");
        String senderId  = (String) map.getOrDefault("senderId", "");
        String color     = (String) map.getOrDefault("color", "#888888");
        String text      = (String) map.getOrDefault("text", "");
        long   timestamp = toLong(map.get("timestamp"));
        return new Message(sender, senderId, color, text, timestamp);
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

    private Message decrypt(Message msg, String roomId) {
        if (SYSTEM.equals(msg.getSenderId())) return msg;
        try {
            msg.setText(Crypto.decrypt(msg.getText(), roomId));
        } catch (Exception e) {
            msg.setText("[Failed to decrypt message]");
        }
        return msg;
    }
}
