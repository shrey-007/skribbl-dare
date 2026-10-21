package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Rarity string

const (
	RarityCommon    Rarity = "Common"
	RarityUncommon  Rarity = "Uncommon"
	RarityRare      Rarity = "Rare"
	RarityEpic      Rarity = "Epic"
	RarityLegendary Rarity = "Legendary"
)

type Tier struct {
	Name        string `bson:"name" json:"name"`
	Rarity      Rarity `bson:"rarity" json:"rarity"`
	MinPoints   int    `bson:"min_points" json:"min_points"`
	ColorHex    string `bson:"color_hex" json:"color_hex"`
	StickerID   string `bson:"sticker_id" json:"sticker_id"`
	Description string `bson:"description" json:"description"`
}

var AvailableTiers = []Tier{
	{
		Name:        "The Warm-Up Artist",
		Rarity:      RarityCommon,
		MinPoints:   0,
		ColorHex:    "#10b981", // Emerald green
		StickerID:   "warmup_artist",
		Description: "Beginning the artistic journey with gentle, exploratory strokes.",
	},
	{
		Name:        "Rules Are Rules",
		Rarity:      RarityUncommon,
		MinPoints:   100,
		ColorHex:    "#3b82f6", // Electric blue
		StickerID:   "rules_are_rules",
		Description: "Strict adherence to constraints. The law of the canvas is absolute.",
	},
	{
		Name:        "Blind Picasso",
		Rarity:      RarityRare,
		MinPoints:   300,
		ColorHex:    "#a855f7", // Royal purple
		StickerID:   "blind_picasso",
		Description: "Abstract mastery beyond normal perception. True avant-garde chaos.",
	},
	{
		Name:        "Absolute Menace",
		Rarity:      RarityEpic,
		MinPoints:   750,
		ColorHex:    "#ef4444", // Fiery crimson
		StickerID:   "absolute_menace",
		Description: "Feared by guessers and artists alike. Thrives in severe drawing handicaps.",
	},
	{
		Name:        "What Am I Looking At?",
		Rarity:      RarityLegendary,
		MinPoints:   1500,
		ColorHex:    "#f59e0b", // Radiant gold
		StickerID:   "what_am_i_looking_at",
		Description: "Transcends comprehension. A celestial doodle of unfathomable proportions.",
	},
}

// CalculateTier returns the highest tier unlocked based on total points.
func CalculateTier(points int) Tier {
	highest := AvailableTiers[0]
	for _, tier := range AvailableTiers {
		if points >= tier.MinPoints {
			highest = tier
		}
	}
	return highest
}

type User struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	DiscordID           string             `bson:"discord_id" json:"discord_id"`
	Username            string             `bson:"username" json:"username"`
	GlobalName          string             `bson:"global_name" json:"global_name"`
	Avatar              string             `bson:"avatar" json:"avatar"`
	AvatarURL           string             `bson:"avatar_url" json:"avatar_url"`
	InGameNickname      string             `bson:"ingame_nickname" json:"ingame_nickname"`
	Points              int                `bson:"points" json:"points"`
	Tier                Tier               `bson:"tier" json:"tier"`
	CompletedDaresCount int                `bson:"completed_dares_count" json:"completed_dares_count"`
	CurrentRoomCode     string             `bson:"current_room_code" json:"current_room_code"`
	ActiveDare          *Dare              `bson:"active_dare,omitempty" json:"active_dare,omitempty"`
	CreatedAt           time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time          `bson:"updated_at" json:"updated_at"`
}

type Dare struct {
	ID             string `bson:"id" json:"id"`
	Text           string `bson:"text" json:"text"`
	Category       string `bson:"category" json:"category"`
	Difficulty     Rarity `bson:"difficulty" json:"difficulty"`
	Points         int    `bson:"points" json:"points"`
	IsExperimental bool   `bson:"is_experimental" json:"is_experimental"`
}

type DareHistoryEntry struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	DiscordID   string             `bson:"discord_id" json:"discord_id"`
	DareID      string             `bson:"dare_id" json:"dare_id"`
	DareText    string             `bson:"dare_text" json:"dare_text"`
	Word        string             `bson:"word" json:"word"`
	PointsGiven int                `bson:"points_given" json:"points_given"`
	RoomCode    string             `bson:"room_code" json:"room_code"`
	CompletedAt time.Time          `bson:"completed_at" json:"completed_at"`
}

type CompleteDareRequest struct {
	DareID   string `json:"dare_id"`
	Word     string `json:"word"`
	RoomCode string `json:"room_code"`
}

type CompleteDareResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	PointsEarned int    `json:"points_earned"`
	TotalPoints  int    `json:"total_points"`
	Tier         Tier   `json:"tier"`
	TierUpgraded bool   `json:"tier_upgraded"`
}

type JoinRoomRequest struct {
	RoomCode string `json:"room_code"`
	Nickname string `json:"nickname"`
}

type PlayerBadgeInfo struct {
	DiscordID      string `json:"discord_id"`
	Username       string `json:"username"`
	InGameNickname string `json:"ingame_nickname"`
	Points         int    `json:"points"`
	Tier           Tier   `json:"tier"`
	AvatarURL      string `json:"avatar_url"`
}

type StartRoundDareRequest struct {
	RoomCode        string `json:"room_code"`
	DrawerNickname  string `json:"drawer_nickname"`
	DrawerDiscordID string `json:"drawer_discord_id"`
	DareID          string `json:"dare_id"`
	DareText        string `json:"dare_text"`
	DareDifficulty  string `json:"dare_difficulty"`
	Points          int    `json:"points"`
	Word            string `json:"word"`
}

type VoteDareRequest struct {
	RoomCode        string `json:"room_code"`
	VoterNickname   string `json:"voter_nickname"`
	VoterDiscordID  string `json:"voter_discord_id"`
	DrawerNickname  string `json:"drawer_nickname"`
	DareID          string `json:"dare_id"`
}

type VoteDareResponse struct {
	Success      bool   `json:"success"`
	CurrentVotes int    `json:"current_votes"`
	NeededVotes  int    `json:"needed_votes"`
	Approved     bool   `json:"approved"`
	Message      string `json:"message"`
}
