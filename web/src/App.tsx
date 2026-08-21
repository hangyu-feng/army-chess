import { useEffect, useMemo, useRef, useState } from "react";

type Seat = "north" | "east" | "south" | "west";
type Phase = "lobby" | "setup" | "playing" | "finished";
type Mode = "four_dark" | "double_visible" | "fully_visible";
type Clock = "fast" | "standard" | "relaxed";

type Player = { username: string; ready: boolean; connected: boolean; eliminated: boolean; misses: number };
type VisiblePiece = { id: string; owner: Seat; kind?: string; revealed?: boolean };
type Move = { seat: Seat; from: string; to: string; result: string };
type View = {
  matchId?: string;
  version: number;
  phase: Phase;
  mode: Mode;
  clock: Clock;
  turn?: Seat;
  deadline?: string;
  setupDeadline?: string;
  opening: Seat;
  players: Record<Seat, Player>;
  pieces: Record<string, VisiblePiece>;
  revealedFlags: Record<string, string>;
  legalMoves?: string[];
  lastMove?: Move;
  drawOffer?: Seat;
  result?: { outcome: string; team?: string; reason?: string };
};
type Room = { code: string; hostUsername?: string; phase: Phase; mode: Mode; clock: Clock; opening: Seat; participants: Participant[]; spectatorCap: number };
type Participant = { username: string; seat?: Seat; role: "player" | "spectator"; connected: boolean };
type ReplayState = { phase: Phase; turn?: Seat; pieces: Record<string, { owner: Seat; kind: string }>; lastMove?: Move; result?: { outcome: string; team?: string; reason?: string } };
type ReplayEvent = { sequence: number; type: string; payload: ReplayState; createdAt: string };
type ProfileSummary = { id: string; username: string; matches: number; wins: number; losses: number; draws: number };
type MatchSummary = { id: string; outcome: string; mode: Mode; clock: Clock; startedAt?: string; finishedAt?: string };

const seats: Seat[] = ["north", "east", "south", "west"];
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
  const [roomCode, setRoomCode] = useState<string | null>(null);
  const [error, setError] = useState("");
  const path = window.location.pathname;
  useEffect(() => { api<{ username: string }>("/api/me").then((data) => setUser(data.username)).catch(() => undefined); }, []);
  if (path.startsWith("/replay/")) return <ReplayScreen matchId={decodeURIComponent(path.slice("/replay/".length))} />;
  if (path.startsWith("/profile/")) return <ProfileScreen username={decodeURIComponent(path.slice("/profile/".length))} />;
  if (!user) return <SignIn onSignedIn={setUser} />;
  if (!roomCode) return <Home user={user} onRoom={(code) => { setError(""); setRoomCode(code); }} onError={setError} error={error} />;
  return <RoomScreen user={user} code={roomCode} onLeave={() => setRoomCode(null)} onError={setError} error={error} />;
}

function ReplayScreen({ matchId }: { matchId: string }) {
  const [events, setEvents] = useState<ReplayEvent[]>([]);
  const [index, setIndex] = useState(0);
  const [error, setError] = useState("");
  useEffect(() => { api<{ events: ReplayEvent[] }>(`/api/matches/${encodeURIComponent(matchId)}/replay`).then((data) => setEvents(data.events)).catch((err) => setError((err as Error).message)); }, [matchId]);
  const current = events[index]?.payload;
  return <main className="page-shell replay-shell"><header className="topbar"><a className="text-button" href="/">← 指挥台</a><span className="room-code">REPLAY / {matchId}</span><span className="user-chip">FULL TRUTH</span></header>{error ? <p className="error centered">{error}</p> : !current ? <div className="loading">正在加载回放…</div> : <section className="replay-layout"><div><div className="eyebrow">MATCH REPLAY</div><h1 className="replay-title">战局回放</h1><p className="lede">事件 {index + 1} / {events.length} · {current.phase === "finished" ? "比赛结束" : current.phase}</p><ReplayBoard state={current} /></div><aside className="replay-controls"><button className="secondary" disabled={index === 0} onClick={() => setIndex((value) => value - 1)}>← 上一步</button><button className="primary" disabled={index >= events.length - 1} onClick={() => setIndex((value) => value + 1)}>下一步 →</button><div className="event-card"><span className="event-tag">{events[index].type}</span><strong>序列 {events[index].sequence}</strong>{current.lastMove && <span>{seatLabels[current.lastMove.seat]} · {current.lastMove.from} → {current.lastMove.to}</span>}</div>{current.result && <div className="result-banner"><span>结果</span><strong>{current.result.outcome === "draw" ? "和棋" : `${teamLabel(current.result.team)} 获胜`}</strong><small>{current.result.reason}</small></div>}</aside></section>}</main>;
}

