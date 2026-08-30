import { useEffect, useMemo, useRef, useState } from "react";
import { boardForMatchMode, type BoardDefinition, type BoardNode } from "./board";

type Seat = "north" | "east" | "south" | "west";
type Phase = "lobby" | "setup" | "playing" | "finished";
type Mode = "four_dark" | "double_visible" | "fully_visible";
type MatchMode = "two_vs_two" | "one_vs_one";
type Clock = "fast" | "standard" | "relaxed";

type Player = { username: string; ready: boolean; connected: boolean; eliminated: boolean; misses: number };
type VisiblePiece = { id: string; owner: Seat; kind?: string; revealed?: boolean };
type Move = { seat: Seat; from: string; to: string; result: string };
type View = {
  matchId?: string;
  version: number;
  phase: Phase;
  paused?: boolean;
  matchMode?: MatchMode;
  mode: Mode;
  clock: Clock;
  turn?: Seat;
  deadline?: string;
  setupDeadline?: string;
  opening: Seat;
  players: Record<Seat, Player>;
  participants: Participant[];
  pieces: Record<string, VisiblePiece>;
  revealedFlags: Record<string, string>;
  legalMoves?: string[];
  lastMove?: Move;
  drawOffer?: Seat;
  result?: { outcome: string; team?: string; reason?: string };
};
type Room = { code: string; hostUsername?: string; phase: Phase; matchMode?: MatchMode; mode: Mode; clock: Clock; opening: Seat; participants: Participant[]; spectatorCap: number };
type Participant = { username: string; seat?: Seat; role: "player" | "spectator"; connected: boolean; self?: boolean };
type ReplayState = { phase: Phase; matchMode?: MatchMode; turn?: Seat; pieces: Record<string, { owner: Seat; kind: string }>; lastMove?: Move; result?: { outcome: string; team?: string; reason?: string } };
type ReplayEvent = { sequence: number; type: string; payload: ReplayState; createdAt: string };
type ProfileSummary = { id: string; username: string; matches: number; wins: number; losses: number; draws: number };
type MatchSummary = { id: string; outcome: string; mode: Mode; clock: Clock; startedAt?: string; finishedAt?: string };
type AppConfig = { baseUrl?: string };

const seats: Seat[] = ["north", "east", "south", "west"];
const oneVsOneSeats: Seat[] = ["north", "south"];
const usernamePattern = /^[a-z][a-z_]{1,18}[a-z]$/;
const seatLabels: Record<Seat, string> = { north: "北", east: "东", south: "南", west: "西" };
const pieceLabels: Record<string, string> = {
  flag: "军旗", commander: "司令", marshal: "军长", division: "师长", brigade: "旅长", regiment: "团长",
  battalion: "营长", bomb: "炸弹", company: "连长", platoon: "排长", engineer: "工兵", mine: "地雷",
};

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { credentials: "same-origin", headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) }, ...init });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error ?? "请求失败");
  return body as T;
}

function useCountdown(deadline?: string) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(timer);
  }, []);
  if (!deadline) return null;
  return Math.max(0, Math.ceil((new Date(deadline).getTime() - now) / 1000));
}

export function App() {
  const [user, setUser] = useState<string | null>(null);
  const [roomCode, setRoomCode] = useState<string | null>(() => roomCodeFromLocation());
  const [baseUrl, setBaseUrl] = useState("");
  const [error, setError] = useState("");
  const path = window.location.pathname;
  useEffect(() => {
    api<AppConfig>("/api/config").then((data) => setBaseUrl(data.baseUrl?.trim() || window.location.origin)).catch(() => setBaseUrl(window.location.origin));
    api<{ username: string }>("/api/me").then((data) => setUser(data.username)).catch(() => undefined);
  }, []);
  useEffect(() => {
    if (roomCode) canonicalizeRoomLocation(roomCode);
  }, [roomCode]);
  if (path.startsWith("/replay/")) return <ReplayScreen matchId={decodeURIComponent(path.slice("/replay/".length))} />;
  if (path.startsWith("/profile/")) return <ProfileScreen username={decodeURIComponent(path.slice("/profile/".length))} />;
  if (!user) return <SignIn onSignedIn={setUser} />;
  if (!roomCode) return <Home user={user} onRoom={(code) => { setError(""); setRoomCode(code); }} onSwitchUser={() => setUser(null)} onError={setError} error={error} />;
  return <RoomScreen user={user} code={roomCode} baseUrl={baseUrl} onLeave={() => { forgetRoomMembership(roomCode, user); clearRoomLocation(); setRoomCode(null); }} onError={setError} error={error} />;
}

function roomCodeFromLocation() {
  if (/^\/(?:api|replay|profile)(?:\/|$)/.test(window.location.pathname)) return null;
  const pathMatch = window.location.pathname.match(/^\/([^/]+)\/?$/);
  const pathCode = pathMatch?.[1];
  if (pathCode && isRoomCode(pathCode)) return pathCode.toUpperCase();
  const queryCode = new URLSearchParams(window.location.search).get("room")?.trim();
  return queryCode && isRoomCode(queryCode) ? queryCode.toUpperCase() : null;
}

