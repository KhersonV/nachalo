package repository

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "time"
)

type ChatParticipant struct {
    UserID int `json:"userId"`
    Name string `json:"name"`
}

type ChatConversation struct {
    ID int64 `json:"id"`
    Kind string `json:"kind"`
    CreatedAt time.Time `json:"createdAt"`
    Participants []ChatParticipant `json:"participants"`
    UnreadCount int `json:"unreadCount"`
}

func (s ChatStore) CreateConversation(ctx context.Context, userID int, recipients []int) (ChatConversation, error) {
    var result ChatConversation
    if len(recipients) < 1 || len(recipients) > 49 { return result, fmt.Errorf("select 1–49 recipients") }
    seen := map[int]bool{userID: true}
    for _, id := range recipients {
        if id <= 0 || seen[id] { return result, fmt.Errorf("invalid or duplicate participant") }
        seen[id] = true
    }
    tx, err := s.DB.BeginTx(ctx, nil)
    if err != nil { return result, err }
    defer tx.Rollback()
    for id := range seen {
        var exists bool
        if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM player_profiles WHERE user_id=$1)`, id).Scan(&exists); err != nil { return result, err }
        if !exists { return result, ErrChatRecipientNotFound }
    }
    if len(recipients) == 1 {
        low, high := userID, recipients[0]
        if low > high { low, high = high, low }
        err = tx.QueryRowContext(ctx, `INSERT INTO chat_conversations (kind,direct_low,direct_high) VALUES ('DIRECT',$1,$2)
            ON CONFLICT (direct_low,direct_high) DO UPDATE SET direct_low=EXCLUDED.direct_low
            RETURNING id,kind,created_at`, low, high).Scan(&result.ID, &result.Kind, &result.CreatedAt)
    } else {
        err = tx.QueryRowContext(ctx, `INSERT INTO chat_conversations (kind) VALUES ('MULTI') RETURNING id,kind,created_at`).Scan(&result.ID, &result.Kind, &result.CreatedAt)
    }
    if err != nil { return result, err }
    for id := range seen {
        _, err = tx.ExecContext(ctx, `INSERT INTO chat_conversation_participants (conversation_id,user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, result.ID, id)
        if err != nil { return result, err }
    }
    if err = tx.Commit(); err != nil { return result, err }
    conversations, err := s.ListConversations(ctx, userID)
    if err != nil { return result, err }
    for _, conversation := range conversations { if conversation.ID == result.ID { return conversation, nil } }
    return result, ErrChatForbidden
}

func (s ChatStore) ListConversations(ctx context.Context, userID int) ([]ChatConversation, error) {
    rows, err := s.DB.QueryContext(ctx, `
        SELECT c.id,c.kind,c.created_at,
          COALESCE((SELECT json_agg(json_build_object('userId',p.user_id,'name',LEFT(COALESCE(NULLIF(btrim(profile.name),''),'Player'),100)) ORDER BY p.user_id)
            FROM chat_conversation_participants p JOIN player_profiles profile ON profile.user_id=p.user_id
            WHERE p.conversation_id=c.id AND p.left_at IS NULL),'[]'::json),
          (SELECT count(*) FROM chat_messages m WHERE m.channel='PRIVATE' AND m.conversation_id=c.id AND m.id>mine.read_message_id
            AND m.sender_user_id<>$1 AND NOT EXISTS (SELECT 1 FROM chat_mutes mute WHERE mute.viewer_user_id=$1 AND mute.muted_user_id=m.sender_user_id))
        FROM chat_conversations c JOIN chat_conversation_participants mine ON mine.conversation_id=c.id
        WHERE mine.user_id=$1 AND mine.left_at IS NULL ORDER BY c.id DESC`, userID)
    if err != nil { return nil, err }
    defer rows.Close()
    result := make([]ChatConversation, 0)
    for rows.Next() {
        var c ChatConversation
        var members []byte
        if err = rows.Scan(&c.ID,&c.Kind,&c.CreatedAt,&members,&c.UnreadCount); err != nil { return nil, err }
        if err = json.Unmarshal(members,&c.Participants); err != nil { return nil, err }
        result = append(result,c)
    }
    return result, rows.Err()
}

func (s ChatStore) MarkConversationRead(ctx context.Context, userID int, conversationID int64) error {
    result, err := s.DB.ExecContext(ctx, `UPDATE chat_conversation_participants p SET read_message_id=GREATEST(p.read_message_id,
        COALESCE((SELECT max(id) FROM chat_messages WHERE channel='PRIVATE' AND conversation_id=$1),0))
        WHERE p.conversation_id=$1 AND p.user_id=$2 AND p.left_at IS NULL`, conversationID,userID)
    if err != nil { return err }
    count, err := result.RowsAffected()
    if err != nil { return err }
    if count == 0 { return ErrChatForbidden }
    return nil
}

func (s ChatStore) LeaveConversation(ctx context.Context, userID int, conversationID int64) error {
    tx, err := s.DB.BeginTx(ctx,nil)
    if err != nil { return err }
    defer tx.Rollback()
    var kind string
    err = tx.QueryRowContext(ctx, `SELECT kind FROM chat_conversations WHERE id=$1 FOR UPDATE`,conversationID).Scan(&kind)
    if errors.Is(err,sql.ErrNoRows) { return ErrChatConversationNotFound }
    if err != nil { return err }
    if kind != "MULTI" { return ErrChatForbidden }
    result, err := tx.ExecContext(ctx, `UPDATE chat_conversation_participants SET left_at=now() WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL`,conversationID,userID)
    if err != nil { return err }
    count, err := result.RowsAffected()
    if err != nil { return err }
    if count == 0 { return ErrChatForbidden }
    return tx.Commit()
}

func ActiveChatConversationParticipants(ctx context.Context, conversationID int64) ([]int,error) {
    rows, err := DB.QueryContext(ctx, `SELECT user_id FROM chat_conversation_participants WHERE conversation_id=$1 AND left_at IS NULL`,conversationID)
    if err != nil { return nil,err }
    defer rows.Close()
    ids := []int{}
    for rows.Next() { var id int; if err=rows.Scan(&id); err!=nil { return nil,err }; ids=append(ids,id) }
    return ids,rows.Err()
}

func (s ChatStore) SearchChatPlayers(ctx context.Context, viewerID int, term string) ([]ChatParticipant,error) {
    rows,err := s.DB.QueryContext(ctx, `SELECT user_id,LEFT(COALESCE(NULLIF(btrim(name),''),'Player'),100) FROM player_profiles
        WHERE user_id<>$1 AND name ILIKE '%' || $2 || '%' ORDER BY name,user_id LIMIT 30`,viewerID,term)
    if err!=nil { return nil,err }
    defer rows.Close()
    players:=[]ChatParticipant{}
    for rows.Next() { var p ChatParticipant; if err=rows.Scan(&p.UserID,&p.Name); err!=nil { return nil,err }; players=append(players,p) }
    return players,rows.Err()
}