function ReplayBoard({ state }: { state: ReplayState }) {
  const nodes = Array.from({ length: 144 }, (_, index) => { const x = index % 12; const y = Math.floor(index / 12); return `n${String(x).padStart(2, "0")}_${String(y).padStart(2, "0")}`; });
  return <div className="board replay-board" role="grid" aria-label="回放棋盘">{nodes.map((node) => { const piece = state.pieces[node]; return <div key={node} className={`node ${piece ? `occupied ${piece.owner}` : ""} ${node.match(/n(05|06)_(05|06)/) ? "camp" : ""}`} role="gridcell">{piece && <><span className="piece-owner">{seatLabels[piece.owner]}</span><span className="piece-label">{pieceLabels[piece.kind] ?? piece.kind}</span></>}</div>; })}</div>;
}

function ProfileScreen({ username }: { username: string }) {
  const [summary, setSummary] = useState<ProfileSummary | null>(null);
  const [matches, setMatches] = useState<MatchSummary[]>([]);
  const [error, setError] = useState("");
  useEffect(() => { Promise.all([api<ProfileSummary>(`/api/profiles/${encodeURIComponent(username)}`), api<{ matches: MatchSummary[] }>(`/api/profiles/${encodeURIComponent(username)}/matches`)]).then(([profile, history]) => { setSummary(profile); setMatches(history.matches); }).catch((err) => setError((err as Error).message)); }, [username]);
  return <main className="page-shell"><header className="topbar"><a className="text-button" href="/">← 指挥台</a><span className="room-code">PROFILE / {username}</span><span className="user-chip">PUBLIC RECORD</span></header>{error ? <p className="error centered">{error}</p> : !summary ? <div className="loading">正在读取资料…</div> : <section className="profile-layout"><div className="profile-hero"><div className="eyebrow">PLAYER PROFILE</div><h1>{summary.username}</h1><p className="lede">无密码用户名资料 · 任何输入同名用户名的人都可以访问。</p><div className="stats-grid"><div><strong>{summary.matches}</strong><span>对局</span></div><div><strong>{summary.wins}</strong><span>胜利</span></div><div><strong>{summary.losses}</strong><span>失利</span></div><div><strong>{summary.draws}</strong><span>和棋</span></div></div></div><div className="history-panel"><div className="panel-heading"><span className="panel-index">05</span><h2>比赛历史</h2></div>{matches.length === 0 ? <p className="empty-copy">还没有可显示的比赛。</p> : matches.map((match) => <a className="history-row" href={`/replay/${match.id}`} key={match.id}><span>{match.finishedAt ? new Date(match.finishedAt).toLocaleDateString() : "进行中"}</span><strong>{match.outcome}</strong><small>{modeLabel(match.mode)} · {clockLabel(match.clock)}</small></a>)}</div></section>}</main>;
}

function SignIn({ onSignedIn }: { onSignedIn: (username: string) => void }) {
  const [username, setUsername] = useState("");
  const [error, setError] = useState("");
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    try { const data = await api<{ username: string }>("/api/session", { method: "POST", body: JSON.stringify({ username: username.trim().toLowerCase() }) }); onSignedIn(data.username); }
    catch (err) { setError((err as Error).message); }
  }
  return <main className="auth-shell"><section className="auth-card">
    <div className="eyebrow">TACTICAL COMMAND TABLE · V1</div><h1>四国军棋</h1><p className="lede">和朋友在一张自托管的战术棋盘上，完成一场 2 vs 2 对局。</p>
    <form onSubmit={submit}><label htmlFor="username">用户名</label><input id="username" autoFocus value={username} onChange={(e) => setUsername(e.target.value)} placeholder="例如 red_cedar" pattern="[a-z][a-z_]{1,18}[a-z]" required /><button type="submit">进入指挥台</button></form>
    {error && <p className="error">{error}</p>}<p className="fine-print">v1 使用无密码用户名登录。任何人输入同一用户名都可以访问该用户名的资料和对局，这是本版本明确接受的限制。</p>
  </section></main>;
}