function isRoomCode(code: string) {
  return /^[A-Za-z][A-Za-z0-9]{7}$/.test(code) && /[0-9]/.test(code);
}

function canonicalizeRoomLocation(code: string) {
  const url = new URL(window.location.href);
  if (url.pathname === `/${code}` && !url.search) return;
  url.pathname = `/${code}`;
  url.search = "";
  window.history.replaceState({}, "", `${url.pathname}${url.search}${url.hash}`);
}

function clearRoomLocation() {
  const url = new URL(window.location.href);
  url.pathname = "/";
  url.search = "";
  window.history.replaceState({}, "", `${url.pathname}${url.search}${url.hash}`);
}

function roomMembershipKey(code: string, user: string) {
  return `army-chess:room:${code}:${user}`;
}

function rememberRoomMembership(code: string, user: string) {
  window.sessionStorage.setItem(roomMembershipKey(code, user), "joined");
}

function hasRoomMembership(code: string, user: string) {
  return window.sessionStorage.getItem(roomMembershipKey(code, user)) === "joined";
}

function forgetRoomMembership(code: string, user: string) {
  window.sessionStorage.removeItem(roomMembershipKey(code, user));
}

function seatsForMatchMode(matchMode?: MatchMode) {
  return matchMode === "one_vs_one" ? oneVsOneSeats : seats;
}

function matchModeFor(view: Pick<View, "matchMode"> | Pick<Room, "matchMode">): MatchMode {
  return view.matchMode ?? "two_vs_two";
}

function roomShareURL(baseUrl: string, code: string) {
  const url = new URL(baseUrl || window.location.origin);
  const basePath = url.pathname.replace(/\/+$/, "");
  url.pathname = `${basePath}/${code}`;
  url.search = "";
  url.hash = "";
  return url.toString();
}

function ReplayScreen({ matchId }: { matchId: string }) {
  const [events, setEvents] = useState<ReplayEvent[]>([]);
  const [index, setIndex] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => { api<{ events: ReplayEvent[] }>(`/api/matches/${encodeURIComponent(matchId)}/replay`).then((data) => setEvents(data.events)).catch((err) => setError((err as Error).message)); }, [matchId]);
  const current = events[index]?.payload;
  return <main className="page-shell replay-shell"><header className="topbar"><a className="text-button" href="/">← 返回</a><span className="room-code">回放 / {matchId}</span><span className="user-chip">回放</span></header>{error ? <p className="error centered">{error}</p> : !current ? <div className="loading">正在加载回放…</div> : <section className="replay-layout"><div><h1 className="replay-title">战局回放</h1><p className="lede">事件 {index + 1} / {events.length} · {current.phase === "finished" ? "比赛结束" : current.phase}</p><ReplayBoard state={current} /></div><aside className="replay-controls"><button className="secondary" disabled={index === 0} onClick={() => setIndex((value) => value - 1)}>← 上一步</button><button className="primary" disabled={index >= events.length - 1} onClick={() => setIndex((value) => value + 1)}>下一步 →</button><div className="event-card"><span className="event-tag">{events[index].type}</span><strong>序列 {events[index].sequence}</strong>{current.lastMove && <span>{seatLabels[current.lastMove.seat]} · {current.lastMove.from} → {current.lastMove.to}</span>}</div>{current.result && <div className="result-banner"><span>结果</span><strong>{current.result.outcome === "draw" ? "和棋" : `${teamLabel(current.result.team)} 获胜`}</strong><small>{current.result.reason}</small></div>}</aside></section>}</main>;
}

function ReplayBoard({ state }: { state: ReplayState }) {
  return <BoardCanvas board={boardForMatchMode(state.matchMode)} pieces={state.pieces} readOnly className="replay-board" />;
}

function ProfileScreen({ username }: { username: string }) {
  const [summary, setSummary] = useState<ProfileSummary | null>(null);
  const [matches, setMatches] = useState<MatchSummary[]>([]);
  const [error, setError] = useState("");
  useEffect(() => { Promise.all([api<ProfileSummary>(`/api/profiles/${encodeURIComponent(username)}`), api<{ matches: MatchSummary[] }>(`/api/profiles/${encodeURIComponent(username)}/matches`)]).then(([profile, history]) => { setSummary(profile); setMatches(history.matches); }).catch((err) => setError((err as Error).message)); }, [username]);
  return <main className="page-shell"><header className="topbar"><a className="text-button" href="/">← 返回</a><span className="room-code">资料 / {username}</span><span className="user-chip">资料</span></header>{error ? <p className="error centered">{error}</p> : !summary ? <div className="loading">正在读取资料…</div> : <section className="profile-layout"><div className="profile-hero"><h1>{summary.username}</h1><p className="lede">对局统计</p><div className="stats-grid"><div><strong>{summary.matches}</strong><span>对局</span></div><div><strong>{summary.wins}</strong><span>胜利</span></div><div><strong>{summary.losses}</strong><span>失利</span></div><div><strong>{summary.draws}</strong><span>和棋</span></div></div></div><div className="history-panel"><div className="panel-heading"><h2>比赛历史</h2></div>{matches.length === 0 ? <p className="empty-copy">还没有可显示的比赛。</p> : matches.map((match) => <a className="history-row" href={`/replay/${match.id}`} key={match.id}><span>{match.finishedAt ? new Date(match.finishedAt).toLocaleDateString() : "进行中"}</span><strong>{match.outcome}</strong><small>{modeLabel(match.mode)} · {clockLabel(match.clock)}</small></a>)}</div></section>}</main>;
}

