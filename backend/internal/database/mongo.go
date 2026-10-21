package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"skribbl-dare-backend/internal/dares"
	"skribbl-dare-backend/internal/models"
)

type MongoDB struct {
	Client      *mongo.Client
	Database    *mongo.Database
	Users       *mongo.Collection
	DareHistory *mongo.Collection
}

func ConnectMongo(uri, dbName string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the primary
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(dbName)
	usersColl := db.Collection("users")
	historyColl := db.Collection("dare_history")

	// Ensure unique index on discord_id
	_, err = usersColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "discord_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("[WARN] Failed to create index on discord_id: %v", err)
	}

	// Ensure index on current_room_code
	_, _ = usersColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "current_room_code", Value: 1}},
	})

	log.Printf("[INFO] Connected successfully to MongoDB at %s (DB: %s)", uri, dbName)
	return &MongoDB{
		Client:      client,
		Database:    db,
		Users:       usersColl,
		DareHistory: historyColl,
	}, nil
}

func (db *MongoDB) GetUserByDiscordID(ctx context.Context, discordID string) (*models.User, error) {
	var user models.User
	err := db.Users.FindOne(ctx, bson.M{"discord_id": discordID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (db *MongoDB) GetUserByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := db.Users.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (db *MongoDB) CreateOrUpdateDiscordUser(ctx context.Context, discordID, username, globalName, avatar string) (*models.User, error) {
	now := time.Now()
	avatarURL := ""
	if avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", discordID, avatar)
	}

	filter := bson.M{"discord_id": discordID}
	update := bson.M{
		"$set": bson.M{
			"username":    username,
			"global_name": globalName,
			"avatar":      avatar,
			"avatar_url":  avatarURL,
			"updated_at":  now,
		},
		"$setOnInsert": bson.M{
			"discord_id":            discordID,
			"points":                0,
			"tier":                  models.AvailableTiers[0],
			"completed_dares_count": 0,
			"created_at":            now,
		},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var user models.User
	err := db.Users.FindOneAndUpdate(ctx, filter, update, opts).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *MongoDB) UpdateInGameInfo(ctx context.Context, discordID, nickname, roomCode string) error {
	filter := bson.M{"discord_id": discordID}
	update := bson.M{
		"$set": bson.M{
			"ingame_nickname":   nickname,
			"current_room_code": roomCode,
			"updated_at":        time.Now(),
		},
	}
	_, err := db.Users.UpdateOne(ctx, filter, update)
	return err
}

func (db *MongoDB) SetActiveDare(ctx context.Context, discordID string, dare *models.Dare) error {
	filter := bson.M{"discord_id": discordID}
	update := bson.M{
		"$set": bson.M{
			"active_dare": dare,
			"updated_at":  time.Now(),
		},
	}
	_, err := db.Users.UpdateOne(ctx, filter, update)
	return err
}

func (db *MongoDB) AwardPointsAndCompleteDare(ctx context.Context, discordID, dareID, word, roomCode string) (*models.CompleteDareResponse, error) {
	user, err := db.GetUserByDiscordID(ctx, discordID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Locate dare details
	dare, found := dares.FindDareByID(dareID)
	if !found {
		if user.ActiveDare != nil && user.ActiveDare.ID == dareID {
			dare = *user.ActiveDare
		} else {
			dare = models.Dare{
				ID:         dareID,
				Text:       "Completed custom dare challenge",
				Category:   "General",
				Difficulty: models.RarityCommon,
				Points:     25,
			}
		}
	}

	oldTier := user.Tier
	newPoints := user.Points + dare.Points
	newTier := models.CalculateTier(newPoints)
	tierUpgraded := newTier.MinPoints > oldTier.MinPoints

	now := time.Now()
	// Update user in DB
	filter := bson.M{"discord_id": discordID}
	update := bson.M{
		"$set": bson.M{
			"points":      newPoints,
			"tier":        newTier,
			"active_dare": nil,
			"updated_at":  now,
		},
		"$inc": bson.M{
			"completed_dares_count": 1,
		},
	}

	_, err = db.Users.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update user points: %w", err)
	}

	// Record history
	historyEntry := models.DareHistoryEntry{
		ID:          primitive.NewObjectID(),
		UserID:      user.ID,
		DiscordID:   discordID,
		DareID:      dare.ID,
		DareText:    dare.Text,
		Word:        word,
		PointsGiven: dare.Points,
		RoomCode:    roomCode,
		CompletedAt: now,
	}
	_, _ = db.DareHistory.InsertOne(ctx, historyEntry)

	return &models.CompleteDareResponse{
		Success:      true,
		Message:      fmt.Sprintf("Dare completed! You earned +%d points.", dare.Points),
		PointsEarned: dare.Points,
		TotalPoints:  newPoints,
		Tier:         newTier,
		TierUpgraded: tierUpgraded,
	}, nil
}

func (db *MongoDB) GetPlayersInRoom(ctx context.Context, roomCode string) ([]models.PlayerBadgeInfo, error) {
	if roomCode == "" {
		return []models.PlayerBadgeInfo{}, nil
	}

	filter := bson.M{"current_room_code": roomCode}
	cursor, err := db.Users.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var players []models.PlayerBadgeInfo
	for cursor.Next(ctx) {
		var u models.User
		if err := cursor.Decode(&u); err == nil {
			players = append(players, models.PlayerBadgeInfo{
				DiscordID:      u.DiscordID,
				Username:       u.Username,
				InGameNickname: u.InGameNickname,
				Points:         u.Points,
				Tier:           u.Tier,
				AvatarURL:      u.AvatarURL,
			})
		}
	}
	return players, nil
}

func (db *MongoDB) GetLeaderboard(ctx context.Context, limit int64) ([]models.User, error) {
	opts := options.Find().SetSort(bson.D{{Key: "points", Value: -1}}).SetLimit(limit)
	cursor, err := db.Users.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (db *MongoDB) GetDareHistory(ctx context.Context, discordID string, limit int64) ([]models.DareHistoryEntry, error) {
	opts := options.Find().SetSort(bson.D{{Key: "completed_at", Value: -1}}).SetLimit(limit)
	cursor, err := db.DareHistory.Find(ctx, bson.M{"discord_id": discordID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var history []models.DareHistoryEntry
	if err := cursor.All(ctx, &history); err != nil {
		return nil, err
	}
	return history, nil
}
