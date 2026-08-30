import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "../src/App";

describe("login and home screens", () => {
  let container: HTMLDivElement;
  let root: Root;
  let signedIn: boolean;

  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = typeof input === "string" ? input : input.toString();
    if (path === "/api/config") {
      return Promise.resolve(new Response(JSON.stringify({ baseUrl: "https://chess.example.test" }), { status: 200, headers: { "Content-Type": "application/json" } }));
    }
    if (path === "/api/me") {
      return Promise.resolve(signedIn
        ? new Response(JSON.stringify({ username: "baihua" }), { status: 200, headers: { "Content-Type": "application/json" } })
        : new Response(JSON.stringify({ error: "not signed in" }), { status: 401, headers: { "Content-Type": "application/json" } }));
    }
    if (path === "/api/session" && init?.method === "DELETE") {
      signedIn = false;
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    if (path === "/api/rooms" && init?.method === "POST") {
      return Promise.resolve(new Response(JSON.stringify({ code: "ABCD2345", phase: "lobby", mode: "four_dark", clock: "standard", opening: "north", participants: [], spectatorCap: 50 }), { status: 201, headers: { "Content-Type": "application/json" } }));
    }
    if (path === "/api/rooms/ABCD2345") {
      return Promise.resolve(new Response(JSON.stringify({ code: "ABCD2345", phase: "lobby", mode: "four_dark", clock: "standard", opening: "north", participants: [], spectatorCap: 50 }), { status: 200, headers: { "Content-Type": "application/json" } }));
    }
    return Promise.resolve(new Response(JSON.stringify({}), { status: 200, headers: { "Content-Type": "application/json" } }));
  });

  beforeEach(() => {
    signedIn = false;
    fetchMock.mockClear();
    window.history.replaceState({}, "", "/");
    window.sessionStorage.clear();
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
    globalThis.fetch = fetchMock as typeof fetch;
    container = document.createElement("div");
    document.body.append(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  async function renderApp() {
    root = createRoot(container);
    await act(async () => {
      root.render(<App />);
      await Promise.resolve();
    });
  }

  async function waitFor(condition: () => boolean) {
    for (let attempt = 0; attempt < 20; attempt += 1) {
      if (condition()) return;
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
      });
    }
    throw new Error("condition was not met before timeout");
  }

  it("keeps the simplified login copy and CTA", async () => {
    await renderApp();

    expect(container.querySelector("h1")?.textContent).toBe("四国军棋");
    expect(container.querySelector("button[type=submit]")?.textContent).toBe("进入");
    expect(container.textContent).toContain("3–20 个小写英文字母和下划线，首尾为字母");
    expect(container.textContent).not.toContain("TACTICAL COMMAND TABLE");
    expect(container.textContent).not.toContain("自托管的战术棋盘");
    expect(container.textContent).not.toContain("v1 使用无密码用户名登录");
  });

  it("explains the username format when client-side validation fails", async () => {
    await renderApp();
    const input = container.querySelector("#username") as HTMLInputElement;
    input.value = "ab";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await act(async () => {
      (container.querySelector("button[type=submit]") as HTMLButtonElement).click();
      await Promise.resolve();
    });
    expect(container.textContent).toContain("用户名格式：3–20 个小写英文字母和下划线");
    expect(fetchMock.mock.calls.some(([input, init]) => input === "/api/session" && init?.method === "POST")).toBe(false);
  });

  it("shows the simplified home header and lets the user switch usernames", async () => {
    signedIn = true;
    await renderApp();
    await waitFor(() => container.querySelector(".home-grid") !== null);

    expect(container.querySelector(".brand-name")?.textContent).toBe("四国军棋");
    expect(container.querySelector(".brand-mark")).toBeNull();
    expect(container.querySelector(".feature-strip")).toBeNull();
    expect(container.querySelector("a.user-chip")).toBeNull();

    const switchButton = container.querySelector("button.switch-user") as HTMLButtonElement | null;
    expect(switchButton?.textContent).toContain("baihua");
    expect(switchButton?.textContent).toContain("切换用户名");

    await act(async () => {
      switchButton?.click();
      await Promise.resolve();
    });
    await waitFor(() => container.querySelector("#username") !== null);

    expect(signedIn).toBe(false);
    expect(fetchMock).toHaveBeenCalledWith("/api/session", expect.objectContaining({ method: "DELETE" }));
  });

  it("shows a configured room link and copies it", async () => {
    signedIn = true;
    await renderApp();
    await waitFor(() => container.querySelector(".home-grid") !== null);

    const createButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "创建新房间");
    await act(async () => {
      createButton?.click();
      await Promise.resolve();
    });
    await waitFor(() => container.querySelector(".join-room-card") !== null);

    const link = container.querySelector("input[aria-label='房间分享链接']") as HTMLInputElement | null;
    expect(window.location.pathname).toBe("/ABCD2345");
    expect(window.location.search).toBe("");
    expect(link?.value).toBe("https://chess.example.test/ABCD2345");
    expect(container.querySelector(".share-link")).toBeNull();
    expect(container.textContent).not.toContain("进入后默认在旁观席");

    const copyButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "复制");
    await act(async () => {
      copyButton?.click();
      await Promise.resolve();
    });
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("https://chess.example.test/ABCD2345");
    expect(container.textContent).toContain("已复制");
  });
});