function SignIn({ onSignedIn }: { onSignedIn: (username: string) => void }) {
  const [username, setUsername] = useState("");
  const [error, setError] = useState("");
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    const normalizedUsername = username.trim().toLowerCase();
    if (!usernamePattern.test(normalizedUsername)) {
      setError("用户名格式：3–20 个小写英文字母和下划线，首尾必须是字母，例如 red_cedar。");
      return;
    }
    try { const data = await api<{ username: string }>("/api/session", { method: "POST", body: JSON.stringify({ username: normalizedUsername }) }); onSignedIn(data.username); }
    catch (err) { setError((err as Error).message); }
  }
  return <main className="auth-shell"><section className="auth-card">
    <h1>四国军棋</h1>
    <form onSubmit={submit}><label htmlFor="username">用户名</label><input id="username" aria-describedby="username-help" autoFocus value={username} onChange={(e) => { setUsername(e.target.value); setError(""); }} placeholder="例如 red_cedar" required /><p id="username-help" className="input-help">3–20 个小写英文字母和下划线，首尾为字母。</p><button type="submit">进入</button></form>
    {error && <p className="error">{error}</p>}
  </section></main>;
}

function Home({ user, onRoom, onSwitchUser, onError, error }: { user: string; onRoom: (code: string) => void; onSwitchUser: () => void; onError: (message: string) => void; error: string }) {
  const [code, setCode] = useState("");
  async function create() { try { const room = await api<Room>("/api/rooms", { method: "POST", body: "{}" }); onRoom(room.code); } catch (err) { onError((err as Error).message); } }
  async function join() { try { const room = await api<Room>(`/api/rooms/${code.trim().toUpperCase()}/join`, { method: "POST", body: JSON.stringify({}) }); onRoom(room.code); } catch (err) { onError((err as Error).message); } }
  async function switchUser() { try { await api<unknown>("/api/session", { method: "DELETE" }); onSwitchUser(); } catch (err) { onError((err as Error).message); } }
  return <main className="page-shell"><header className="topbar"><div className="brand-name">四国军棋</div><button className="user-chip switch-user" type="button" onClick={switchUser}>{user} · 切换用户名</button></header>
    <section className="home-grid"><div className="hero-panel"><h1>房间</h1><p>创建一个房间，或使用邀请码加入朋友的对局。</p><button className="primary large" onClick={create}>创建新房间</button></div>
    <div className="join-panel"><div className="panel-heading"><h2>加入房间</h2></div><label htmlFor="room-code">房间邀请码</label><input id="room-code" value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} maxLength={8} placeholder="ABCDEFGH" /><div className="button-row"><button className="primary" disabled={code.length !== 8} onClick={join}>进入房间</button></div><p className="fine-print">进入房间后默认在旁观席，可随时选择空座位或返回旁观席。</p></div></section>
    {error && <p className="error centered">{error}</p>}
  </main>;
}

