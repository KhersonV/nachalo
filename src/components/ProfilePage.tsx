"use client";

import UiText from "@/components/UiText";

import React, { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { API_BASE } from "@/utils/serviceUrls";
import { useAuth } from "@/contexts/AuthContext";
import { getHeroPortrait } from "@/features/heroes/heroArtwork";
import { matchesProfileView, ownedHeroes, performConfirmedSocialAction, profileViewId, relationActions, xpProgress, type Relation, type RelationAction, type SocialTab } from "@/features/profile/profileView";
import { normalizeAvatarPath } from "@/utils/normalizeAvatarPath";
import styles from "@/styles/ProfilePage.module.css";
import { useGameLanguage } from "@/hooks/useGameLanguage";
import { uiText } from "@/i18n/uiText";

type Activity = "in_match" | "in_lobby" | "offline";
type Player = {
    userId: number;
    name: string;
    image: string;
    activityStatus: Activity;
    characterType: string;
    level: number;
    experience: number;
    maxExperience: number;
    balance?: number;
};
type Progress = { matchesPlayed: number; wins: number; losses: number; winRate: number };
type OwnProfile = { player: Player; progress: Progress | null };
type PublicProfile = { player: Player; progress: Progress | null; friendRelation: Relation };
type Hero = { id: string; displayName: string; image?: string; owned: boolean; enabled: boolean; active: boolean; sortOrder: number };
type SocialPlayer = { userId: number; name: string; image: string; activityStatus: Activity; characterType: string; level: number };
type SearchPlayer = SocialPlayer & { friendRelation: Relation };
type SocialData = { friends: SocialPlayer[]; incoming: SocialPlayer[]; outgoing: SocialPlayer[] };
type ResponseData<T> = { data: T };

const TABS: { key: SocialTab; label: string }[] = [
    { key: "friends", label: "Friends" },
    { key: "incoming", label: "Incoming" },
    { key: "outgoing", label: "Outgoing" },
    { key: "search", label: "Search" },
];

async function readApi<T>(path: string, token: string): Promise<T> {
    const response = await fetch(API_BASE + path, { headers: { Authorization: "Bearer " + token }, cache: "no-store" });
    if (!response.ok) throw new Error("Could not load data (" + response.status + ")");
    const body = await response.json() as ResponseData<T>;
    if (!body || !("data" in body)) throw new Error("The server returned an incomplete response");
    return body.data;
}

async function writeApi(path: string, token: string, method: "POST" | "DELETE" | "PATCH", body?: unknown): Promise<void> {
    const response = await fetch(API_BASE + path, {
        method,
        headers: { Authorization: "Bearer " + token, ...(body ? { "Content-Type": "application/json" } : {}) },
        ...(body ? { body: JSON.stringify(body) } : {}),
    });
    if (!response.ok) throw new Error((await response.text()) || "The action failed");
}

function activityLabel(status: Activity): string {
    return status === "in_match" ? "In Match" : status === "in_lobby" ? "In Lobby" : "Offline";
}

function PlayerRow({ player, children }: { player: SocialPlayer; children?: React.ReactNode }) {
    return <div className={styles.playerRow}>
        <img className={styles.rowAvatar} src={normalizeAvatarPath(player.image)} alt="" />
        <div className={styles.playerMeta}><strong>{player.name}</strong><span><UiText text={activityLabel(player.activityStatus)} /> · <UiText text={player.characterType.charAt(0).toUpperCase() + player.characterType.slice(1)} /> <UiText text="· Hero Lv." /> {player.level}</span></div>
        <div className={styles.rowActions}><Link href={"/profile?view=" + player.userId}><UiText text="View Profile" /></Link>{children}</div>
    </div>;
}

function Identity({ player, own }: { player: Player; own: boolean }) {
    const progress = xpProgress(player.experience, player.maxExperience);
    return <section className={styles.identity} aria-label="Player identity">
        <div className={styles.accountIdentity}>
            <img className={styles.avatar} src={normalizeAvatarPath(player.image)} alt={player.name + " avatar"} />
            <div className={styles.accountCopy}>
                <span className={styles.eyebrow}><UiText text={own ? "PLAYER IDENTITY" : "PLAYER PROFILE"} /></span>
                <h1>{player.name}</h1>
                <span className={styles.activity} data-status={player.activityStatus}><UiText text={activityLabel(player.activityStatus)} /></span>
                {own && Number.isFinite(player.balance) && <span className={styles.accountBalance}>{player.balance!.toLocaleString()} <UiText text="Gold" /></span>}
            </div>
        </div>
        <div className={styles.heroProgress}>
            <div className={styles.progressHeading}><span><UiText text="Active hero progress" /></span><strong><UiText text="Level" /> {player.level}</strong></div>
            <span className={styles.xpText}>{Number.isFinite(player.experience) ? player.experience.toLocaleString() : "—"}{Number.isFinite(player.maxExperience) && player.maxExperience > 0 ? " / " + player.maxExperience.toLocaleString() : ""} <UiText text="XP" /></span>
            {progress !== null && <div className={styles.expTrack} role="progressbar" aria-label="Active hero experience" aria-valuenow={player.experience} aria-valuemin={0} aria-valuemax={player.maxExperience}><div style={{ width: progress + "%" }} /></div>}
        </div>
        <div className={styles.activeHero}>
            <img src={getHeroPortrait(player.characterType)} alt="" />
            <div><span className={styles.eyebrow}><UiText text="ACTIVE HERO" /></span><strong><UiText text={player.characterType.charAt(0).toUpperCase() + player.characterType.slice(1)} /></strong><small><UiText text="Hero Level" /> {player.level}</small></div>
        </div>
    </section>;
}

function Overview({ progress, own }: { progress: Progress | null; own: boolean }) {
    const values = progress && [progress.matchesPlayed, progress.wins, progress.losses, progress.winRate].every(Number.isFinite)
        ? [{ label: "Matches", value: progress.matchesPlayed }, { label: "Wins", value: progress.wins }, { label: "Losses", value: progress.losses }, { label: "Win Rate", value: progress.winRate + "%" }]
        : null;
    return <section className={styles.panel} aria-label="Player overview">
        <div className={styles.sectionHead}><div><span className={styles.eyebrow}><UiText text="PLAYER OVERVIEW" /></span><h2><UiText text="Match record" /></h2></div>{own && <Link href="/matches"><UiText text="View Match History ↗" /></Link>}</div>
        {values ? <div className={styles.stats}>{values.map(item => <div className={styles.stat} key={item.label}><span><UiText text={item.label} /></span><strong>{item.value}</strong></div>)}</div> : <p className={styles.empty}><UiText text="Match statistics are unavailable." /></p>}
    </section>;
}

export default function ProfilePage() {
    const { language } = useGameLanguage();
    const t = (source: string) => uiText(language, source);
    const router = useRouter();
    const params = useSearchParams();
    const { user } = useAuth();
    const token = user?.token ?? "";
    const viewId = profileViewId(params.get("view"));
    const ownMode = viewId === null;
    const [own, setOwn] = useState<{ token: string; data: OwnProfile } | null>(null);
    const [ownError, setOwnError] = useState("");
    const [publicProfile, setPublicProfile] = useState<{ token: string; data: PublicProfile } | null>(null);
    const [publicError, setPublicError] = useState("");
    const [heroes, setHeroes] = useState<{ token: string; data: Hero[] } | null>(null);
    const [heroesError, setHeroesError] = useState("");
    const [social, setSocial] = useState<{ token: string; data: SocialData } | null>(null);
    const [socialError, setSocialError] = useState("");
    const [tab, setTab] = useState<SocialTab>("friends");
    const [editOpen, setEditOpen] = useState(false);
    const [editName, setEditName] = useState("");
    const [editImage, setEditImage] = useState("");
    const [saving, setSaving] = useState(false);
    const [editError, setEditError] = useState("");
    const [editNotice, setEditNotice] = useState("");
    const [searchQuery, setSearchQuery] = useState("");
    const [searchedQuery, setSearchedQuery] = useState("");
    const [searchResults, setSearchResults] = useState<SearchPlayer[]>([]);
    const [searching, setSearching] = useState(false);
    const [searchError, setSearchError] = useState("");
    const [actionKey, setActionKey] = useState("");
    const [actionError, setActionError] = useState("");
    const [actionNotice, setActionNotice] = useState("");
    const publicRequest = useRef(0);
    const socialRequest = useRef(0);
    const searchRequest = useRef(0);
    const scopeKey = token + ":" + (viewId ?? "self");
    const scopeRef = useRef(scopeKey);
    scopeRef.current = scopeKey;

    useEffect(() => {
        setActionKey(""); setActionError(""); setActionNotice("");
        setSearchQuery(""); setSearchedQuery(""); setSearchResults([]); setSearchError("");
        setEditOpen(false); setEditError(""); setEditNotice("");
    }, [scopeKey]);

    useEffect(() => {
        setOwnError(""); setHeroesError(""); setSocialError("");
    }, [token]);

    const refreshOwn = useCallback(async () => {
        if (!token) return;
        const data = await readApi<OwnProfile>("/game/profile", token);
        setOwn({ token, data });
        setOwnError("");
    }, [token]);

    const refreshHeroes = useCallback(async () => {
        if (!token) return;
        const result = await fetch(API_BASE + "/game/heroes", { headers: { Authorization: "Bearer " + token }, cache: "no-store" });
        if (!result.ok) throw new Error("Could not load heroes (" + result.status + ")");
        const response = await result.json() as { heroes: Hero[] };
        if (!Array.isArray(response.heroes)) throw new Error("Hero roster is unavailable");
        setHeroes({ token, data: response.heroes });
        setHeroesError("");
    }, [token]);

    const refreshSocial = useCallback(async () => {
        if (!token) return;
        const requestId = ++socialRequest.current;
        const [friends, incoming, outgoing] = await Promise.all([
            readApi<SocialPlayer[]>("/game/friends", token),
            readApi<SocialPlayer[]>("/game/friends/requests/incoming", token),
            readApi<SocialPlayer[]>("/game/friends/requests/outgoing", token),
        ]);
        if (requestId !== socialRequest.current) return;
        setSocial({ token, data: { friends, incoming, outgoing } });
        setSocialError("");
    }, [token]);

    const refreshPublic = useCallback(async (id: number) => {
        if (!token) return;
        const requestId = ++publicRequest.current;
        const data = await readApi<PublicProfile>("/game/profile/" + id, token);
        if (requestId !== publicRequest.current || data.player.userId !== id) return;
        setPublicProfile({ token, data });
        setPublicError("");
    }, [token]);

    const runSearch = useCallback(async (query: string) => {
        if (!token) return;
        const q = query.trim();
        if (q.length < 2) { setSearchError("Enter at least 2 characters."); return; }
        const requestId = ++searchRequest.current;
        setSearching(true);
        setSearchError("");
        try {
            const data = await readApi<SearchPlayer[]>("/game/players/search?q=" + encodeURIComponent(q), token);
            if (requestId !== searchRequest.current) return;
            setSearchResults(data);
            setSearchedQuery(q);
        } catch (error) {
            if (requestId === searchRequest.current) setSearchError(error instanceof Error ? error.message : "Search failed");
        } finally {
            if (requestId === searchRequest.current) setSearching(false);
        }
    }, [token]);

    useEffect(() => {
        if (!token || !ownMode) return;
        void refreshOwn().catch(error => setOwnError(error instanceof Error ? error.message : "Profile unavailable"));
        void refreshHeroes().catch(error => setHeroesError(error instanceof Error ? error.message : "Heroes unavailable"));
        void refreshSocial().catch(error => setSocialError(error instanceof Error ? error.message : "Social data unavailable"));
        const interval = window.setInterval(() => {
            void refreshOwn().catch(() => undefined);
            void refreshHeroes().catch(() => undefined);
            void refreshSocial().catch(() => undefined);
        }, 8000);
        return () => { window.clearInterval(interval); ++socialRequest.current; ++searchRequest.current; };
    }, [token, ownMode, refreshOwn, refreshHeroes, refreshSocial]);

    useEffect(() => {
        if (!token || viewId === null) return;
        setPublicError("");
        setPublicProfile(null);
        void refreshPublic(viewId).catch(error => setPublicError(error instanceof Error ? error.message : "Player profile unavailable"));
        return () => { ++publicRequest.current; };
    }, [token, viewId, refreshPublic]);

    const saveIdentity = async (event: React.FormEvent) => {
        event.preventDefault();
        if (!token || !ownMode) return;
        setSaving(true); setEditError(""); setEditNotice("");
        try {
            const response = await fetch(API_BASE + "/game/profile", {
                method: "PATCH",
                headers: { Authorization: "Bearer " + token, "Content-Type": "application/json" },
                body: JSON.stringify({ name: editName, image: editImage }),
            });
            if (!response.ok) throw new Error((await response.text()) || "Profile save failed");
            const body = await response.json() as ResponseData<OwnProfile>;
            if (!body?.data?.player) throw new Error("The server did not confirm the profile");
            setOwn({ token, data: body.data });
            setEditOpen(false);
            setEditNotice("Profile saved");
        } catch (error) { setEditError(error instanceof Error ? error.message : "Profile save failed"); }
        finally { setSaving(false); }
    };

    const runRelationAction = async (action: RelationAction, targetId: number) => {
        if (!token || !user || targetId === user.id) return;
        const key = action + ":" + targetId;
        const actionScope = scopeKey;
        if (actionKey) return;
        setActionKey(key); setActionError(""); setActionNotice("");
        const path = action === "send" ? "/game/friends/add"
            : action === "remove" ? "/game/friends/" + targetId
            : action === "cancel" ? "/game/friends/requests/" + targetId
            : "/game/friends/requests/" + targetId + "/" + (action === "accept" ? "accept" : "reject");
        const method = action === "remove" || action === "cancel" ? "DELETE" : "POST";
        try {
            await performConfirmedSocialAction(
                () => writeApi(path, token, method, action === "send" ? { friendUserId: targetId } : undefined),
                async () => {
                    if (scopeRef.current !== actionScope) return;
                    if (viewId !== null) await refreshPublic(targetId);
                    else {
                        await refreshSocial();
                        if (searchedQuery) await runSearch(searchedQuery);
                    }
                },
            );
            if (scopeRef.current === actionScope) setActionNotice(action === "send" ? "Friend request sent" : action === "remove" ? "Friend removed" : action === "accept" ? "Request accepted" : action === "decline" ? "Request declined" : "Request canceled");
        } catch (error) { if (scopeRef.current === actionScope) setActionError(error instanceof Error ? error.message : "Social action failed"); }
        finally { setActionKey(current => current === key ? "" : current); }
    };

    const renderActions = (relation: Relation, targetId: number) => relationActions(relation, targetId, user?.id ?? -1).map(action =>
        <button className={action === "remove" || action === "decline" || action === "cancel" ? styles.dangerButton : styles.primaryButton}
            type="button" key={action} disabled={Boolean(actionKey)} onClick={() => void runRelationAction(action, targetId)}>
            {t(actionKey === action + ":" + targetId ? "Working…" : action === "send" ? "Add Friend" : action === "remove" ? "Remove" : action === "accept" ? "Accept" : action === "decline" ? "Decline" : "Cancel")}
        </button>,
    );

    const ownData = own?.token === token ? own.data : null;
    const publicData = publicProfile?.token === token && matchesProfileView(viewId, publicProfile.data.player.userId) ? publicProfile.data : null;
    const activeData = ownMode ? ownData : publicData;
    const roster = heroes?.token === token ? ownedHeroes(heroes.data) : null;
    const socialData = social?.token === token ? social.data : null;
    const fromGame = params.get("from") === "game";
    const instanceId = params.get("instance_id");
    const backPath = fromGame && instanceId ? "/game?instance_id=" + encodeURIComponent(instanceId) : "/mode";

    return <main className={styles.page}>
        <header className={styles.topRow}><div><span className={styles.eyebrow}><UiText text="NACHALO · PLAYER" /></span><h2>{t(ownMode ? "Your Profile" : "Player Profile")}</h2></div><button className={styles.secondaryButton} type="button" onClick={() => router.push(backPath)}>{t(fromGame && !ownMode ? "Back to Game" : "To Modes")}</button></header>
        {!activeData ? <section className={styles.panel} role={ownError || publicError ? "alert" : "status"}>{t((ownMode ? ownError : publicError) || "Loading profile…")}{(ownMode ? ownError : publicError) && <button className={styles.secondaryButton} type="button" onClick={() => void (ownMode ? refreshOwn() : refreshPublic(viewId!)).catch(error => ownMode ? setOwnError(String(error)) : setPublicError(String(error)))}><UiText text="Retry" /></button>}</section> : <>
            <Identity player={activeData.player} own={ownMode} />
            {ownMode && <div className={styles.ownerTools}>
                <button className={styles.secondaryButton} type="button" aria-expanded={editOpen} onClick={() => { setEditName(activeData.player.name); setEditImage(""); setEditError(""); setEditOpen(value => !value); }}><UiText text="Edit identity" /></button>
                <span className={styles.subtle}><UiText text="Account avatar and nickname" /></span>
                <Link href="/equipment"><UiText text="View Inventory ↗" /></Link><Link href="/base"><UiText text="View Base ↗" /></Link>
                {editNotice && <span className={styles.success} role="status">{t(editNotice)}</span>}
            </div>}
            {ownMode && editOpen && <form className={styles.editor} onSubmit={event => void saveIdentity(event)}>
                <label><UiText text="Nickname" /><input value={editName} onChange={event => setEditName(event.target.value)} maxLength={32} /></label>
                <label><UiText text="New avatar URL (optional)" /><input value={editImage} onChange={event => setEditImage(event.target.value)} maxLength={512} placeholder={t("Leave blank to keep saved avatar")} /></label>
                <button className={styles.primaryButton} type="submit" disabled={saving}>{t(saving ? "Saving…" : "Save")}</button>
                {editError && <p className={styles.error} role="alert">{editError}</p>}
            </form>}
            <div className={styles.overviewGrid}>
                <Overview progress={activeData.progress} own={ownMode} />
                {ownMode && <section className={styles.panel} aria-label="Hero roster">
                    <div className={styles.sectionHead}><div><span className={styles.eyebrow}><UiText text="YOUR COMPANY" /></span><h2><UiText text="Owned heroes" /></h2></div><Link href="/base"><UiText text="Manage in Tavern ↗" /></Link></div>
                    {heroesError && <p className={styles.error} role="alert">{heroesError}</p>}
                    {!roster ? <p className={styles.empty}>{t(heroesError ? "Hero roster unavailable." : "Loading heroes…")}</p> : roster.length === 0 ? <p className={styles.empty}><UiText text="No owned heroes returned by the server." /></p> : <div className={styles.heroRoster}>{roster.map(hero => <div className={styles.heroTile} key={hero.id}><img src={getHeroPortrait(hero.id, hero.image)} alt="" /><strong>{hero.displayName}</strong>{hero.active && <span><UiText text="ACTIVE" /></span>}</div>)}</div>}
                </section>}
            </div>
            {ownMode ? <section className={styles.panel} aria-label="Social">
                <div className={styles.sectionHead}><div><span className={styles.eyebrow}><UiText text="SOCIAL" /></span><h2><UiText text="Players & friends" /></h2></div></div>
                <div className={styles.socialTabs} role="group" aria-label="Social views">{TABS.map(item => <button key={item.key} type="button" aria-pressed={tab === item.key} onClick={() => setTab(item.key)}>{t(item.label)}{socialData && item.key !== "search" ? " " + socialData[item.key].length : ""}</button>)}</div>
                {socialError && <p className={styles.error} role="alert">{socialError} <button type="button" onClick={() => void refreshSocial().catch(error => setSocialError(String(error)))}><UiText text="Retry" /></button></p>}
                {actionError && <p className={styles.error} role="alert">{actionError}</p>}
                {actionNotice && <p className={styles.success} role="status">{actionNotice}</p>}
                {tab === "search" ? <div className={styles.socialBody}>
                    <form className={styles.searchForm} onSubmit={event => { event.preventDefault(); void runSearch(searchQuery); }}><label htmlFor="profile-search"><UiText text="Find players by name" /></label><div><input id="profile-search" value={searchQuery} onChange={event => setSearchQuery(event.target.value)} placeholder={t("At least 2 characters")} /><button className={styles.primaryButton} type="submit" disabled={searching}>{t(searching ? "Searching…" : "Search")}</button></div></form>
                    {searchError && <p className={styles.error} role="alert">{searchError}</p>}
                    {searching && <p className={styles.subtle} role="status"><UiText text="Searching players…" /></p>}
                    {!searching && searchedQuery && searchResults.length === 0 && <p className={styles.empty}><UiText text="No players found for “" />{searchedQuery}”.</p>}
                    {searchResults.length > 0 && <div className={styles.playerList}>{searchResults.map(player => <PlayerRow key={player.userId} player={player}>{renderActions(player.friendRelation, player.userId)}</PlayerRow>)}</div>}
                </div> : !socialData ? <p className={styles.empty}>{t(socialError ? "Social data unavailable." : "Loading social data…")}</p> : socialData[tab].length === 0 ? <p className={styles.empty}>{t(tab === "friends" ? "Your friend list is empty." : tab === "incoming" ? "No incoming requests." : "No outgoing requests.")}</p> : <div className={styles.playerList}>{socialData[tab].map(player => <PlayerRow key={player.userId} player={player}>
                    {tab === "friends" ? <button className={styles.dangerButton} type="button" disabled={Boolean(actionKey)} onClick={() => void runRelationAction("remove", player.userId)}>{t(actionKey === "remove:" + player.userId ? "Removing…" : "Remove")}</button> : tab === "incoming" ? <><button className={styles.primaryButton} type="button" disabled={Boolean(actionKey)} onClick={() => void runRelationAction("accept", player.userId)}><UiText text="Accept" /></button><button className={styles.dangerButton} type="button" disabled={Boolean(actionKey)} onClick={() => void runRelationAction("decline", player.userId)}><UiText text="Decline" /></button></> : <><span className={styles.pending}><UiText text="Pending" /></span><button className={styles.dangerButton} type="button" disabled={Boolean(actionKey)} onClick={() => void runRelationAction("cancel", player.userId)}><UiText text="Cancel" /></button></>}
                </PlayerRow>)}</div>}
            </section> : <section className={styles.panel} aria-label="Friend relation"><div className={styles.sectionHead}><div><span className={styles.eyebrow}><UiText text="SOCIAL" /></span><h2><UiText text="Connection" /></h2></div></div><p className={styles.subtle}><UiText text="Status:" /> {publicData?.friendRelation ?? "—"}</p>{actionError && <p className={styles.error} role="alert">{actionError}</p>}{actionNotice && <p className={styles.success} role="status">{actionNotice}</p>}<div className={styles.publicActions}>{publicData && renderActions(publicData.friendRelation, publicData.player.userId)}</div></section>}
        </>}
    </main>;
}