function Home({ user, onRoom, onError, error }: { user: string; onRoom: (code: string) => void; onError: (message: string) => void; error: string }) {
  const [code, setCode] = useState("");
  async function create() { try { const room = await api<Room>("/api/rooms", { method: "POST", body: "{}" }); onRoom(room.code); } catch (err) { onError((err as Error).message); } }
  async function join(spectator = false) { try { const room = await api<Room>(`/api/rooms/${code.trim().toUpperCase()}/join`, { method: "POST", body: JSON.stringify({ spectator }) }); onRoom(room.code); } catch (err) { onError((err as Error).message); } }
  return <main className="page-shell"><header className="topbar"><div><span className="brand-mark">四</span><span className="brand-name">军棋 / COMMAND TABLE</span></div><a className="user-chip" href={`/profile/${encodeURIComponent(user)}`}>{user} ↗</a></header>
    <section className="home-grid"><div className="hero-panel"><div className="eyebrow">PRIVATE MULTIPLAYER ROOM</div><h1>布置你的阵地，<br /><em>等待队友入场。</em></h1><p>创建一个八位邀请码房间，四个座位就绪后开始对局。连接中断不会暂停时钟。</p><button className="primary large" onClick={create}>＋ 创建新房间</button></div>
    <div className="join-panel"><div className="panel-heading"><span className="panel-index">01</span><h2>加入房间</h2></div><label htmlFor="room-code">房间邀请码</label><input id="room-code" value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} maxLength={8} placeholder="ABCDEFGH" /><div className="button-row"><button className="primary" disabled={code.length !== 8} onClick={() => join(false)}>加入对局</button><button className="secondary" disabled={code.length !== 8} onClick={() => join(true)}>旁观</button></div><p className="fine-print">邀请码持有人可以占用空座位，或以旁观者身份加入。实时旁观永远看不到未公开的棋子等级。</p></div></section>
    {error && <p className="error centered">{error}</p>}<section className="feature-strip"><div><strong>四种视野</strong><span>四暗 / 队友可见 / 全明</span></div><div><strong>持久回放</strong><span>事件与快照可恢复</span></div><div><strong>自托管</strong><span>Docker Compose 部署</span></div></section>
  </main>;
}

function RoomScreen({ user, code, onLeave, onError, error }: { user: string; code: string; onLeave: () => void; onError: (message: string) => void; error: string }) {
  const [room, setRoom] = useState<Room | null>(null);
  const [view, setView] = useState<View | null>(null);
  const [joined, setJoined] = useState(false);
  const socketRef = useRef<WebSocket | null>(null);
  const [spectator, setSpectator] = useState(false);
  useEffect(() => {
    let alive = true;
    api<Room>(`/api/rooms/${code}`).then((data) => { if (alive) setRoom(data); }).catch((err) => onError((err as Error).message));
    return () => { alive = false; socketRef.current?.close(); };
  }, [code, onError]);
  async function join() {
    try { const data = await api<Room>(`/api/rooms/${code}/join`, { method: "POST", body: JSON.stringify({ spectator }) }); setRoom(data); setJoined(true); connect(); }
    catch (err) { onError((err as Error).message); }
  }
  function connect() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const socket = new WebSocket(`${protocol}//${window.location.host}/api/rooms/${code}/ws`);
    socket.onopen = () => { socketRef.current = socket; };
    socket.onmessage = (event) => { const message = JSON.parse(event.data) as { type: string; payload: View | { message: string } }; if (message.type === "snapshot") setView(message.payload as View); if (message.type === "error") onError((message.payload as { message: string }).message); };
    socket.onerror = () => onError("实时连接失败，请检查服务端状态");
    socket.onclose = () => { socketRef.current = null; };
  }
  function send(type: string, payload: unknown = {}) { socketRef.current?.send(JSON.stringify({ type, requestId: crypto.randomUUID(), payload })); }
  if (!room) return <main className="page-shell"><div className="loading">正在读取房间…</div></main>;
  if (!joined) return <main className="page-shell"><header className="topbar"><button className="text-button" onClick={onLeave}>← 返回</button><span className="room-code">{code}</span><span className="user-chip">{user}</span></header><section className="join-room-card"><div className="eyebrow">ROOM {room.code}</div><h1>选择你的进入方式</h1><p>当前阶段：{phaseLabel(room.phase)} · {room.participants.length} 人在线或已登记</p><div className="entry-options"><button className="primary" onClick={() => { setSpectator(false); join(); }}>作为玩家加入</button><button className="secondary" onClick={() => { setSpectator(true); join(); }}>进入旁观席</button></div></section></main>;
  return <main className="game-shell"><header className="topbar"><button className="text-button" onClick={onLeave}>← 退出房间</button><div className="room-code"><span>ROOM</span> {code}</div><span className="connection-dot">● 已连接</span></header>{error && <div className="toast error">{error}</div>}
    {!view ? <div className="loading">正在建立加密的实时连接…</div> : <div className="room-layout"><aside className="side-panel left"><RoomHeader room={room} view={view} /><SeatList view={view} user={user} send={send} spectator={spectator} /></aside><section className="board-panel"><Board view={view} user={user} send={send} spectator={spectator} /><GameActions view={view} user={user} send={send} spectator={spectator} /></section><aside className="side-panel right"><EventPanel view={view} /><RoomControls room={room} view={view} send={send} spectator={spectator} /></aside></div>}
  </main>;
}