function RoomScreen({ user, code, baseUrl, onLeave, onError, error }: { user: string; code: string; baseUrl: string; onLeave: () => void; onError: (message: string) => void; error: string }) {
  const [room, setRoom] = useState<Room | null>(null);
  const [view, setView] = useState<View | null>(null);
  const [joined, setJoined] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<"connecting" | "connected" | "error" | "disconnected">("connecting");
  const socketRef = useRef<WebSocket | null>(null);
  useEffect(() => {
    let alive = true;
    let timedOut = false;
    const controller = new AbortController();
    const timeout = window.setTimeout(() => {
      timedOut = true;
      controller.abort();
      if (alive) onError("读取房间超时，请检查房间链接或服务状态。");
    }, 8000);
    api<Room>(`/api/rooms/${code}`, { signal: controller.signal })
      .then((data) => {
        if (!alive) return;
        setRoom(data);
        if (!hasRoomMembership(code, user)) return;
        return api<Room>(`/api/rooms/${code}/join`, { method: "POST", body: JSON.stringify({}) })
          .then((joinedRoom) => {
            if (!alive) return;
            setRoom(joinedRoom);
            setJoined(true);
            connect();
          })
          .catch((err) => {
            forgetRoomMembership(code, user);
            if (alive) onError((err as Error).message);
          });
      })
      .catch((err) => { if (alive && !timedOut && (err as Error).name !== "AbortError") onError((err as Error).message); })
      .finally(() => window.clearTimeout(timeout));
    return () => { alive = false; window.clearTimeout(timeout); controller.abort(); socketRef.current?.close(); };
  }, [code, onError]);
  async function join() {
    try { const data = await api<Room>(`/api/rooms/${code}/join`, { method: "POST", body: JSON.stringify({}) }); rememberRoomMembership(code, user); setRoom(data); setJoined(true); connect(); }
    catch (err) { onError((err as Error).message); }
  }
  function connect() {
    setConnectionStatus("connecting");
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const socket = new WebSocket(`${protocol}//${window.location.host}/api/rooms/${code}/ws`);
    socket.onopen = () => { socketRef.current = socket; setConnectionStatus("connected"); };
    socket.onmessage = (event) => { const message = JSON.parse(event.data) as { type: string; payload: View | { message: string } }; if (message.type === "snapshot") setView(message.payload as View); if (message.type === "error") onError((message.payload as { message: string }).message); };
    socket.onerror = () => setConnectionStatus("error");
    socket.onclose = () => { socketRef.current = null; setConnectionStatus((current) => current === "error" ? current : "disconnected"); };
  }
  function send(type: string, payload: unknown = {}) { socketRef.current?.send(JSON.stringify({ type, requestId: crypto.randomUUID(), payload })); }
  if (!room) {
    if (!error) return <main className="page-shell"><div className="loading">正在读取房间…</div></main>;
    const loadError = error === "room not found" ? "房间不存在或已过期。" : error;
    return <main className="page-shell"><section className="join-room-card room-load-error"><h1>无法读取房间</h1><p>{loadError}</p><button className="primary large" onClick={onLeave}>返回房间入口</button></section></main>;
  }
  if (!joined) return <main className="page-shell"><header className="topbar"><button className="text-button" onClick={onLeave}>← 返回</button><span className="room-code">{code}</span><span className="user-chip">{user}</span></header><section className="join-room-card"><h1>进入房间</h1><p>当前阶段：{phaseLabel(room.phase)} · {room.participants.length} 人在线或已登记</p><button className="primary large" onClick={join}>进入</button><ShareRoom code={code} baseUrl={baseUrl} /></section></main>;
  const mySeat = view ? seatFor(view, user) : undefined;
  const setupMode = Boolean(view && !view.paused && (view.phase === "lobby" || view.phase === "setup"));
  const activeSeats = view ? seatsForMatchMode(view.matchMode) : seats;
  const roomStatus = view && (!setupMode || !mySeat) && view.phase !== "playing" ? `等待${activeSeats.length}席就绪` : null;
  const connectionLabels = { connecting: "连接中", connected: "已连接", error: "连接失败", disconnected: "已断开" };
  return <main className="game-shell"><header className="topbar"><button className="text-button" onClick={onLeave}>← 退出房间</button><div className="room-code"><span>ROOM</span> {code}</div><span className={`connection-status ${connectionStatus}`}>● {connectionLabels[connectionStatus]}</span></header>{error && <div className="toast error">{error}</div>}
    {!view ? <div className="loading">正在建立实时连接…</div> : <div className="room-layout"><aside className="side-panel left"><RoomHeader view={view} status={roomStatus} /><GameStatus view={view} user={user} /><SeatList view={view} user={user} send={send} /></aside><section className="board-panel"><GameActions view={view} user={user} send={send} /><Board view={view} user={user} send={send} /></section><aside className="side-panel right"><ShareRoom code={code} baseUrl={baseUrl} /><EventPanel view={view} /><RoomControlPanel view={view} send={send} /><RoomControls room={room} view={view} user={user} send={send} /></aside></div>}
  </main>;
}

function RoomHeader({ view, status }: { view: View; status: string | null }) { return <div className="room-heading"><h1>{view.paused ? "对局已暂停" : phaseLabel(view.phase)}</h1><p>{matchModeLabel(view.matchMode)} · {modeLabel(view.mode)} · {clockLabel(view.clock)} · 开局：{seatLabels[view.opening]}</p>{status && <p className="room-status">{status}</p>}</div>; }

function ShareRoom({ code, baseUrl }: { code: string; baseUrl: string }) {
  const [copied, setCopied] = useState(false);
  const shareUrl = roomShareURL(baseUrl, code);
  async function copy() {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }
  return <div className="share-card"><div className="panel-heading"><h2>分享房间</h2></div><div className="share-row"><input aria-label="房间分享链接" readOnly value={shareUrl} /><button className="secondary" type="button" onClick={copy}>{copied ? "已复制" : "复制"}</button></div></div>;
}

function participantFor(view: View, user: string) {
  const participants = view.participants ?? [];
  return participants.find((participant) => participant.self) ?? participants.find((participant) => participant.username === user);
}

function seatFor(view: View, user: string): Seat | undefined {
  const participant = participantFor(view, user);
  if (participant?.seat) return participant.seat;
  return seatsForMatchMode(view.matchMode).find((seat) => view.players[seat]?.username === user);
}

