export type ChatEmoteCategory = "classic" | "fantasy";

export type ChatEmote = {
    id: string;
    code: `:${string}:`;
    name: string;
    category: ChatEmoteCategory;
    animatedSrc: string;
    staticSrc?: string;
};

// Add entries only after their local artwork exists in public/emotes/.
export const chatEmotes: readonly ChatEmote[] = [];

const byCode = new Map<string, ChatEmote>(chatEmotes.map(emote => [emote.code, emote]));
const codePattern = /:[A-Za-z][A-Za-z0-9_]*:/g;

export function splitChatEmotes(body: string): (string | ChatEmote)[] {
    if (!chatEmotes.length) return [body];
    const parts: (string | ChatEmote)[] = [];
    let cursor = 0;
    for (const match of body.matchAll(codePattern)) {
        const emote = byCode.get(match[0]);
        if (!emote) continue;
        if (match.index > cursor) parts.push(body.slice(cursor, match.index));
        parts.push(emote);
        cursor = match.index + match[0].length;
    }
    if (cursor < body.length) parts.push(body.slice(cursor));
    return parts.length ? parts : [body];
}