class TestWebSocket {
  static current: TestWebSocket | null = null;
  readonly sent: string[] = [];
  readonly url: string;
  readyState = 0;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(url: string) {
    this.url = url;
    TestWebSocket.current = this;
    queueMicrotask(() => {
      this.readyState = 1;
      this.onopen?.();
    });
  }

  send(value: string) {
    this.sent.push(value);
  }

  emit(payload: unknown) {
    this.onmessage?.({ data: JSON.stringify(payload) });
  }

  close() {
    this.readyState = 3;
    this.onclose?.();
  }

  fail() {
    this.onerror?.();
    this.close();
  }
}

describe("room board interactions", () => {
  let container: HTMLDivElement;
  let root: Root;
  let originalWebSocket: typeof WebSocket;

  const room = { code: "ABCD2345", phase: "lobby", mode: "four_dark", clock: "standard", opening: "north", participants: [], spectatorCap: 50 };
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const path = typeof input === "string" ? input : input.toString();
    if (path === "/api/config") return Promise.resolve(new Response(JSON.stringify({ baseUrl: "https://chess.example.test" }), { status: 200 }));
    if (path === "/api/me") return Promise.resolve(new Response(JSON.stringify({ username: "baihua" }), { status: 200 }));
    if (path === "/api/rooms/ABCD2345" || path === "/api/rooms/ABCD2345/join") return Promise.resolve(new Response(JSON.stringify(room), { status: 200 }));
    if (path === "/api/rooms/MISSING1") return Promise.resolve(new Response(JSON.stringify({ error: "room not found" }), { status: 404 }));
    return Promise.resolve(new Response(JSON.stringify({}), { status: 200 }));
  });

  beforeEach(() => {
    originalWebSocket = globalThis.WebSocket;
    globalThis.WebSocket = TestWebSocket as unknown as typeof WebSocket;
    TestWebSocket.current = null;
    fetchMock.mockClear();
    window.sessionStorage.clear();
    window.history.replaceState({}, "", "/?room=ABCD2345");
    globalThis.fetch = fetchMock as typeof fetch;
    container = document.createElement("div");
    document.body.append(container);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockResolvedValue(undefined) } });
  });

  afterEach(() => {
    act(() => root.unmount());
    globalThis.WebSocket = originalWebSocket;
    container.remove();
  });

  async function renderApp() {
    root = createRoot(container);
    await act(async () => {
      root.render(<App />);
      await Promise.resolve();
    });
  }

  async function waitFor(condition: () => boolean) {
    for (let attempt = 0; attempt < 30; attempt += 1) {
      if (condition()) return;
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 0));
      });
    }
    throw new Error("condition was not met before timeout");
  }

  it("offers a way back to the room entry page when a room cannot be loaded", async () => {
    window.history.replaceState({}, "", "/?room=MISSING1");
    await renderApp();
    await waitFor(() => container.querySelector(".room-load-error") !== null);

    expect(container.textContent).toContain("房间不存在或已过期");
    const back = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "返回房间入口");
    await act(async () => {
      back?.click();
      await Promise.resolve();
    });
    await waitFor(() => container.querySelector(".home-grid") !== null);
    expect(window.location.search).toBe("");
  });

  it("restores a room from its canonical path after a refresh", async () => {
    window.history.replaceState({}, "", "/ABCD2345");
    window.sessionStorage.setItem("army-chess:room:ABCD2345:baihua", "joined");
    await renderApp();
    await waitFor(() => container.querySelector(".game-shell") !== null);

    expect(fetchMock.mock.calls.some(([input, init]) => input === "/api/rooms/ABCD2345/join" && init?.method === "POST")).toBe(true);
    expect(window.location.pathname).toBe("/ABCD2345");
  });

  it("normalizes legacy room query links to the canonical path", async () => {
    window.history.replaceState({}, "", "/?room=ABCD2345");
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);

    expect(window.location.pathname).toBe("/ABCD2345");
    expect(window.location.search).toBe("");
  });

  it("does not interpret non-eight-character paths as rooms", async () => {
    window.history.replaceState({}, "", "/ABC1234");
    await renderApp();
    await waitFor(() => container.querySelector(".home-grid") !== null);

    expect(fetchMock.mock.calls.some(([input]) => input === "/api/rooms/ABC1234")).toBe(false);
  });

  it("does not interpret room-shaped paths without a digit or with a leading digit as rooms", async () => {
    for (const path of ["/ABCDEFGH", "/2ABC3456"]) {
      window.history.replaceState({}, "", path);
      await renderApp();
      await waitFor(() => container.querySelector(".home-grid") !== null);
      expect(fetchMock.mock.calls.some(([input]) => input === `/api/rooms/${path.slice(1)}`)).toBe(false);
      act(() => root.unmount());
    }
  });

  it("renders the classic board and sends a lobby arrangement swap", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    const joinRequest = fetchMock.mock.calls.find(([input, init]) => input === "/api/rooms/ABCD2345/join" && init?.method === "POST");
    expect(joinRequest?.[1]?.body).toBe("{}");
    await waitFor(() => TestWebSocket.current?.readyState === 1);
    TestWebSocket.current?.emit({
      type: "snapshot",
      payload: {
        version: 1,
        phase: "lobby",
        mode: "four_dark",
        clock: "standard",
        opening: "north",
        players: {
          north: { username: "baihua", ready: false, connected: true, eliminated: false, misses: 0 },
          east: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          south: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          west: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
        },
        pieces: {
          "north-r1-1L": { id: "north-1", owner: "north", kind: "company" },
          "north-r1-1R": { id: "north-2", owner: "north", kind: "engineer" },
        },
        revealedFlags: {},
      },
    });
    await waitFor(() => container.querySelector("svg.board-svg") !== null);

    const board = container.querySelector("svg.board-svg") as SVGSVGElement;
    expect(board.getAttribute("viewBox")).toBe("80 80 840 840");
    expect(container.querySelectorAll("circle")).toHaveLength(0);
    expect(container.querySelectorAll("rect.board-slot")).toHaveLength(129);
    expect(Array.from(container.querySelectorAll("rect.board-slot")).some((slot) => slot.getAttribute("width") === "38" && slot.getAttribute("height") === "32")).toBe(true);
    expect(Array.from(container.querySelectorAll("rect.board-slot")).some((slot) => slot.getAttribute("width") === "40" && slot.getAttribute("height") === "36" && slot.getAttribute("rx") === "9")).toBe(true);
    expect(container.querySelectorAll(".setup-slot")).toHaveLength(8);
    expect(container.querySelector(".board-stage.with-setup-tray")).not.toBeNull();
    expect(container.textContent).toContain("可放入右侧临时区");
    const leaveButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "离座");
    await act(async () => {
      leaveButton?.click();
      await Promise.resolve();
    });
    const leaveMessage = JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}") as { type?: string };
    expect(leaveMessage.type).toBe("seat.leave");

    const first = container.querySelector('[aria-label^="north-r1-1L"]') as SVGGElement;
    const second = container.querySelector('[aria-label^="north-r1-1R"]') as SVGGElement;
    await act(async () => {
      first.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });
    expect(first.classList.contains("selected")).toBe(true);
    const firstTraySlot = container.querySelector(".setup-slot") as HTMLButtonElement;
    await act(async () => {
      firstTraySlot.click();
      await Promise.resolve();
    });
    expect(first.classList.contains("occupied")).toBe(false);
    expect(firstTraySlot.classList.contains("occupied")).toBe(true);
    await act(async () => {
      firstTraySlot.click();
      await Promise.resolve();
    });
    expect(firstTraySlot.classList.contains("selected")).toBe(true);
    await act(async () => {
      first.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });
    expect(first.classList.contains("occupied")).toBe(true);
    await act(async () => {
      first.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });
    expect(first.classList.contains("selected")).toBe(true);
    await act(async () => {
      second.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });
    const message = JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}") as { type: string; payload: { pieces: Record<string, { id: string; kind: string }> } };
    expect(message.type).toBe("setup.replace");
    expect(message.payload.pieces["north-r1-1L"]).toMatchObject({ id: "north-2", kind: "engineer" });
    expect(message.payload.pieces["north-r1-1R"]).toMatchObject({ id: "north-1", kind: "company" });
  });

  it("shows the lobby start control once all four seats are filled", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    await waitFor(() => TestWebSocket.current?.readyState === 1);

    const baseView = {
      version: 1,
      phase: "lobby",
      mode: "four_dark",
      clock: "standard",
      opening: "north",
      pieces: {},
      revealedFlags: {},
    };
    const players = {
      north: { username: "north_user", ready: false, connected: true, eliminated: false, misses: 0 },
      east: { username: "baihua", ready: false, connected: true, eliminated: false, misses: 0 },
      south: { username: "south_user", ready: false, connected: true, eliminated: false, misses: 0 },
      west: { username: "west_user", ready: false, connected: true, eliminated: false, misses: 0 },
    };
    const participants = [
      { username: "north_user", seat: "north", role: "player", connected: true },
      { username: "baihua", seat: "east", role: "player", connected: true, self: true },
      { username: "south_user", seat: "south", role: "player", connected: true },
      { username: "west_user", seat: "west", role: "player", connected: true },
    ];
    TestWebSocket.current?.emit({ type: "snapshot", payload: { ...baseView, players, participants } });
    await waitFor(() => container.querySelector(".game-actions") !== null);

    const startButton = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "开始部署") as HTMLButtonElement | undefined;
    expect(startButton).toBeDefined();
    expect(startButton?.disabled).toBe(false);
    await act(async () => {
      startButton?.click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "room.start" });
  });

  it("lists every spectator and lets the current user take and leave a seat", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    await waitFor(() => TestWebSocket.current?.readyState === 1);

    const baseView = {
      version: 1,
      phase: "lobby",
      mode: "four_dark",
      clock: "standard",
      opening: "north",
      pieces: {},
      revealedFlags: {},
    };
    TestWebSocket.current?.emit({
      type: "snapshot",
      payload: {
        ...baseView,
        participants: [
          { username: "baihua", role: "spectator", connected: true, self: true },
          { username: "observer", role: "spectator", connected: true },
          { username: "away", role: "spectator", connected: false },
          { username: "north_user", seat: "north", role: "player", connected: true },
        ],
        players: {
          north: { username: "north_user", ready: false, connected: true, eliminated: false, misses: 0 },
          east: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          south: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          west: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
        },
      },
    });
    await waitFor(() => container.querySelectorAll(".spectator-row").length === 3);
    expect(container.querySelector(".room-control-panel .panel-heading h2")?.textContent).toBe("房间控制");
    expect(container.querySelector(".room-control-panel")?.textContent).toContain("房间内所有成员都可以使用这些控制");
    const unavailableStart = Array.from(container.querySelectorAll(".room-control-panel button")).find((button) => button.textContent === "等待四名玩家入座") as HTMLButtonElement | undefined;
    expect(unavailableStart?.disabled).toBe(true);
    const seatList = container.querySelector(".seat-list") as HTMLElement;
    expect(seatList.querySelector(".panel-heading h2")?.textContent).toBe("座位与队伍");
    expect(seatList.querySelector(".spectator-list .panel-heading h2")?.textContent).toBe("观众");
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("baihua");
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("observer");
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("away");

    const eastRow = Array.from(container.querySelectorAll(".seat-row")).find((row) => row.querySelector(".seat-badge.east")) as HTMLElement;
    await act(async () => {
      (eastRow.querySelector("button") as HTMLButtonElement).click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "seat.select", payload: { seat: "east" } });

    TestWebSocket.current?.emit({
      type: "snapshot",
      payload: {
        ...baseView,
        version: 2,
        participants: [
          { username: "baihua", seat: "east", role: "player", connected: true, self: true },
          { username: "observer", role: "spectator", connected: true },
          { username: "away", role: "spectator", connected: false },
          { username: "north_user", seat: "north", role: "player", connected: true },
        ],
        players: {
          north: { username: "north_user", ready: false, connected: true, eliminated: false, misses: 0 },
          east: { username: "baihua", ready: false, connected: true, eliminated: false, misses: 0 },
          south: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          west: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
        },
      },
    });
    await waitFor(() => container.querySelectorAll(".spectator-row").length === 2);
    expect(seatList.querySelector(".spectator-list")?.textContent).not.toContain("baihua");
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("observer");
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("away");
    const seatedEastRow = Array.from(container.querySelectorAll(".seat-row")).find((row) => row.querySelector(".seat-badge.east")) as HTMLElement;
    expect(seatedEastRow.textContent).toContain("baihua");
    expect(seatedEastRow.querySelector("button")?.textContent).toBe("离座");

    await act(async () => {
      (seatedEastRow.querySelector("button") as HTMLButtonElement).click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "seat.leave" });

    TestWebSocket.current?.emit({
      type: "snapshot",
      payload: {
        ...baseView,
        version: 3,
        participants: [
          { username: "baihua", role: "spectator", connected: true, self: true },
          { username: "observer", role: "spectator", connected: true },
          { username: "away", role: "spectator", connected: false },
          { username: "north_user", seat: "north", role: "player", connected: true },
        ],
        players: {
          north: { username: "north_user", ready: false, connected: true, eliminated: false, misses: 0 },
          east: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          south: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
          west: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
        },
      },
    });
    await waitFor(() => container.querySelectorAll(".spectator-row").length === 3);
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("baihua");
    expect(seatList.querySelector(".spectator-list")?.textContent).toContain("observer");
  });

  it("lets a spectator use the shared pause, resume, stop, and reset controls", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    await waitFor(() => TestWebSocket.current?.readyState === 1);

    const players = {
      north: { username: "north_user", ready: true, connected: true, eliminated: false, misses: 0 },
      east: { username: "east_user", ready: true, connected: true, eliminated: false, misses: 0 },
      south: { username: "south_user", ready: true, connected: true, eliminated: false, misses: 0 },
      west: { username: "west_user", ready: true, connected: true, eliminated: false, misses: 0 },
    };
    const participants = [
      { username: "baihua", role: "spectator", connected: true, self: true },
      { username: "north_user", seat: "north", role: "player", connected: true },
      { username: "east_user", seat: "east", role: "player", connected: true },
      { username: "south_user", seat: "south", role: "player", connected: true },
      { username: "west_user", seat: "west", role: "player", connected: true },
    ];
    const view = { version: 1, phase: "playing", mode: "four_dark", clock: "standard", opening: "north", players, participants, pieces: {}, revealedFlags: {} };
    TestWebSocket.current?.emit({ type: "snapshot", payload: view });
    await waitFor(() => container.querySelector(".room-control-panel") !== null);

    const button = (label: string) => Array.from(container.querySelectorAll(".room-control-panel button")).find((candidate) => candidate.textContent === label) as HTMLButtonElement;
    await act(async () => {
      button("暂停对局").click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "room.pause" });

    TestWebSocket.current?.emit({ type: "snapshot", payload: { ...view, version: 2, paused: true } });
    await waitFor(() => button("继续对局")?.disabled === false);
    await act(async () => {
      button("继续对局").click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "room.resume" });

    await act(async () => {
      button("停止对局").click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "room.stop" });
    TestWebSocket.current?.emit({ type: "snapshot", payload: { ...view, version: 3, phase: "finished", result: { outcome: "stopped", reason: "manual_stop" } } });
    await waitFor(() => button("重置房间")?.disabled === false);
    await act(async () => {
      button("重置房间").click();
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "room.reset" });
  });

  it("keeps playable moves unhighlighted and marks the current player's turn", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    await waitFor(() => TestWebSocket.current?.readyState === 1);

    TestWebSocket.current?.emit({
      type: "snapshot",
      payload: {
        version: 1,
        phase: "playing",
        mode: "four_dark",
        clock: "standard",
        opening: "east",
        turn: "east",
        deadline: new Date(Date.now() + 45_000).toISOString(),
        players: { east: { username: "baihua", ready: true, connected: true, eliminated: false, misses: 0 } },
        participants: [{ username: "baihua", seat: "east", role: "player", connected: true, self: true }],
        pieces: { "north-r1-1L": { id: "east-1", owner: "east", kind: "company" } },
        legalMoves: ["north-r1-1L->north-r1-1R"],
        revealedFlags: {},
      },
    });
    await waitFor(() => container.querySelector('[aria-label^="north-r1-1L"]') !== null);

    expect(container.querySelector('[aria-label^="north-r1-1L"]')?.classList.contains("playable")).toBe(false);
    expect(container.querySelector(".clock-card.your-turn")).not.toBeNull();
    expect(container.querySelector(".clock-card.your-turn.turn-east")).not.toBeNull();
    expect(container.querySelector(".clock-card")?.textContent).toContain("你的回合");
  });

  it("renders the 1v1 seat set and lets any room member change match mode", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    await waitFor(() => TestWebSocket.current?.readyState === 1);

    TestWebSocket.current?.emit({
      type: "snapshot",
      payload: {
        version: 1,
        phase: "lobby",
        matchMode: "one_vs_one",
        mode: "four_dark",
        clock: "standard",
        opening: "north",
        players: {
          north: { username: "north_user", ready: false, connected: true, eliminated: false, misses: 0 },
          south: { username: "", ready: false, connected: false, eliminated: false, misses: 0 },
        },
        participants: [{ username: "baihua", role: "spectator", connected: true, self: true }],
        pieces: {},
        revealedFlags: {},
      },
    });
    await waitFor(() => container.querySelector(".room-control-panel") !== null);

    expect(container.querySelectorAll(".seat-row")).toHaveLength(2);
    expect(container.querySelector(".seat-badge.east")).toBeNull();
    expect(container.querySelector("svg.board-svg.board-1v1")).not.toBeNull();
    expect(container.querySelector("svg.board-svg")?.getAttribute("viewBox")).toBe("0 0 760 1260");
    expect(container.querySelectorAll("rect.board-slot")).toHaveLength(65);
    expect(container.querySelectorAll(".board-node.frontline")).toHaveLength(3);
    expect(container.querySelectorAll(".board-node.mountain")).toHaveLength(2);
    expect(container.textContent).toContain("等待两名玩家入座");
    expect(container.querySelector(".left .clock-card")).not.toBeNull();
    expect(container.querySelector(".board-panel .clock-card")).toBeNull();

    const matchMode = container.querySelector("select") as HTMLSelectElement;
    expect(matchMode.value).toBe("one_vs_one");
    expect(matchMode.disabled).toBe(false);
    await act(async () => {
      matchMode.value = "two_vs_two";
      matchMode.dispatchEvent(new Event("change", { bubbles: true }));
      await Promise.resolve();
    });
    expect(JSON.parse(TestWebSocket.current?.sent.at(-1) ?? "{}")).toMatchObject({ type: "room.mode", payload: { matchMode: "two_vs_two" } });
  });

  it("shows realtime connection failure inline instead of as a popup", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const join = Array.from(container.querySelectorAll("button")).find((button) => button.textContent === "进入");
    await act(async () => {
      join?.click();
      await Promise.resolve();
    });
    await waitFor(() => TestWebSocket.current?.readyState === 1);
    TestWebSocket.current?.fail();
    await waitFor(() => container.querySelector(".connection-status.error") !== null);

    expect(container.querySelector(".connection-status.error")?.textContent).toContain("连接失败");
    expect(container.querySelector(".toast")).toBeNull();
  });

  it("returns from the room entry screen to username selection", async () => {
    await renderApp();
    await waitFor(() => container.querySelector(".join-room-card") !== null);
    const back = Array.from(container.querySelectorAll("button")).find((button) => button.textContent?.includes("返回"));
    await act(async () => {
      back?.click();
      await Promise.resolve();
    });
    expect(window.location.search).toBe("");
    expect(container.querySelector(".home-grid")).not.toBeNull();
  });
});