function SeatList({ view, user, send }: { view: View; user: string; send: (type: string, payload?: unknown) => void }) {
  const mySeat = seatFor(view, user);
  const spectators = (view.participants ?? []).filter((participant) => participant.role === "spectator");
  const canChangeSeats = view.phase === "lobby" || view.phase === "setup";
  const activeSeats = seatsForMatchMode(view.matchMode);
  const teamKey = matchModeFor(view) === "one_vs_one" ? <span><i className="team north-south" />北 vs 南 · 对手</span> : <><span><i className="team north-south" />北 / 南 · 同队</span><span><i className="team east-west" />东 / 西 · 同队</span></>;
  return <div className="seat-list"><div className="panel-heading"><h2>座位与队伍</h2></div>{activeSeats.map((seat) => { const player = view.players[seat]; const mine = mySeat === seat; return <div className={`seat-row ${mine ? "mine" : ""} ${view.turn === seat ? "turn" : ""}`} key={seat}><span className={`seat-badge ${seat}`}>{seatLabels[seat]}</span><div><strong>{player?.username || "等待玩家"}</strong><span>{player?.eliminated ? "已出局" : player?.connected ? (mine ? "你 · 已连接" : "在线") : player?.username ? "暂离" : "开放座位"}</span></div>{canChangeSeats && (mine ? <button className="mini-button" onClick={() => send("seat.leave")}>离座</button> : !player?.username ? <button className="mini-button" onClick={() => send("seat.select", { seat })}>入座</button> : null)}{view.turn === seat && view.phase === "playing" && <span className="turn-mark">行动中</span>}</div>; })}<div className="team-key">{teamKey}</div><div className="spectator-list"><div className="panel-heading"><h2>观众</h2><span className="participant-count">{spectators.length}</span></div>{spectators.length === 0 ? <p className="empty-copy">暂无观众</p> : spectators.map((participant, index) => <div className="spectator-row" key={`${participant.username}-${index}`}><span className="spectator-dot" /><div><strong>{participant.username}{participant.self ? "（你）" : ""}</strong><span>{participant.connected ? "在线 · 可随时入座" : "暂离"}</span></div></div>)}</div></div>;
}