function RoomHeader({ view }: { room: Room; view: View }) { return <div className="room-heading"><div className="eyebrow">{view.phase === "playing" ? "LIVE MATCH" : "ROOM LOBBY"}</div><h1>{phaseLabel(view.phase)}</h1><p>{modeLabel(view.mode)} · {clockLabel(view.clock)} · 开局：{seatLabels[view.opening]}</p></div>; }

function SeatList({ view, user, send, spectator }: { view: View; user: string; send: (type: string, payload?: unknown) => void; spectator: boolean }) {
  return <div className="seat-list"><div className="panel-heading"><span className="panel-index">02</span><h2>座位与队伍</h2></div>{seats.map((seat) => { const player = view.players[seat]; const mine = player?.username === user; return <div className={`seat-row ${mine ? "mine" : ""} ${view.turn === seat ? "turn" : ""}`} key={seat}><span className={`seat-badge ${seat}`}>{seatLabels[seat]}</span><div><strong>{player?.username || "等待玩家"}</strong><span>{player?.eliminated ? "已出局" : player?.connected ? (mine ? "你 · 已连接" : "在线") : player?.username ? "暂离" : "开放座位"}</span></div>{(view.phase === "lobby" || view.phase === "setup") && !spectator && !player?.username && <button className="mini-button" onClick={() => send("seat.select", { seat })}>入座</button>}{view.turn === seat && view.phase === "playing" && <span className="turn-mark">行动中</span>}</div>; })}<div className="team-key"><span><i className="team north-south" />北 / 南 · 同队</span><span><i className="team east-west" />东 / 西 · 同队</span></div></div>;
}

function Board({ view, user, send, spectator }: { view: View; user: string; send: (type: string, payload?: unknown) => void; spectator: boolean }) {
  const [selected, setSelected] = useState<string | null>(null);
  const [setupSwap, setSetupSwap] = useState<string | null>(null);
  const mySeat = seats.find((seat) => view.players[seat]?.username === user);
  const legal = useMemo(() => new Set((view.legalMoves ?? []).map((move) => move.replace("->", "|"))), [view.legalMoves]);
  const nodes = Array.from({ length: 144 }, (_, index) => { const x = index % 12; const y = Math.floor(index / 12); return `n${String(x).padStart(2, "0")}_${String(y).padStart(2, "0")}`; });
  function click(node: string) {
    const piece = view.pieces[node];
    if (view.phase === "setup" && piece?.owner === mySeat) {
      if (!setupSwap) { setSetupSwap(node); return; }
      const pieces: Record<string, { id: string; owner: Seat; kind: string }> = Object.fromEntries(Object.entries(view.pieces).filter(([, value]) => value.owner === mySeat && value.kind).map(([key, value]) => [key, { id: value.id, owner: value.owner, kind: value.kind! }]));
      const first = pieces[setupSwap]; const second = pieces[node]; if (first && second) { pieces[setupSwap] = { ...first, id: second.id, kind: second.kind }; pieces[node] = { ...second, id: first.id, kind: first.kind }; send("setup.replace", { pieces }); }
      setSetupSwap(null); return;
    }
    if (view.phase !== "playing" || spectator) return;
    if (selected && legal.has(`${selected}|${node}`)) { send("move", { from: selected, to: node }); setSelected(null); return; }
    if (piece?.owner === mySeat && piece.kind) setSelected(node); else setSelected(null);
  }
  return <div className="board-wrap"><div className="board-caption"><span>BOARD V1 · 12 × 12</span><span>{view.phase === "setup" ? "点击两枚己方棋子交换位置" : view.phase === "playing" ? "选择棋子，再选择高亮目标" : "等待四席就绪"}</span></div><div className="board" role="grid" aria-label="四国军棋棋盘">{nodes.map((node) => { const piece = view.pieces[node]; const target = selected ? legal.has(`${selected}|${node}`) : false; return <button key={node} className={`node ${node.includes("_05") || node.includes("_06") ? "middle-line" : ""} ${piece ? `occupied ${piece.owner}` : ""} ${target ? "target" : ""} ${selected === node || setupSwap === node ? "selected" : ""} ${node.match(/n(05|06)_(05|06)/) ? "camp" : ""}`} onClick={() => click(node)} role="gridcell" aria-label={`${node}${piece ? ` ${piece.kind ? pieceLabels[piece.kind] : "未知棋子"}` : " 空位"}`}>{piece && <><span className="piece-owner">{seatLabels[piece.owner]}</span><span className="piece-label">{piece.kind ? pieceLabels[piece.kind] : "?"}</span></>}</button>; })}</div></div>;
}

