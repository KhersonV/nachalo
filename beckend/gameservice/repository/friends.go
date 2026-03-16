package repository

import (
    "database/sql"
    "fmt"
)

type FriendSummary struct {
    UserID        int    `json:"userId"`
    Name          string `json:"name"`
    Image         string `json:"image"`
    CharacterType string `json:"characterType"`
    Level         int    `json:"level"`
}

type PlayerSearchSummary struct {
    UserID        int    `json:"userId"`
    Name          string `json:"name"`
    Image         string `json:"image"`
    CharacterType string `json:"characterType"`
    Level         int    `json:"level"`
}

type FriendRequestSummary struct {
    UserID        int    `json:"userId"`
    Name          string `json:"name"`
    Image         string `json:"image"`
    CharacterType string `json:"characterType"`
    Level         int    `json:"level"`
    CreatedAt     string `json:"createdAt"`
}

func PlayerExists(userID int) (bool, error) {
    var exists bool
    if err := DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM players WHERE user_id = $1)`, userID).Scan(&exists); err != nil {
        return false, fmt.Errorf("PlayerExists: %w", err)
    }
    return exists, nil
}

func AreFriends(userID int, otherUserID int) (bool, error) {
    var exists bool
    if err := DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1
            FROM player_friends
            WHERE user_id = $1 AND friend_user_id = $2
        )
    `, userID, otherUserID).Scan(&exists); err != nil {
        return false, fmt.Errorf("AreFriends: %w", err)
    }
    return exists, nil
}

func GetFriendRelation(userID int, otherUserID int) (string, error) {
    if userID == otherUserID {
        return "self", nil
    }

    friends, err := AreFriends(userID, otherUserID)
    if err != nil {
        return "none", fmt.Errorf("GetFriendRelation AreFriends: %w", err)
    }
    if friends {
        return "friend", nil
    }

    var outgoing bool
    if err := DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1
            FROM player_friend_requests
            WHERE requester_user_id = $1 AND target_user_id = $2
        )
    `, userID, otherUserID).Scan(&outgoing); err != nil {
        return "none", fmt.Errorf("GetFriendRelation outgoing: %w", err)
    }
    if outgoing {
        return "outgoing", nil
    }

    var incoming bool
    if err := DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1
            FROM player_friend_requests
            WHERE requester_user_id = $1 AND target_user_id = $2
        )
    `, otherUserID, userID).Scan(&incoming); err != nil {
        return "none", fmt.Errorf("GetFriendRelation incoming: %w", err)
    }
    if incoming {
        return "incoming", nil
    }

    return "none", nil
}

func CreateFriendRequest(requesterUserID int, targetUserID int) error {
    relation, err := GetFriendRelation(requesterUserID, targetUserID)
    if err != nil {
        return fmt.Errorf("CreateFriendRequest relation: %w", err)
    }
    if relation == "self" || relation == "friend" || relation == "outgoing" {
        return nil
    }

    if relation == "incoming" {
        // If there is an opposite incoming request, auto-accept into friendship.
        return AcceptFriendRequest(requesterUserID, targetUserID)
    }

    if _, err := DB.Exec(`
        INSERT INTO player_friend_requests (requester_user_id, target_user_id)
        VALUES ($1, $2)
        ON CONFLICT (requester_user_id, target_user_id) DO NOTHING
    `, requesterUserID, targetUserID); err != nil {
        return fmt.Errorf("CreateFriendRequest insert: %w", err)
    }
    return nil
}