function Board({ view, user, send }: { view: View; user: string; send: (type: string, payload?: unknown) => void }) {
  const [selected, setSelected] = useState<string | null>(null);
  const mySeat = seatFor(view, user);
  const setupMode = !view.paused && (view.phase === "lobby" || view.phase === "setup");
  const [setupSelection, setSetupSelection] = useState<SetupSelection>(null);
  const [setupArrangement, setSetupArrangement] = useState<SetupArrangement | null>(null);
  const legal = useMemo(() => new Set((view.legalMoves ?? []).map((move) => move.replace("->", "|"))), [view.legalMoves]);

  useEffect(() => {
    if (!setupMode || !mySeat) {
      setSetupArrangement(null);
      setSetupSelection(null);
      return;
    }
    const pieces: Record<string, BoardPiece> = {};
    for (const [node, piece] of Object.entries(view.pieces)) {
      if (piece.owner === mySeat && piece.kind) pieces[node] = { id: piece.id, owner: piece.owner, kind: piece.kind };
    }
    setSetupArrangement({ pieces, tray: Array.from({ length: setupTraySize }, () => null) });
    setSetupSelection(null);
  }, [mySeat, setupMode, view.version]);

  const displayPieces = useMemo(() => {
    if (!setupArrangement || !mySeat) return view.pieces;
    const pieces = { ...view.pieces } as Record<string, BoardPiece>;
    for (const [node, piece] of Object.entries(view.pieces)) {
      if (piece.owner === mySeat) delete pieces[node];
    }
    return { ...pieces, ...setupArrangement.pieces };
  }, [mySeat, setupArrangement, view.pieces]);

  function persistArrangement(arrangement: SetupArrangement) {
    setSetupArrangement(arrangement);
    if (arrangement.tray.some(Boolean)) return;
    const pieces = Object.fromEntries(Object.entries(arrangement.pieces).map(([node, piece]) => [node, { id: piece.id, owner: piece.owner, kind: piece.kind }])) as Record<string, BoardPiece>;
    send("setup.replace", { pieces });
  }

  function clickSetupBoard(node: string) {
    if (!setupArrangement || !mySeat) return;
    const ownPiece = setupArrangement.pieces[node];
    const externalPiece = view.pieces[node];
    if (externalPiece && externalPiece.owner !== mySeat) {
      setSetupSelection(null);
      return;
    }
    if (!setupSelection) {
      if (ownPiece) setSetupSelection({ type: "board", node });
      return;
    }
    if (setupSelection.type === "board") {
      if (setupSelection.node === node) {
        setSetupSelection(null);
        return;
      }
      if (!ownPiece) {
        const pieces = { ...setupArrangement.pieces };
        pieces[node] = pieces[setupSelection.node];
        delete pieces[setupSelection.node];
        persistArrangement({ ...setupArrangement, pieces });
      } else {
        const pieces = { ...setupArrangement.pieces, [setupSelection.node]: ownPiece, [node]: setupArrangement.pieces[setupSelection.node] };
        persistArrangement({ ...setupArrangement, pieces });
      }
      setSetupSelection(null);
      return;
    }
    const tray = [...setupArrangement.tray];
    const trayPiece = tray[setupSelection.index];
    if (!trayPiece) {
      setSetupSelection(null);
      return;
    }
    if (ownPiece) {
      tray[setupSelection.index] = ownPiece;
    } else {
      tray[setupSelection.index] = null;
    }
    persistArrangement({ pieces: { ...setupArrangement.pieces, [node]: trayPiece }, tray });
    setSetupSelection(null);
  }

  function clickSetupTray(index: number) {
    if (!setupArrangement) return;
    const trayPiece = setupArrangement.tray[index];
    if (!setupSelection) {
      if (trayPiece) setSetupSelection({ type: "tray", index });
      return;
    }
    if (setupSelection.type === "tray") {
      if (setupSelection.index === index) {
        setSetupSelection(null);
        return;
      }
      const tray = [...setupArrangement.tray];
      tray[index] = tray[setupSelection.index];
      tray[setupSelection.index] = trayPiece;
      setSetupArrangement({ ...setupArrangement, tray });
      setSetupSelection({ type: "tray", index });
      return;
    }
    const pieces = { ...setupArrangement.pieces };
    const boardPiece = pieces[setupSelection.node];
    if (!boardPiece) {
      setSetupSelection(null);
      return;
    }
    const tray = [...setupArrangement.tray];
    tray[index] = boardPiece;
    delete pieces[setupSelection.node];
    if (trayPiece) pieces[setupSelection.node] = trayPiece;
    persistArrangement({ pieces, tray });
    setSetupSelection(null);
  }

  function click(node: string) {
    if (setupMode && mySeat) { clickSetupBoard(node); return; }
    if (view.phase !== "playing" || !mySeat) return;
    if (selected && legal.has(`${selected}|${node}`)) { send("move", { from: selected, to: node }); setSelected(null); return; }
    const piece = view.pieces[node];
    if (piece?.owner === mySeat && piece.kind) setSelected(node); else setSelected(null);
  }
  const setupSelectedNode = setupSelection?.type === "board" ? setupSelection.node : null;
  const setupSelectedTray = setupSelection?.type === "tray" ? setupSelection.index : null;
  const trayCount = setupArrangement?.tray.filter(Boolean).length ?? 0;
  const selectedPiece = setupSelection?.type === "board" ? setupArrangement?.pieces[setupSelection.node] : setupSelection?.type === "tray" ? setupArrangement?.tray[setupSelection.index] : undefined;
  const showSetupTray = setupMode && Boolean(mySeat);
  const board = boardForMatchMode(view.matchMode);
  const setupHint = setupMode && mySeat ? trayCount > 0 ? `临时区还有 ${trayCount} 枚棋子待放回` : selectedPiece ? `已选择：${pieceLabels[selectedPiece.kind ?? ""] ?? "棋子"}，再选择位置` : "点击棋子选择；可放入右侧临时区" : null;
  return <div className="board-wrap">{setupHint && <div className="board-caption"><span>{setupHint}</span></div>}<div className={`board-stage ${showSetupTray ? "with-setup-tray" : ""}`}><BoardCanvas board={board} pieces={displayPieces} selected={selected ?? setupSelectedNode} targets={selected ? legal : undefined} onNodeClick={click} /><SetupTray visible={showSetupTray} pieces={setupArrangement?.tray ?? []} selected={setupSelectedTray} onSlotClick={clickSetupTray} /></div></div>;
}

type BoardPiece = { id?: string; owner: Seat; kind?: string };
type SetupSelection = { type: "board"; node: string } | { type: "tray"; index: number } | null;
type SetupArrangement = { pieces: Record<string, BoardPiece>; tray: Array<BoardPiece | null> };
const setupTraySize = 8;

function SetupTray({ visible, pieces, selected, onSlotClick }: { visible: boolean; pieces: Array<BoardPiece | null>; selected: number | null; onSlotClick: (index: number) => void }) {
  if (!visible) return null;
  return <aside className="setup-tray"><div className="setup-tray-title">临时区</div><div className="setup-tray-help">先选棋子，再点击空位取出。放回全部棋子后自动保存。</div><div className="setup-tray-slots">{pieces.map((piece, index) => <button key={index} type="button" className={`setup-slot ${piece ? `occupied ${piece.owner}` : "empty"} ${selected === index ? "selected" : ""}`} onClick={() => onSlotClick(index)} aria-label={`临时位置 ${index + 1}${piece ? ` ${piece.kind ? pieceLabels[piece.kind] : "棋子"}` : " 空位"}`}>{piece ? <><span className="setup-slot-owner">{seatLabels[piece.owner]}</span><span>{piece.kind ? pieceLabels[piece.kind] : "棋子"}</span></> : <span>空位</span>}</button>)}</div></aside>;
}

