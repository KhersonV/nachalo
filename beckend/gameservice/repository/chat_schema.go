package repository

import "fmt"

// Chat follows the existing gameservice startup migration convention.
const chatSchema = `
CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGSERIAL PRIMARY KEY,
    channel TEXT NOT NULL CHECK (channel IN ('MAIN', 'GROUP', 'TRADE', 'PRIVATE', 'SYSTEM')),
    sender_user_id INTEGER REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    sender_character_id INTEGER REFERENCES player_characters(id) ON DELETE SET NULL,
    sender_nickname TEXT NOT NULL CHECK (char_length(sender_nickname) BETWEEN 1 AND 100),
    body TEXT NOT NULL CHECK (char_length(btrim(body)) BETWEEN 1 AND 1000),
    party_id TEXT,
    recipient_user_id INTEGER REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (channel IN ('MAIN', 'TRADE') AND sender_user_id IS NOT NULL AND party_id IS NULL AND recipient_user_id IS NULL) OR
        (channel = 'GROUP' AND sender_user_id IS NOT NULL AND NULLIF(btrim(party_id), '') IS NOT NULL AND recipient_user_id IS NULL) OR
        (channel = 'PRIVATE' AND sender_user_id IS NOT NULL AND recipient_user_id IS NOT NULL AND sender_user_id <> recipient_user_id AND party_id IS NULL) OR
        (channel = 'SYSTEM' AND sender_user_id IS NULL AND sender_character_id IS NULL AND party_id IS NULL AND recipient_user_id IS NULL)
    )
);
CREATE INDEX IF NOT EXISTS chat_public_page_idx ON chat_messages (channel, id DESC) WHERE channel IN ('MAIN', 'TRADE');
CREATE INDEX IF NOT EXISTS chat_group_page_idx ON chat_messages (party_id, id DESC) WHERE channel = 'GROUP';
CREATE INDEX IF NOT EXISTS chat_private_pair_page_idx ON chat_messages (LEAST(sender_user_id, recipient_user_id), GREATEST(sender_user_id, recipient_user_id), id DESC) WHERE channel = 'PRIVATE';
CREATE INDEX IF NOT EXISTS chat_sender_rate_idx ON chat_messages (sender_user_id, created_at DESC) WHERE sender_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS chat_retention_idx ON chat_messages (created_at);
CREATE TABLE IF NOT EXISTS chat_mutes (
    viewer_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    muted_user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (viewer_user_id, muted_user_id),
    CHECK (viewer_user_id <> muted_user_id)
);
CREATE INDEX IF NOT EXISTS chat_mutes_sender_idx ON chat_mutes (muted_user_id, viewer_user_id);
`

func EnsureChatSchema() error {
	if _, err := DB.Exec(chatSchema); err != nil {
		return fmt.Errorf("create chat schema: %w", err)
	}
	if _, err := DB.Exec(chatConversationSchema); err != nil {
		return fmt.Errorf("create private conversations: %w", err)
	}
	return nil
}

const chatConversationSchema = `
CREATE TABLE IF NOT EXISTS chat_conversations (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('DIRECT', 'MULTI')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    direct_low INTEGER REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    direct_high INTEGER REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    CHECK ((kind='DIRECT' AND direct_low IS NOT NULL AND direct_high IS NOT NULL AND direct_low<direct_high) OR
           (kind='MULTI' AND direct_low IS NULL AND direct_high IS NULL)),
    UNIQUE (direct_low, direct_high)
);
CREATE TABLE IF NOT EXISTS chat_conversation_participants (
    conversation_id BIGINT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES player_profiles(user_id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    read_message_id BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX IF NOT EXISTS chat_participant_user_idx ON chat_conversation_participants (user_id, conversation_id) WHERE left_at IS NULL;
DO $$
DECLARE old_check TEXT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='chat_messages' AND column_name='conversation_id') THEN
        DELETE FROM chat_messages WHERE channel='PRIVATE';
        ALTER TABLE chat_messages ADD COLUMN conversation_id BIGINT REFERENCES chat_conversations(id) ON DELETE CASCADE;
        SELECT conname INTO old_check FROM pg_constraint
        WHERE conrelid='chat_messages'::regclass AND contype='c'
          AND pg_get_constraintdef(oid) LIKE '%recipient_user_id%'
          AND pg_get_constraintdef(oid) LIKE '%PRIVATE%'
          AND pg_get_constraintdef(oid) LIKE '%party_id%' LIMIT 1;
        IF old_check IS NOT NULL THEN
            EXECUTE format('ALTER TABLE chat_messages DROP CONSTRAINT %I', old_check);
        END IF;
        ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_channel_shape CHECK (
            (channel IN ('MAIN','TRADE') AND sender_user_id IS NOT NULL AND party_id IS NULL AND recipient_user_id IS NULL AND conversation_id IS NULL) OR
            (channel='GROUP' AND sender_user_id IS NOT NULL AND NULLIF(btrim(party_id),'') IS NOT NULL AND recipient_user_id IS NULL AND conversation_id IS NULL) OR
            (channel='PRIVATE' AND sender_user_id IS NOT NULL AND party_id IS NULL AND recipient_user_id IS NULL AND conversation_id IS NOT NULL) OR
            (channel='SYSTEM' AND sender_user_id IS NULL AND sender_character_id IS NULL AND party_id IS NULL AND recipient_user_id IS NULL AND conversation_id IS NULL)
        );
    END IF;
END $$;
DROP INDEX IF EXISTS chat_private_pair_page_idx;
CREATE INDEX IF NOT EXISTS chat_private_conversation_page_idx ON chat_messages (conversation_id, id DESC) WHERE channel='PRIVATE';
`
