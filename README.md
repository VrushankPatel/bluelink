# BlueLink

[![Build & Release](https://github.com/VrushankPatel/bluelink/actions/workflows/build.yml/badge.svg)](https://github.com/VrushankPatel/bluelink/actions/workflows/build.yml)
[![Download](https://img.shields.io/github/v/release/VrushankPatel/bluelink?label=download&color=blue)](https://github.com/VrushankPatel/bluelink/releases/latest)

Encrypted, anonymous CLI chat backed by Firebase Realtime Database.

Messages are encrypted end-to-end with **AES-256-GCM** (key derived from the room ID, random nonce per message). No account required — just pick a username on first run.

---

## Download

Grab the latest JAR from the [Releases page](https://github.com/VrushankPatel/bluelink/releases/latest) and run it — no installation, no config:

```bash
java -jar bluelink-1.0.0.jar
```

Requires **Java 17+** only.

---

## Requirements

| Tool | Version |
|------|---------|
| Java | 17+ |
| Maven | 3.8+ (build only) |

---

## Building a shareable JAR

Anyone who has the JAR can chat — no extra setup needed. The Firebase credentials and database URL are bundled inside at build time.

### 1. Add your Firebase credentials

Copy your Firebase service-account JSON to:

```
src/main/resources/firebase-credentials.json
```

> This file is in `.gitignore` — it will never be committed.

### 2. Set the database URL

Edit `src/main/resources/bluelink.properties`:

```properties
firebase.database.url=https://<your-project-id>-default-rtdb.firebaseio.com
```

### 3. Build

```bash
mvn package -q
```

This produces `target/bluelink-1.0.0.jar` — a self-contained fat JAR with everything bundled in.

### 4. Share

Send `target/bluelink-1.0.0.jar` to anyone. They only need Java 17+:

```bash
java -jar bluelink-1.0.0.jar
```

---

## Usage

```bash
# Create a new room (prints the room ID — share it with friends)
java -jar bluelink-1.0.0.jar

# Join an existing room
java -jar bluelink-1.0.0.jar <room-id>
```

On first run you will be prompted for a display name. Your identity is saved to `~/.bluelink/config.json` — completely local, nothing sent to any server.

### In-chat commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Clear the screen |
| `/exit` | Leave the room and quit |
| `Ctrl+C` | Graceful disconnect |

---

## How it works

1. Each room has an 8-digit numeric ID — share it out-of-band with whoever you want to chat with.
2. Messages are encrypted with AES-256-GCM before being written to Firebase. The server never sees plaintext.
3. The encryption key is derived from the room ID — only people who know the room ID can decrypt messages.
4. User identity (ID, display name, color) is stored locally in `~/.bluelink/config.json` — no accounts, no sign-up.

---

## Project structure

```
src/main/
├── java/io/github/vrushankpatel/bluelink/
│   ├── Main.java               # Entry point, argument parsing
│   ├── ChatSession.java        # Input loop + message polling
│   ├── config/
│   │   └── UserConfig.java     # Local identity persistence
│   └── firebase/
│       ├── FirebaseClient.java # All Firebase Realtime DB operations
│       ├── Crypto.java         # AES-256-GCM encrypt/decrypt
│       ├── Message.java        # Message model
│       └── Participant.java    # Participant model
└── resources/
    ├── firebase-credentials.json  # ← add before building (gitignored)
    └── bluelink.properties        # ← set firebase.database.url here
```

---

## License

[MIT](LICENSE)
