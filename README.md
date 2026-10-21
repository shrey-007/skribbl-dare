# 🎨 Skribbl Dare — Challenges & Achievement Badges

A full-stack Chrome Extension and Go backend for **[skribbl.io](https://skribbl.io)** that adds randomized drawing dares, a **"Completed Dare"** reward button directly into the game interface, and real-time achievement tags with custom stickers next to players' names!

---

## 🌟 Features

1. **Word Selection Dares**:
   - Whenever it is your turn to draw, the word selection modal prompts you with an active **Dare Challenge** (from a database of 150+ challenges) alongside the 3 word choices.
   - Choose the word you think you can complete the dare with, or use the **🔄 Reroll** button!
2. **"Completed Dare" Button in Skribbl UI**:
   - Injected directly into the Skribbl game rating container / toolbar next to the thumbs-up / like button.
   - Drawer clicks to claim completion — triggers celebratory confetti bursts and synthesized audio fanfare!
3. **Point Rewards & 5 Achievement Tiers**:
   - Points are awarded based on dare difficulty (`Common`, `Uncommon`, `Rare`, `Epic`, `Legendary`).
   - Tiers & custom SVG stickers:
     | Tier Name | Rarity | Points Required | Sticker Theme |
     | :--- | :--- | :--- | :--- |
     | **The Warm-Up Artist** | Common | 0 pts | Flame paintbrush with artist palette |
     | **Rules Are Rules** | Uncommon | 100 pts | Law book with golden gavel and checks |
     | **Blind Picasso** | Rare | 300 pts | Cubist abstract face with retro shades |
     | **Absolute Menace** | Epic | 750 pts | Mischievous demon horns with wicked grin |
     | **What Am I Looking At?** | Legendary | 1500 pts | Cosmic portal magnifying glass & stars |
4. **Player List Badges & Stickers**:
   - Whenever any player in the room has an achievement tag, their custom animated sticker and colored tier pill display right beside their name in the Skribbl.io player list.
5. **Discord Authentication**:
   - Standard Discord OAuth2 flow (just like Typo client) stores user identity, points, and achievements securely in MongoDB.
   - Includes instant one-click Dev Demo Login for development testing without requiring Discord Bot keys immediately.
6. **Real-Time Room Sync**:
   - Go WebSocket Hub syncs room players, active dares, and dare completion celebrations live across extension users in the same room.

---

## 🏗️ Architecture

```
skribbl-dare/
├── backend/
│   ├── cmd/server/main.go            # HTTP & WebSocket Server
│   ├── internal/
│   │   ├── api/                      # Routes, auth, dares, and room handlers
│   │   ├── auth/                     # Discord OAuth2 and JWT token logic
│   │   ├── config/                   # Environment loader
│   │   ├── database/                 # MongoDB client & queries
│   │   ├── dares/                    # 150+ Dares seed catalog
│   │   ├── models/                   # Structs for Users, Dares, Tiers
│   │   └── ws/                       # Gorilla WebSocket Hub
│   ├── .env                          # Local environment settings
│   └── go.mod
└── extension/
    ├── manifest.json                 # Chrome Manifest V3
    ├── assets/
    │   ├── icons/                    # Extension icons
    │   └── stickers/                 # 5 Custom SVG Stickers
    ├── background/
    │   └── service-worker.js         # Token management & API proxy
    ├── content/
    │   ├── skribbl.js                # Game DOM observer, button & card injection
    │   └── skribbl.css               # Game UI overlay styling
    └── popup/
        ├── popup.html                # Extension dashboard UI
        ├── popup.css                 # Sleek dark theme styling
        └── popup.js                  # Login, tier progress, and history logic
```

---

## 🚀 Getting Started

### 1. Prerequisites
- **Go** 1.22+
- **MongoDB** running locally on `localhost:27017` (or MongoDB Atlas connection string)
- **Google Chrome**, **Brave**, or **Microsoft Edge**

### 2. Start the Go Backend
```bash
cd backend

# Ensure dependencies are tidy
go mod download

# Build and run the server
go run ./cmd/server/main.go
```
The server will start on `http://localhost:8090`.

Verify health check:
```bash
curl http://localhost:8090/health
# {"status":"ok","service":"skribbl-dare-api"}
```

### 3. Load the Extension into Chrome
1. Open Google Chrome and navigate to `chrome://extensions/`
2. Toggle on **"Developer mode"** in the top-right corner.
3. Click **"Load unpacked"** in the top-left.
4. Select the `skribbl-dare/extension` directory.
5. The **Skribbl Dare** extension icon will now appear in your browser toolbar!

---

## 🔐 How Authentication Works (Discord OAuth2)

Like Typo and other top Skribbl.io companion tools, user authentication connects via **Discord OAuth2**:

1. **User clicks "Connect with Discord"** in the extension popup.
2. The browser navigates to `GET /api/auth/discord/login`, which redirects to:
   ```
   https://discord.com/api/oauth2/authorize?client_id={CLIENT_ID}&redirect_uri={REDIRECT_URI}&response_type=code&scope=identify%20email
   ```
3. Discord prompts the user to authorize.
4. Discord redirects to `GET /api/auth/discord/callback?code=...`.
5. The Go backend exchanges the authorization code for a Discord access token, retrieves the user's profile (`id`, `username`, `avatar`), upserts the record in **MongoDB**, and generates a signed **JWT**.
6. The callback page posts the JWT back to the extension via `window.postMessage`, storing it securely in `chrome.storage.local`.
7. Subsequent requests to `/api/dares/complete` pass `Authorization: Bearer <JWT>` to claim dare rewards.

### Setting Up Your Own Discord App (Optional)
1. Go to the [Discord Developer Portal](https://discord.com/developers/applications).
2. Create an Application, go to **OAuth2**, and add a Redirect:
   `http://localhost:8090/api/auth/discord/callback`
3. Copy your **Client ID** and **Client Secret**.
4. In `backend/.env`, set:
   ```env
   DISCORD_CLIENT_ID=your_client_id_here
   DISCORD_CLIENT_SECRET=your_client_secret_here
   DISCORD_REDIRECT_URI=http://localhost:8090/api/auth/discord/callback
   ```

### Quick Dev Demo Login (Built-In)
If you don't have Discord API credentials right away, open the extension popup and use the **"Instant Demo Login"** box:
- Enter any username (e.g. `PicassoPro`) and initial points (e.g. `350`).
- Instantly logs you in and sets your tier and sticker so you can test in-game immediately!

---

## 🎮 How to Play with Skribbl Dare

1. Go to **[https://skribbl.io](https://skribbl.io)**.
2. Join a public room or create a private room with friends.
3. When it is your turn to pick a word:
   - A glowing **Dare Card** will appear right above your 3 word choices.
   - Shows the random challenge and points bonus (e.g. *Draw using a maximum of 3 strokes [+120 pts]*).
   - Click the word you want to draw.
4. During your turn:
   - A floating **Dare HUD** stays in the corner of your screen as a reminder.
   - When you finish fulfilling your dare, click the **"🎯 Completed Dare!"** button next to the like button.
   - You will hear the celebration fanfare, see confetti burst across the canvas, and your points will be banked to your MongoDB profile!
5. In the player list, your custom **Sticker** and **Tier Badge** will proudly display next to your username for everyone to see!
