package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const ChatRetention = 90 * 24 * time.Hour

var (
	ErrChatSenderNotFound    = errors.New("chat sender profile not found")
	ErrChatRecipientNotFound = errors.New("chat recipient not found")
	ErrChatRateLimited       = errors.New("chat rate limit exceeded")
	ErrChatForbidden         = errors.New("conversation access denied")
	ErrChatConversationNotFound = errors.New("conversation not found")
)

type ChatMessage struct {
	ID                int64     `json:"id"`
	Channel           string    `json:"channel"`
	SenderUserID      int       `json:"senderUserId"`
	SenderCharacterID *int      `json:"senderCharacterId,omitempty"`
	SenderNickname    string    `json:"senderNickname"`
	Body              string    `json:"body"`
	PartyID           *string   `json:"partyId,omitempty"`
	RecipientUserID   *int      `json:"recipientUserId,omitempty"`
	ConversationID    *int64    `json:"conversationId,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

type ChatCreate struct {
	Channel         string
	SenderUserID    int
	Body            string
	PartyID         string
	RecipientUserID int
	ConversationID int64
}

type ChatHistoryQuery struct {
	Channel      string
	ViewerUserID int
	PartyID      string
	OtherUserID  int
	ConversationID int64
	BeforeID     int64
	Limit        int
}

type ChatStore struct{ DB *sql.DB }

func (s ChatStore) PlayerExists(ctx context.Context, userID int) (bool, error) {
	var exists bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM player_profiles WHERE user_id=$1)`, userID).Scan(&exists)
	return exists, err
}

