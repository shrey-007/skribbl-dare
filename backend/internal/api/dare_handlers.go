package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"skribbl-dare-backend/internal/auth"
	"skribbl-dare-backend/internal/database"
	"skribbl-dare-backend/internal/dares"
	"skribbl-dare-backend/internal/models"
	"skribbl-dare-backend/internal/ws"
)

type DareHandler struct {
	db  *database.MongoDB
	hub *ws.Hub
}

func NewDareHandler(db *database.MongoDB, hub *ws.Hub) *DareHandler {
	return &DareHandler{db: db, hub: hub}
}

func (h *DareHandler) GetRandomDare(w http.ResponseWriter, r *http.Request) {
	dare := dares.GetRandomDare()

	// If request has JWT claims, associate active dare with user
	if claims, ok := r.Context().Value("claims").(*auth.JWTClaims); ok && claims != nil {
		_ = h.db.SetActiveDare(r.Context(), claims.DiscordID, &dare)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"dare":    dare,
	})
}

func (h *DareHandler) GetAllDares(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": len(dares.AllDares),
		"dares": dares.AllDares,
	})
}

func (h *DareHandler) GetTiers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tiers": models.AvailableTiers,
	})
}

func (h *DareHandler) CompleteDare(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("claims").(*auth.JWTClaims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized: Discord login required to earn dare points", http.StatusUnauthorized)
		return
	}

	var req models.CompleteDareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DareID == "" {
		http.Error(w, "Invalid request body: dare_id is required", http.StatusBadRequest)
		return
	}

	resp, err := h.db.AwardPointsAndCompleteDare(r.Context(), claims.DiscordID, req.DareID, req.Word, req.RoomCode)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to complete dare: %v", err), http.StatusInternalServerError)
		return
	}

	// Fetch updated user to broadcast
	updatedUser, _ := h.db.GetUserByDiscordID(r.Context(), claims.DiscordID)
	if updatedUser != nil && req.RoomCode != "" {
		// Broadcast to room WebSocket
		eventPayload, _ := json.Marshal(map[string]interface{}{
			"user":          updatedUser.Username,
			"nickname":      updatedUser.InGameNickname,
			"points_earned": resp.PointsEarned,
			"total_points":  resp.TotalPoints,
			"tier":          resp.Tier,
			"tier_upgraded": resp.TierUpgraded,
			"word":          req.Word,
		})
		h.hub.BroadcastToRoom(req.RoomCode, ws.EventDareCompleted, eventPayload)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *DareHandler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.GetLeaderboard(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"leaderboard": users,
	})
}