function GameActions({ view, user, send, spectator }: { view: View; user: string; send: (type: string, payload?: unknown) => void; spectator: boolean }) {
  const countdown = useCountdown(view.phase === "setup" ? view.setupDeadline : view.deadline);
  const mine = seats.find((seat) => view.players[seat]?.username === user);
  const player = mine ? view.players[mine] : undefined;
  return <div className="game-actions"><div className="clock-card"><span>{view.phase === "setup" ? "部署倒计时" : view.phase === "playing" ? `轮到${seatLabels[view.turn ?? "north"]}` : "比赛状态"}</span><strong>{countdown === null ? "—" : `${String(Math.floor(countdown / 60)).padStart(2, "0")}:${String(countdown % 60).padStart(2, "0")}`}</strong></div>{view.phase === "setup" && !spectator && <button className="primary" onClick={() => send("ready", { ready: !player?.ready })}>{player?.ready ? "取消准备" : "准备就绪"}</button>}{view.phase === "playing" && !spectator && <><button className="secondary" onClick={() => send("draw.offer")}>提议和棋</button><button className="danger-button" onClick={() => send("resign")}>认输</button></>}{view.phase === "finished" && !spectator && <button className="primary" onClick={() => send("rematch.ready", { ready: true })}>准备再来一局</button>}{view.drawOffer && view.drawOffer !== mine && <button className="primary" onClick={() => send("draw.respond", { accept: true })}>接受和棋</button>}</div>;
}

function EventPanel({ view }: { view: View }) { return <div className="event-panel"><div className="panel-heading"><span className="panel-index">03</span><h2>战况记录</h2></div>{view.lastMove ? <div className="event-card"><span className="event-tag">最近行动</span><strong>{seatLabels[view.lastMove.seat]} · {view.lastMove.from} → {view.lastMove.to}</strong><span>{combatLabel(view.lastMove.result)}</span></div> : <p className="empty-copy">对局事件会显示在这里。<br />所有时钟由服务端计时。</p>}{view.drawOffer && <div className="draw-banner">{seatLabels[view.drawOffer]}方提议和棋</div>}{view.result && <div className="result-banner"><span>比赛结束</span><strong>{view.result.outcome === "draw" ? "和棋" : `${teamLabel(view.result.team)} 获胜`}</strong><small>{view.result.reason}</small>{view.matchId && <a href={`/replay/${view.matchId}`}>查看完整回放 ↗</a>}</div>}</div>; }

function RoomControls({ room, view, send, spectator }: { room: Room; view: View; send: (type: string, payload?: unknown) => void; spectator: boolean }) { return <div className="room-controls"><div className="panel-heading"><span className="panel-index">04</span><h2>房间设置</h2></div><label>信息模式<select value={view.mode} disabled={view.phase !== "lobby" || spectator} onChange={(e) => send("settings.update", { mode: e.target.value as Mode, clock: view.clock })}><option value="four_dark">四暗</option><option value="double_visible">队友可见</option><option value="fully_visible">全明（玩家）</option></select></label><label>时钟<select value={view.clock} disabled={view.phase !== "lobby" || spectator} onChange={(e) => send("settings.update", { mode: view.mode, clock: e.target.value as Clock })}><option value="fast">快速 · 20 秒</option><option value="standard">标准 · 60 秒</option><option value="relaxed">休闲 · 120 秒</option></select></label><p className="fine-print">{room.participants.filter((p) => p.role === "spectator").length} / {room.spectatorCap} 位旁观者</p></div>; }

function phaseLabel(phase: Phase) { return ({ lobby: "等待入场", setup: "部署阵地", playing: "对局进行中", finished: "比赛结束" })[phase]; }
function modeLabel(mode: Mode) { return ({ four_dark: "四暗模式", double_visible: "队友可见", fully_visible: "全明模式" })[mode]; }
function clockLabel(clock: Clock) { return ({ fast: "快速时钟", standard: "标准时钟", relaxed: "休闲时钟" })[clock]; }
function teamLabel(team?: string) { return team === "north_south" ? "北南队" : "东西队"; }
function combatLabel(result: string) { return ({ move: "移动", attacker_survives: "进攻方存活", defender_survives: "防守方存活", both_removed: "双方移除", engineer_cleared_mine: "工兵排雷", mine_survives: "地雷保留", mine_removed_attacker: "触雷" }[result] ?? result); }
