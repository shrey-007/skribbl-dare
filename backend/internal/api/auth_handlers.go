package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"skribbl-dare-backend/internal/auth"
	"skribbl-dare-backend/internal/config"
	"skribbl-dare-backend/internal/database"
	"skribbl-dare-backend/internal/models"
)

type AuthHandler struct {
	cfg *config.Config
	db  *database.MongoDB
}

func NewAuthHandler(cfg *config.Config, db *database.MongoDB) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db}
}

func (h *AuthHandler) DiscordLogin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.DiscordClientID == "" {
		http.Error(w, "Discord OAuth is not configured. Set DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET in .env or use dev login.", http.StatusServiceUnavailable)
		return
	}

	state := fmt.Sprintf("%d", time.Now().UnixNano())
	authURL := auth.GetDiscordAuthURL(h.cfg.DiscordClientID, h.cfg.DiscordRedirectURI, state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) DiscordCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code from Discord", http.StatusBadRequest)
		return
	}

	tokenResp, err := auth.ExchangeDiscordCode(h.cfg.DiscordClientID, h.cfg.DiscordClientSecret, h.cfg.DiscordRedirectURI, code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to exchange code: %v", err), http.StatusInternalServerError)
		return
	}

	discordUser, err := auth.FetchDiscordUser(tokenResp.AccessToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch Discord user: %v", err), http.StatusInternalServerError)
		return
	}

	user, err := h.db.CreateOrUpdateDiscordUser(r.Context(), discordUser.ID, discordUser.Username, discordUser.GlobalName, discordUser.Avatar)
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(user, h.cfg.JWTSecret)
	if err != nil {
		http.Error(w, "Failed to create session token", http.StatusInternalServerError)
		return
	}

	// Render beautiful HTML callback page communicating token to extension
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderAuthSuccessPage(w, token, user)
}

type DevLoginRequest struct {
	Username string `json:"username"`
	Points   int    `json:"points"`
}

func (h *AuthHandler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.AllowMockAuth {
		http.Error(w, "Mock authentication is disabled", http.StatusForbidden)
		return
	}

	var req DevLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		req.Username = "DoodleMaster"
	}

	mockDiscordID := fmt.Sprintf("mock_%s", req.Username)
	user, err := h.db.CreateOrUpdateDiscordUser(r.Context(), mockDiscordID, req.Username, req.Username, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Points > 0 {
		user.Points = req.Points
		user.Tier = models.CalculateTier(req.Points)
		_ = h.db.Users.FindOneAndUpdate(r.Context(),
			map[string]interface{}{"discord_id": mockDiscordID},
			map[string]interface{}{"$set": map[string]interface{}{"points": req.Points, "tier": user.Tier}},
		)
	}

	token, err := auth.GenerateToken(user, h.cfg.JWTSecret)
	if err != nil {
		http.Error(w, "Failed to sign token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"user":    user,
	})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("claims").(*auth.JWTClaims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByDiscordID(r.Context(), claims.DiscordID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	history, _ := h.db.GetDareHistory(r.Context(), claims.DiscordID, 15)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":    user,
		"history": history,
		"tiers":   models.AvailableTiers,
	})
}

func renderAuthSuccessPage(w http.ResponseWriter, token string, user *models.User) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Skribbl Dare — Authenticated!</title>
  <style>
    body {
      background: #0f172a;
      color: #f8fafc;
      font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
    }
    .card {
      background: rgba(30, 41, 59, 0.9);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 16px;
      padding: 36px;
      text-align: center;
      max-width: 440px;
      box-shadow: 0 20px 40px rgba(0,0,0,0.5);
      backdrop-filter: blur(10px);
    }
    .badge {
      display: inline-block;
      padding: 6px 14px;
      background: %s22;
      border: 1px solid %s;
      color: %s;
      border-radius: 9999px;
      font-weight: bold;
      font-size: 13px;
      margin-top: 12px;
    }
    h1 { margin: 12px 0 6px; font-size: 24px; color: #fff; }
    p { color: #94a3b8; font-size: 15px; line-height: 1.5; }
    .btn {
      display: inline-block;
      margin-top: 20px;
      padding: 12px 28px;
      background: #6366f1;
      color: white;
      text-decoration: none;
      border-radius: 10px;
      font-weight: bold;
      cursor: pointer;
      transition: background 0.2s;
    }
    .btn:hover { background: #4f46e5; }
  </style>
</head>
<body>
  <div class="card">
    <div style="font-size: 54px;">🎨⚡</div>
    <h1>Welcome, %s!</h1>
    <div class="badge">%s (%s)</div>
    <p>You have successfully connected with Discord. Skribbl Dare is now active in your game sessions!</p>
    <button class="btn" onclick="saveAndClose()">Return to Skribbl.io</button>
  </div>

  <script>
    const token = %q;
    const user = %s;

    // Send token to extension via window.postMessage
    window.postMessage({ type: 'SKRIBBL_DARE_AUTH_SUCCESS', token: token, user: user }, '*');
    localStorage.setItem('skribbl_dare_jwt', token);

    function saveAndClose() {
      if (window.opener) {
        window.opener.postMessage({ type: 'SKRIBBL_DARE_AUTH_SUCCESS', token: token, user: user }, '*');
        setTimeout(() => window.close(), 600);
      } else {
        window.location.href = 'https://skribbl.io';
      }
    }

    // Auto close if opened as OAuth popup
    if (window.opener) {
      setTimeout(saveAndClose, 1200);
    }
  </script>
</body>
</html>`, user.Tier.ColorHex, user.Tier.ColorHex, user.Tier.ColorHex, user.Username, user.Tier.Name, user.Tier.Rarity, token, mustJSON(user))

	w.Write([]byte(html))
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