function BoardCanvas({ board, pieces, selected, targets, onNodeClick, readOnly = false, className = "" }: { board: BoardDefinition; pieces: Record<string, BoardPiece>; selected?: string | null; targets?: Set<string>; onNodeClick?: (node: string) => void; readOnly?: boolean; className?: string }) {
  const nodes = Object.values(board.nodes);
  const nodeByID = board.nodes;
  const edges = useMemo(() => {
    const seen = new Set<string>();
    return board.edges.filter((edge) => {
      const key = [edge.from, edge.to].sort().join("|");
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }, [board]);
  function activate(node: string) {
    if (readOnly || !onNodeClick) return;
    onNodeClick(node);
  }
  const oneVsOne = board.version === "board.1v1";
  const viewBox = oneVsOne ? `0 0 ${board.width} ${board.height}` : `80 80 ${board.width - 160} ${board.height - 160}`;
  return <svg className={`board-svg ${oneVsOne ? "board-1v1" : "board-2v2"} ${className}`} viewBox={viewBox} role="grid" aria-label={oneVsOne ? "一对一陆战棋棋盘" : "四国军棋棋盘"}>
    <g className="board-edges" aria-hidden="true">{edges.map((edge) => { const from = nodeByID[edge.from]; const to = nodeByID[edge.to]; return <line key={`${edge.from}-${edge.to}`} className={`board-edge ${edge.type}`} x1={from.x} y1={from.y} x2={to.x} y2={to.y} />; })}</g>
    <g className="board-nodes">{nodes.map((node: BoardNode) => { const piece = pieces[node.id]; const target = targets?.has(node.id) ?? false; const clickable = !readOnly && Boolean(onNodeClick) && node.type !== "mountain"; const slot = oneVsOne ? node.type === "camp" ? { width: 82, height: 58, radius: 28 } : { width: 92, height: 58, radius: node.type === "mountain" ? 28 : 3 } : node.type === "headquarters" ? { width: 42, height: 36, radius: 2 } : node.type === "camp" ? { width: 40, height: 36, radius: 9 } : { width: 38, height: 32, radius: 2 }; return <g key={node.id} className={`board-node ${node.type} ${piece ? `occupied ${piece.owner}` : ""} ${target ? "target" : ""} ${selected === node.id ? "selected" : ""}`} role="gridcell" tabIndex={clickable ? 0 : undefined} onClick={() => activate(node.id)} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); activate(node.id); } }} aria-label={`${node.id}${piece ? ` ${piece.kind ? pieceLabels[piece.kind] : "未知棋子"}` : " 空位"}`}>
      <title>{node.id}</title><rect className="board-slot" x={node.x - slot.width / 2} y={node.y - slot.height / 2} width={slot.width} height={slot.height} rx={slot.radius} />{piece && <><text className="piece-owner" x={node.x} y={node.y - 7}>{seatLabels[piece.owner]}</text><text className="piece-label" x={node.x} y={node.y + 12}>{piece.kind ? pieceLabels[piece.kind] : "?"}</text></>}
    </g>; })}</g>
  </svg>;
}

function GameStatus({ view, user }: { view: View; user: string }) {
  const countdown = useCountdown(view.phase === "setup" ? view.setupDeadline : view.deadline);
  const mine = seatFor(view, user);
  const canAct = Boolean(mine);
  const yourTurn = view.phase === "playing" && !view.paused && canAct && view.turn === mine;
  const urgent = yourTurn && countdown !== null && countdown <= 10;
  const critical = yourTurn && countdown !== null && countdown <= 5;
  return <div className="game-actions room-status-actions"><div className={`clock-card ${yourTurn ? "your-turn" : ""} ${yourTurn && mine ? `turn-${mine}` : ""} ${urgent ? "urgent" : ""} ${critical ? "critical" : ""}`} aria-live="polite"><span>{view.paused ? "对局已暂停" : yourTurn ? "你的回合" : view.phase === "setup" ? "部署倒计时" : view.phase === "playing" ? `轮到${seatLabels[view.turn ?? "north"]}` : "比赛状态"}</span><strong>{view.paused ? "暂停" : countdown === null ? "—" : `${String(Math.floor(countdown / 60)).padStart(2, "0")}:${String(countdown % 60).padStart(2, "0")}`}</strong></div></div>;
}

function GameActions({ view, user, send }: { view: View; user: string; send: (type: string, payload?: unknown) => void }) {
  const mine = seatFor(view, user);
  const player = mine ? view.players[mine] : undefined;
  const canAct = Boolean(mine);
  return <div className="game-actions">{view.phase === "setup" && !view.paused && canAct && <button className="primary" onClick={() => send("ready", { ready: !player?.ready })}>{player?.ready ? "取消准备" : "准备就绪"}</button>}{view.phase === "playing" && !view.paused && canAct && <><button className="secondary" onClick={() => send("draw.offer")}>提议和棋</button><button className="danger-button" onClick={() => send("resign")}>认输</button></>}{view.phase === "finished" && canAct && <button className="primary" onClick={() => send("rematch.ready", { ready: true })}>准备再来一局</button>}{view.drawOffer && view.drawOffer !== mine && !view.paused && canAct && <button className="primary" onClick={() => send("draw.respond", { accept: true })}>接受和棋</button>}</div>;
}

