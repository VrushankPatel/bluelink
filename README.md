# BlueLink

Encrypted, anonymous CLI chat backed by Firebase Realtime Database.

Messages are encrypted with **AES-256-GCM** (key derived from the room ID via SHA-256, random nonce per message). No account required — just a username stored locally.

---

## Requirements

| Tool | Version |
|------|---------|
| Java | 17+ |
| Maven | 3.8+ |
| Firebase project | Realtime Database enabled |

---

## Setup

### 1. Firebase credentials

Download your Firebase service-account JSON from the Firebase console and place it as `firebase-credentials.json` in the working directory, **or** point to it via an environment variable:

```bash
export FIREBASE_CREDENTIALS=/path/to/service-account.json
export FIREBASE_DATABASE_URL=https://<your-project>.firebaseio.com
```

### 2. Build

```bash
mvn package -q
```

This produces a fat JAR at `target/bluelink-1.0.0.jar`.

---

## Usage

```bash
# Create a new room (prints the room ID)
java -jar target/bluelink-1.0.0.jar

# Join an existing room
java -jar target/bluelink-1.0.0.jar <room-id>
```

On first run you will be prompted for a display name. Your identity is saved to `~/.bluelink/config.json`.

### In-chat commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Clear the screen |
| `/exit` | Leave the room and quit |
| `Ctrl+C` | Graceful disconnect |

---

## How it works

1. Each room has an 8-digit numeric ID.
2. Messages are encrypted before being written to Firebase and decrypted on read — the Firebase backend never sees plaintext.
3. The encryption key is derived from the room ID, so only people who know the room ID can read the messages.
4. User identity (ID, display name, color) is stored locally in `~/.bluelink/config.json` — nothing is tied to an account.

---

## Project structure

```
src/main/java/io/github/vrushankpatel/bluelink/
├── Main.java               # Entry point, argument parsing
├── ChatSession.java        # Input loop + message polling
├── config/
│   └── UserConfig.java     # Local identity persistence
└── firebase/
    ├── FirebaseClient.java # All Firebase Realtime DB operations
    ├── Crypto.java         # AES-256-GCM encrypt/decrypt
    ├── Message.java        # Message model
    └── Participant.java    # Participant model
```

---

## License

[MIT](LICENSE)
