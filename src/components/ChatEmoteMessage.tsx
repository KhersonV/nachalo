"use client";

import { memo, useMemo, useState } from "react";
import { splitChatEmotes, type ChatEmote } from "@/utils/chatEmotes";
import styles from "@/styles/GameChat.module.css";

export function EmoteImage({ emote, animate }: { emote: ChatEmote; animate: boolean }) {
    const [failed, setFailed] = useState<string[]>([]);
    const sources = animate ? [emote.animatedSrc, emote.staticSrc] : [emote.staticSrc];
    const src = sources.find((candidate): candidate is string => Boolean(candidate && !failed.includes(candidate)));
    if (!src) return <>{emote.code}</>;
    return <img className={styles.inlineEmote} src={src} width={32} height={32} alt={emote.name} title={emote.name} onError={() => setFailed(previous => [...previous, src])} />;
}

const ChatEmoteMessage = memo(function ChatEmoteMessage({ body, animate }: { body: string; animate: boolean }) {
    const parts = useMemo(() => splitChatEmotes(body), [body]);
    return <span>{parts.map((part, index) => typeof part === "string" ? part : <EmoteImage key={`${part.id}-${index}`} emote={part} animate={animate} />)}</span>;
});

export default ChatEmoteMessage;