function EventPanel({ view }: { view: View }) { return <div className="event-panel"><div className="panel-heading"><h2>战况记录</h2></div>{view.lastMove ? <div className="event-card"><span className="event-tag">最近行动</span><strong>{seatLabels[view.lastMove.seat]} · {view.lastMove.from} → {view.lastMove.to}</strong><span>{combatLabel(view.lastMove.result)}</span></div> : <p className="empty-copy">对局事件会显示在这里。<br />所有时钟由服务端计时。</p>}{view.drawOffer && <div className="draw-banner">{seatLabels[view.drawOffer]}方提议和棋</div>}{view.result && <div className="result-banner"><span>比赛结束</span><strong>{view.result.outcome === "draw" ? "和棋" : view.result.outcome === "stopped" ? "对局已停止" : `${teamLabel(view.result.team)} 获胜`}</strong><small>{view.result.reason}</small>{view.matchId && <a href={`/replay/${view.matchId}`}>查看完整回放 ↗</a>}</div>}</div>; }

function RoomControlPanel({ view, send }: { view: View; send: (type: string, payload?: unknown) => void }) {
  const activeSeats = seatsForMatchMode(view.matchMode);
  const allSeatsFilled = activeSeats.every((seat) => Boolean(view.players[seat]?.username));
  const active = view.phase === "setup" || view.phase === "playing";
  const canStop = active || view.paused;
  const requiredPlayersLabel = activeSeats.length === 2 ? "等待两名玩家入座" : "等待四名玩家入座";
  return <div className="room-controls room-control-panel"><div className="panel-heading"><h2>房间控制</h2></div><button className="primary" disabled={view.phase !== "lobby" || !allSeatsFilled} onClick={() => send("room.start")}>{view.phase === "lobby" ? allSeatsFilled ? "开始部署" : requiredPlayersLabel : "已开始"}</button><div className="room-control-row"><button className="secondary" disabled={view.phase !== "playing" || Boolean(view.paused)} onClick={() => send("room.pause")}>暂停对局</button><button className="secondary" disabled={!view.paused} onClick={() => send("room.resume")}>继续对局</button></div><div className="room-control-row"><button className="danger-button" disabled={!canStop} onClick={() => send("room.stop")}>停止对局</button><button className="secondary" disabled={view.phase === "lobby"} onClick={() => send("room.reset")}>重置房间</button></div><p className="fine-print">房间内所有成员都可以使用这些控制。</p></div>;
}

function RoomControls({ room, view, user, send }: { room: Room; view: View; user: string; send: (type: string, payload?: unknown) => void }) { const canChangeSettings = room.hostUsername === user; const spectatorCount = (view.participants ?? []).filter((participant) => participant.role === "spectator").length; const matchMode = matchModeFor(view); const canChangeMatchMode = view.phase === "lobby" || view.phase === "setup"; return <div className="room-controls"><div className="panel-heading"><h2>房间设置</h2></div><label>对局模式<select value={matchMode} disabled={!canChangeMatchMode} onChange={(e) => send("room.mode", { matchMode: e.target.value as MatchMode })}><option value="two_vs_two">2 vs 2</option><option value="one_vs_one">1 vs 1</option></select></label><label>信息模式<select value={view.mode} disabled={view.phase !== "lobby" || !canChangeSettings} onChange={(e) => send("settings.update", { mode: e.target.value as Mode, clock: view.clock })}><option value="four_dark">四暗</option><option value="double_visible">队友可见</option><option value="fully_visible">全明（玩家）</option></select></label><label>时钟<select value={view.clock} disabled={view.phase !== "lobby" || !canChangeSettings} onChange={(e) => send("settings.update", { mode: view.mode, clock: e.target.value as Clock })}><option value="fast">快速 · 20 秒</option><option value="standard">标准 · 60 秒</option><option value="relaxed">休闲 · 120 秒</option></select></label><p className="fine-print">{spectatorCount} / {room.spectatorCap} 位观众</p></div>; }

function phaseLabel(phase: Phase) { return ({ lobby: "等待入场", setup: "部署阵地", playing: "对局进行中", finished: "比赛结束" })[phase]; }
function modeLabel(mode: Mode) { return ({ four_dark: "四暗模式", double_visible: "队友可见", fully_visible: "全明模式" })[mode]; }
function matchModeLabel(matchMode?: MatchMode) { return matchModeFor({ matchMode }) === "one_vs_one" ? "1 vs 1" : "2 vs 2"; }
function clockLabel(clock: Clock) { return ({ fast: "快速时钟", standard: "标准时钟", relaxed: "休闲时钟" })[clock]; }
function teamLabel(team?: string) { return team === "north_south" ? "北南队" : team === "east_west" ? "东西队" : team === "north" ? "北方" : team === "south" ? "南方" : team ?? "未知队伍"; }
function combatLabel(result: string) { return ({ move: "移动", attacker_survives: "进攻方存活", defender_survives: "防守方存活", both_removed: "双方移除", engineer_cleared_mine: "工兵排雷", mine_survives: "地雷保留", mine_removed_attacker: "触雷" }[result] ?? result); }
