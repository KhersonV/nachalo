package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

type ChatOnlinePlayer struct {
	UserID int    `json:"userId"`
	Name   string `json:"name"`
}

func OnlineChatPlayers(ctx context.Context, ids []int) ([]ChatOnlinePlayer, error) {
	players := make([]ChatOnlinePlayer, 0)
	if len(ids) == 0 {
		return players, nil
	}
	rows, err := DB.QueryContext(ctx, `SELECT user_id, name FROM player_profiles WHERE user_id = ANY($1) ORDER BY lower(name), user_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var player ChatOnlinePlayer
		if err := rows.Scan(&player.UserID, &player.Name); err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	return players, rows.Err()
}

func OnlineChatPlayer(ctx context.Context, userID int) (ChatOnlinePlayer, error) {
	var player ChatOnlinePlayer
	err := DB.QueryRowContext(ctx, `SELECT user_id, name FROM player_profiles WHERE user_id=$1`, userID).Scan(&player.UserID, &player.Name)
	return player, err
}

func ListChatMutes(ctx context.Context, viewerID int) ([]int, error) {
	rows, err := DB.QueryContext(ctx, `SELECT muted_user_id FROM chat_mutes WHERE viewer_user_id=$1 ORDER BY muted_user_id`, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func SetChatMute(ctx context.Context, viewerID, mutedID int, muted bool) error {
	if viewerID <= 0 || mutedID <= 0 || viewerID == mutedID {
		return errors.New("invalid chat mute target")
	}
	if muted {
		var exists bool
		if err := DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM player_profiles WHERE user_id=$1)`, mutedID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
		_, err := DB.ExecContext(ctx, `INSERT INTO chat_mutes(viewer_user_id,muted_user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, viewerID, mutedID)
		return err
	}
	_, err := DB.ExecContext(ctx, `DELETE FROM chat_mutes WHERE viewer_user_id=$1 AND muted_user_id=$2`, viewerID, mutedID)
	return err
}

func MutedChatViewers(ctx context.Context, senderID int, viewerIDs []int) (map[int]struct{}, error) {
	muted := make(map[int]struct{})
	if senderID <= 0 || len(viewerIDs) == 0 {
		return muted, nil
	}
	rows, err := DB.QueryContext(ctx, `SELECT viewer_user_id FROM chat_mutes WHERE muted_user_id=$1 AND viewer_user_id=ANY($2)`, senderID, pq.Array(viewerIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		muted[id] = struct{}{}
	}
	return muted, rows.Err()
}