func ListIncomingFriendRequests(userID int) ([]FriendRequestSummary, error) {
    rows, err := DB.Query(`
        SELECT p.user_id, p.name, p.image, p.character_type, p.level, pfr.created_at::text
        FROM player_friend_requests pfr
        JOIN players p ON p.user_id = pfr.requester_user_id
        WHERE pfr.target_user_id = $1
        ORDER BY pfr.created_at DESC
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("ListIncomingFriendRequests query: %w", err)
    }
    defer rows.Close()

    result := make([]FriendRequestSummary, 0)
    for rows.Next() {
        var item FriendRequestSummary
        var name sql.NullString
        var image sql.NullString
        var characterType sql.NullString
        if err := rows.Scan(&item.UserID, &name, &image, &characterType, &item.Level, &item.CreatedAt); err != nil {
            return nil, fmt.Errorf("ListIncomingFriendRequests scan: %w", err)
        }
        item.Name = name.String
        if item.Name == "" {
            item.Name = "Player"
        }
        item.Image = image.String
        if item.Image == "" {
            item.Image = "/ranger/ranger.webp"
        }
        item.CharacterType = characterType.String
        if item.CharacterType == "" {
            item.CharacterType = "adventurer"
        }
        result = append(result, item)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("ListIncomingFriendRequests rows: %w", err)
    }

    return result, nil
}

func ListOutgoingFriendRequests(userID int) ([]FriendRequestSummary, error) {
    rows, err := DB.Query(`
        SELECT p.user_id, p.name, p.image, p.character_type, p.level, pfr.created_at::text
        FROM player_friend_requests pfr
        JOIN players p ON p.user_id = pfr.target_user_id
        WHERE pfr.requester_user_id = $1
        ORDER BY pfr.created_at DESC
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("ListOutgoingFriendRequests query: %w", err)
    }
    defer rows.Close()

    result := make([]FriendRequestSummary, 0)
    for rows.Next() {
        var item FriendRequestSummary
        var name sql.NullString
        var image sql.NullString
        var characterType sql.NullString
        if err := rows.Scan(&item.UserID, &name, &image, &characterType, &item.Level, &item.CreatedAt); err != nil {
            return nil, fmt.Errorf("ListOutgoingFriendRequests scan: %w", err)
        }
        item.Name = name.String
        if item.Name == "" {
            item.Name = "Player"
        }
        item.Image = image.String
        if item.Image == "" {
            item.Image = "/ranger/ranger.webp"
        }
        item.CharacterType = characterType.String
        if item.CharacterType == "" {
            item.CharacterType = "adventurer"
        }
        result = append(result, item)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("ListOutgoingFriendRequests rows: %w", err)
    }

    return result, nil
}

func CancelOutgoingFriendRequest(requesterUserID int, targetUserID int) error {
    if _, err := DB.Exec(`
        DELETE FROM player_friend_requests
        WHERE requester_user_id = $1 AND target_user_id = $2
    `, requesterUserID, targetUserID); err != nil {
        return fmt.Errorf("CancelOutgoingFriendRequest delete request: %w", err)
    }
    return nil
}

func AcceptFriendRequest(targetUserID int, requesterUserID int) error {
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("AcceptFriendRequest begin tx: %w", err)
    }
    defer tx.Rollback()

    if _, err := tx.Exec(`
        DELETE FROM player_friend_requests
        WHERE requester_user_id = $1 AND target_user_id = $2
    `, requesterUserID, targetUserID); err != nil {
        return fmt.Errorf("AcceptFriendRequest delete request: %w", err)
    }

    if _, err := tx.Exec(`
        INSERT INTO player_friends (user_id, friend_user_id)
        VALUES ($1, $2), ($2, $1)
        ON CONFLICT (user_id, friend_user_id) DO NOTHING
    `, targetUserID, requesterUserID); err != nil {
        return fmt.Errorf("AcceptFriendRequest insert friends: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("AcceptFriendRequest commit: %w", err)
    }
    return nil
}

func RejectFriendRequest(targetUserID int, requesterUserID int) error {
    if _, err := DB.Exec(`
        DELETE FROM player_friend_requests
        WHERE requester_user_id = $1 AND target_user_id = $2
    `, requesterUserID, targetUserID); err != nil {
        return fmt.Errorf("RejectFriendRequest delete request: %w", err)
    }
    return nil
}

