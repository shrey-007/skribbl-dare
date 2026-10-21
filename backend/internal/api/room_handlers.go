package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"skribbl-dare-backend/internal/auth"
	"skribbl-dare-backend/internal/config"
	"skribbl-dare-backend/internal/database"
	"skribbl-dare-backend/internal/models"
	"skribbl-dare-backend/internal/ws"
)

type RoomHandler struct {
	cfg *config.Config
	db  *database.MongoDB
	hub *ws.Hub
}

func NewRoomHandler(cfg *config.Config, db *database.MongoDB, hub *ws.Hub) *RoomHandler {
	return &RoomHandler{cfg: cfg, db: db, hub: hub}
}

func (h *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	var req models.JoinRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RoomCode == "" {
		http.Error(w, "room_code is required", http.StatusBadRequest)
		return
	}

	claims, ok := r.Context().Value("claims").(*auth.JWTClaims)
	if ok && claims != nil {
		_ = h.db.UpdateInGameInfo(r.Context(), claims.DiscordID, req.Nickname, req.RoomCode)
	}

	players, err := h.db.GetPlayersInRoom(r.Context(), req.RoomCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"room_code": req.RoomCode,
		"players":   players,
	})
}

func (h *RoomHandler) GetRoomPlayers(w http.ResponseWriter, r *http.Request) {
	roomCode := chi.URLParam(r, "code")
	if roomCode == "" {
		http.Error(w, "Room code is required", http.StatusBadRequest)
		return
	}

	players, err := h.db.GetPlayersInRoom(r.Context(), roomCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"room_code": roomCode,
		"players":   players,
	})
}

func (h *RoomHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	roomCode := r.URL.Query().Get("room_code")
	nickname := r.URL.Query().Get("nickname")

	var discordID = "guest"
	var tier = models.AvailableTiers[0]
	var points = 0
	var avatarURL = ""

	if tokenStr != "" {
		if claims, err := auth.ValidateToken(tokenStr, h.cfg.JWTSecret); err == nil && claims != nil {
			discordID = claims.DiscordID
			if user, err := h.db.GetUserByDiscordID(r.Context(), claims.DiscordID); err == nil && user != nil {
				tier = user.Tier
				points = user.Points
				avatarURL = user.AvatarURL
				if nickname == "" {
					nickname = user.Username
				}
			}
		}
	}

	ws.ServeWS(h.hub, w, r, discordID, roomCode, nickname, tier, points, avatarURL)
}