func (s ChatStore) Create(ctx context.Context, input ChatCreate) (ChatMessage, error) {
	var message ChatMessage
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return message, err
	}
	defer tx.Rollback()

	// A database lock keeps limits consistent across gameservice replicas.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(81421, $1)`, input.SenderUserID); err != nil {
		return message, err
	}
	var count int
	var last sql.NullTime
	var now time.Time
	err = tx.QueryRowContext(ctx, `
        SELECT COUNT(*), MAX(created_at), clock_timestamp()
        FROM chat_messages
        WHERE sender_user_id=$1 AND created_at > clock_timestamp() - interval '1 minute'
    `, input.SenderUserID).Scan(&count, &last, &now)
	if err != nil {
		return message, err
	}
	if count >= 20 || (last.Valid && now.Sub(last.Time) < time.Second) {
		return message, ErrChatRateLimited
	}

	var nickname string
	var characterID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
        SELECT LEFT(COALESCE(NULLIF(BTRIM(p.name), ''), 'Player'), 100), c.id
        FROM player_profiles p
        LEFT JOIN player_characters c ON c.id=p.selected_character_id AND c.user_id=p.user_id
        WHERE p.user_id=$1
    `, input.SenderUserID).Scan(&nickname, &characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return message, ErrChatSenderNotFound
	}
	if err != nil {
		return message, err
	}
	if input.Channel == "PRIVATE" {
		var member bool
		err = tx.QueryRowContext(ctx, `SELECT true FROM chat_conversation_participants WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL FOR SHARE`, input.ConversationID, input.SenderUserID).Scan(&member)
		if errors.Is(err, sql.ErrNoRows) { return message, ErrChatForbidden }
		if err != nil { return message, err }
	}

	var partyID any
	if input.PartyID != "" {
		partyID = input.PartyID
	}
	var recipientID any
	if input.RecipientUserID > 0 {
		recipientID = input.RecipientUserID
	}
	var conversationID any
	if input.ConversationID > 0 { conversationID = input.ConversationID }
	err = tx.QueryRowContext(ctx, `
        INSERT INTO chat_messages (channel, sender_user_id, sender_character_id, sender_nickname, body, party_id, recipient_user_id, conversation_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, created_at
    `, input.Channel, input.SenderUserID, characterID, nickname, input.Body, partyID, recipientID, conversationID).Scan(&message.ID, &message.CreatedAt)
	if err != nil {
		return message, fmt.Errorf("insert chat message: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return ChatMessage{}, err
	}

	message.CreatedAt = message.CreatedAt.UTC()

	message.Channel = input.Channel
	message.SenderUserID = input.SenderUserID
	if characterID.Valid {
		id := int(characterID.Int64)
		message.SenderCharacterID = &id
	}
	message.SenderNickname = nickname
	message.Body = input.Body
	if input.PartyID != "" {
		id := input.PartyID
		message.PartyID = &id
	}
	if input.RecipientUserID > 0 {
		id := input.RecipientUserID
		message.RecipientUserID = &id
	}
	if input.ConversationID > 0 { id := input.ConversationID; message.ConversationID = &id }
	return message, nil
}

func (s ChatStore) History(ctx context.Context, q ChatHistoryQuery) ([]ChatMessage, error) {
	var sqlQuery string
	args := []any{q.BeforeID, q.Limit}
	switch q.Channel {
	case "MAIN", "TRADE":
		sqlQuery = `SELECT id, channel, sender_user_id, sender_character_id, sender_nickname, body, party_id, recipient_user_id, created_at
            FROM chat_messages WHERE channel=$3 AND ($1=0 OR id<$1) AND created_at >= now()-interval '90 days'
              AND NOT EXISTS (SELECT 1 FROM chat_mutes WHERE viewer_user_id=$4 AND muted_user_id=sender_user_id)
            ORDER BY id DESC LIMIT $2`
		args = append(args, q.Channel, q.ViewerUserID)
	case "GROUP":
		sqlQuery = `SELECT id, channel, sender_user_id, sender_character_id, sender_nickname, body, party_id, recipient_user_id, created_at
            FROM chat_messages WHERE channel='GROUP' AND party_id=$3 AND ($1=0 OR id<$1) AND created_at >= now()-interval '90 days'
              AND NOT EXISTS (SELECT 1 FROM chat_mutes WHERE viewer_user_id=$4 AND muted_user_id=sender_user_id)
            ORDER BY id DESC LIMIT $2`
		args = append(args, q.PartyID, q.ViewerUserID)
	case "PRIVATE":
		var member bool
		err := s.DB.QueryRowContext(ctx, `SELECT true FROM chat_conversation_participants WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL`, q.ConversationID, q.ViewerUserID).Scan(&member)
		if errors.Is(err, sql.ErrNoRows) { return nil, ErrChatForbidden }
		if err != nil { return nil, err }
		sqlQuery = `SELECT id, channel, sender_user_id, sender_character_id, sender_nickname, body, party_id, recipient_user_id, created_at
            FROM chat_messages WHERE channel='PRIVATE' AND conversation_id=$3
              AND ($1=0 OR id<$1)
              AND EXISTS (SELECT 1 FROM chat_conversation_participants WHERE conversation_id=$3 AND user_id=$4 AND left_at IS NULL)
              AND NOT EXISTS (SELECT 1 FROM chat_mutes WHERE viewer_user_id=$4 AND muted_user_id=sender_user_id)
            ORDER BY id DESC LIMIT $2`
		args = append(args, q.ConversationID, q.ViewerUserID)
	default:
		return nil, fmt.Errorf("unsupported chat channel")
	}
	rows, err := s.DB.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]ChatMessage, 0, q.Limit)
	for rows.Next() {
		var m ChatMessage
		var characterID sql.NullInt64
		var partyID sql.NullString
		var recipientID sql.NullInt64
		if err = rows.Scan(&m.ID, &m.Channel, &m.SenderUserID, &characterID, &m.SenderNickname, &m.Body, &partyID, &recipientID, &m.CreatedAt); err != nil {
			return nil, err
		}
		if characterID.Valid {
			id := int(characterID.Int64)
			m.SenderCharacterID = &id
		}
		if partyID.Valid {
			id := partyID.String
			m.PartyID = &id
		}
		if recipientID.Valid {
			id := int(recipientID.Int64)
			m.RecipientUserID = &id
		}
		if q.Channel == "PRIVATE" { id := q.ConversationID; m.ConversationID = &id }
		m.CreatedAt = m.CreatedAt.UTC()
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s ChatStore) PruneExpired(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM chat_messages WHERE channel<>'PRIVATE' AND created_at < now() - interval '90 days'`)
	return err
}