func AddFriendBidirectional(userID int, friendUserID int) error {
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("AddFriendBidirectional begin tx: %w", err)
    }
    defer tx.Rollback()

    if _, err := tx.Exec(`
        INSERT INTO player_friends (user_id, friend_user_id)
        VALUES ($1, $2)
        ON CONFLICT (user_id, friend_user_id) DO NOTHING
    `, userID, friendUserID); err != nil {
        return fmt.Errorf("AddFriendBidirectional insert direct: %w", err)
    }

    if _, err := tx.Exec(`
        INSERT INTO player_friends (user_id, friend_user_id)
        VALUES ($1, $2)
        ON CONFLICT (user_id, friend_user_id) DO NOTHING
    `, friendUserID, userID); err != nil {
        return fmt.Errorf("AddFriendBidirectional insert reverse: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("AddFriendBidirectional commit: %w", err)
    }

    return nil
}

func RemoveFriendBidirectional(userID int, friendUserID int) error {
    tx, err := DB.Begin()
    if err != nil {
        return fmt.Errorf("RemoveFriendBidirectional begin tx: %w", err)
    }
    defer tx.Rollback()

    if _, err := tx.Exec(`
        DELETE FROM player_friends
        WHERE user_id = $1 AND friend_user_id = $2
    `, userID, friendUserID); err != nil {
        return fmt.Errorf("RemoveFriendBidirectional delete direct: %w", err)
    }

    if _, err := tx.Exec(`
        DELETE FROM player_friends
        WHERE user_id = $1 AND friend_user_id = $2
    `, friendUserID, userID); err != nil {
        return fmt.Errorf("RemoveFriendBidirectional delete reverse: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("RemoveFriendBidirectional commit: %w", err)
    }

    return nil
}

func SearchPlayersByName(query string, limit int) ([]PlayerSearchSummary, error) {
    if limit <= 0 {
        limit = 20
    }
    if limit > 50 {
        limit = 50
    }

    rows, err := DB.Query(`
        SELECT user_id, name, image, character_type, level
        FROM players
        WHERE LOWER(name) LIKE LOWER('%' || $1 || '%')
        ORDER BY name ASC
        LIMIT $2
    `, query, limit)
    if err != nil {
        return nil, fmt.Errorf("SearchPlayersByName query: %w", err)
    }
    defer rows.Close()

    result := make([]PlayerSearchSummary, 0)
    for rows.Next() {
        var item PlayerSearchSummary
        var name sql.NullString
        var image sql.NullString
        var characterType sql.NullString
        if err := rows.Scan(&item.UserID, &name, &image, &characterType, &item.Level); err != nil {
            return nil, fmt.Errorf("SearchPlayersByName scan: %w", err)
        }
        item.Name = name.String
        if item.Name == "" {
            item.Name = "Player"
        }
        item.Image = image.String
        if item.Image == "" {
            item.Image = "/ranger/ranger.webp"
        }
        item.CharacterType = characterType.String
        if item.CharacterType == "" {
            item.CharacterType = "adventurer"
        }
        result = append(result, item)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("SearchPlayersByName rows: %w", err)
    }

    return result, nil
}

func ListFriends(userID int) ([]FriendSummary, error) {
    rows, err := DB.Query(`
        SELECT p.user_id, p.name, p.image, p.character_type, p.level
        FROM player_friends pf
        JOIN players p ON p.user_id = pf.friend_user_id
        WHERE pf.user_id = $1
        ORDER BY p.name ASC
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("ListFriends query: %w", err)
    }
    defer rows.Close()

    result := make([]FriendSummary, 0)
    for rows.Next() {
        var item FriendSummary
        var name sql.NullString
        var image sql.NullString
        var characterType sql.NullString
        if err := rows.Scan(&item.UserID, &name, &image, &characterType, &item.Level); err != nil {
            return nil, fmt.Errorf("ListFriends scan: %w", err)
        }
        item.Name = name.String
        if item.Name == "" {
            item.Name = "Player"
        }
        item.Image = image.String
        if item.Image == "" {
            item.Image = "/ranger/ranger.webp"
        }
        item.CharacterType = characterType.String
        if item.CharacterType == "" {
            item.CharacterType = "adventurer"
        }
        result = append(result, item)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("ListFriends rows: %w", err)
    }

    return result, nil
}
