# Chat foundation (Stage 8.1)

Gameservice owns durable chat messages in `chat_messages`. Existing JWT middleware protects both HTTP operations. `senderUserId`, `senderCharacterId`, `senderNickname`, `partyId`, `createdAt`, and `id` are server supplied. Bodies are plain UTF-8 text; render them as text, never as HTML.

## HTTP contract

- `POST /game/chat/messages` with `Authorization: Bearer <JWT>` and JSON `{"channel":"MAIN|GROUP|TRADE|PRIVATE","body":"...","recipientId":2}`. `recipientId` is required only for PRIVATE. Returns `201` with the saved message. SYSTEM cannot be sent by a client. GROUP resolves Party ID from the existing matchmaking Party service using the authenticated user ID.
- `GET /game/chat/messages?channel=MAIN|GROUP|TRADE|PRIVATE&limit=50&beforeId=123&recipientId=2` with the same JWT. `recipientId` is required only for PRIVATE. Returns `{"messages":[...],"nextBeforeId":123|null}`. Messages are newest first; pass `nextBeforeId` to fetch older messages. Limits are 1–100, default 50. `beforeId` is exclusive. GROUP history requires current Party membership. PRIVATE results are restricted to the authenticated user and the selected peer.
- Message fields: `id`, `channel`, `senderUserId`, optional `senderCharacterId`, `senderNickname`, `body`, optional `partyId`, optional `recipientUserId`, `createdAt` (UTC RFC3339). Use `id` as the stable ordering and deduplication cursor; display the timestamp as `HH:mm:ss` in the selected client locale.
- Error statuses: `400` invalid channel/body/recipient/query, `401` missing or invalid JWT, `403` no current Party membership, `404` missing sender/recipient profile, `429` rate limit, `500` unavailable storage/service.

## Limits and retention

Messages contain 1–1000 Unicode characters after trimming. A sender may submit at most one message per second and 20 per rolling minute across gameservice replicas. History exposes the most recent 90 days. A daily cleanup deletes older rows.

## Stage 8.2 live delivery

Successful `POST /game/chat/messages` persists once and queues a best-effort event on the existing authenticated `/ws?token=<JWT>` connection. The event is `{"type":"chat.message","payload":<saved message>}`; `payload` has exactly the message fields listed above. MAIN and TRADE reach every connected player, PRIVATE reaches both participants and all their tabs, and GROUP reaches only currently verified members of the saved Party. Delivery rechecks current matchmaking Party state and fails closed if the Party changed or matchmaking is unavailable. SYSTEM remains reserved; a future trusted server producer should use a separate `chat.system` event, never a client request.

The in-process socket registry already groups connections by user ID, so a future online list can count distinct IDs rather than tabs. Socket delivery is not guaranteed: on reconnect, fetch the latest authorized history pages until a previously seen message ID is reached, then deduplicate and order by ID. The bounded live queue can drop delivery under overload without losing the persisted message. No new history cursor or acknowledgement protocol is required.

Party mutation routes now require a valid JWT and require the request's acting player/leader ID to equal its JWT user ID. Party state remains in matchmaking memory and resets on service restart. The socket registry is process-local; deploying multiple gameservice replicas would require shared fanout before global live delivery spans replicas.

## Stage 8.4 social chat

`GET /game/chat/online` returns `{players:[{userId,name}],count}` for unique authenticated socket users. The same socket carries `presence.join` with `{userId,name}` and `presence.leave` with `{userId}`. Fetch the snapshot on connect or reconnect; events update it between snapshots. Presence is process-local until a shared cross-replica transport exists.

`GET /game/chat/mutes` returns `{mutedUserIds:[...]}`. `PUT` and `DELETE /game/chat/mutes/{id}` mute or unmute another player for the JWT user. Muting hides that sender's MAIN, TRADE, GROUP, and PRIVATE messages from the viewer in both live delivery and history. It never deletes messages, and senders are not told whether recipients muted them. Friend actions continue to use the existing `/game/friends` and friend-request endpoints.
