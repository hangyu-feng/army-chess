export type BoardSeat = "north" | "east" | "south" | "west";
export type BoardNodeType = "station" | "camp" | "headquarters" | "frontline" | "mountain";
export type BoardEdgeType = "road" | "rail";

export type BoardNode = {
  id: string;
  x: number;
  y: number;
  type: BoardNodeType;
  deployFor?: BoardSeat;
  row?: number;
  column?: string;
};

export type BoardEdge = { from: string; to: string; type: BoardEdgeType };
export type BoardDefinition = { version: string; width: number; height: number; nodes: Record<string, BoardNode>; edges: BoardEdge[] };

const columns = ["1L", "2L", "3", "2R", "1R"];
const seats: BoardSeat[] = ["north", "east", "south", "west"];

function localID(seat: BoardSeat, row: number, col: number) {
  return `${seat}-r${row}-${columns[col]}`;
}

function rotate(seat: BoardSeat, x: number, y: number): [number, number] {
  if (seat === "north") return [1000 - x, 1000 - y];
  if (seat === "east") return [y, 1000 - x];
  if (seat === "west") return [1000 - y, x];
  return [x, y];
}

function buildBoard(): BoardDefinition {
  const nodes: Record<string, BoardNode> = {};
  const edges: BoardEdge[] = [];
  const edgeKeys = new Set<string>();
  const gridStep = 47;
  const formationStart = 406;
  const frontRow = 648;

  function addEdge(from: string, to: string, type: BoardEdgeType) {
    const key = [from, to].sort().join("|");
    if (edgeKeys.has(key)) return;
    edgeKeys.add(key);
    edges.push({ from, to, type });
  }

  for (const seat of seats) {
    for (let row = 1; row <= 6; row += 1) {
      for (let col = 0; col < columns.length; col += 1) {
        const [x, y] = rotate(seat, formationStart + col * gridStep, frontRow + (row - 1) * gridStep);
        const camp = (row === 2 || row === 4) && (col === 1 || col === 3) || row === 3 && col === 2;
        const headquarters = row === 6 && (col === 1 || col === 3);
        nodes[localID(seat, row, col)] = {
          id: localID(seat, row, col), x, y,
          type: camp ? "camp" : headquarters ? "headquarters" : "station",
          deployFor: seat, row, column: columns[col],
        };
      }
    }
  }

  const palace: Record<string, [number, number]> = {
    "palace-nw": [410, 410], "palace-n": [500, 410], "palace-ne": [590, 410],
    "palace-w": [410, 500], "palace-c": [500, 500], "palace-e": [590, 500],
    "palace-sw": [410, 590], "palace-s": [500, 590], "palace-se": [590, 590],
  };
  for (const [id, [x, y]] of Object.entries(palace)) nodes[id] = { id, x, y, type: "station" };

  for (const seat of seats) {
    for (const row of [2, 3, 4, 6]) for (let col = 0; col < 4; col += 1) addEdge(localID(seat, row, col), localID(seat, row, col + 1), "road");
    for (const col of [1, 2, 3]) for (let row = 2; row < 6; row += 1) addEdge(localID(seat, row, col), localID(seat, row + 1, col), "road");
    for (const col of [0, 4]) addEdge(localID(seat, 5, col), localID(seat, 6, col), "road");
    for (const row of [1, 5]) for (let col = 0; col < 4; col += 1) addEdge(localID(seat, row, col), localID(seat, row, col + 1), "rail");
    for (const col of [0, 4]) for (let row = 1; row < 5; row += 1) addEdge(localID(seat, row, col), localID(seat, row + 1, col), "rail");

    for (const [row, col] of [[2, 1], [2, 3], [3, 2], [4, 1], [4, 3]]) {
      for (const [deltaRow, deltaCol] of [[-1, -1], [-1, 1], [1, -1], [1, 1]]) {
        addEdge(localID(seat, row, col), localID(seat, row + deltaRow, col + deltaCol), "road");
      }
    }
  }

  const palaceRows = [["palace-nw", "palace-n", "palace-ne"], ["palace-w", "palace-c", "palace-e"], ["palace-sw", "palace-s", "palace-se"]];
  for (const row of palaceRows) for (let col = 0; col < 2; col += 1) addEdge(row[col], row[col + 1], "rail");
  for (let col = 0; col < 3; col += 1) for (let row = 0; row < 2; row += 1) addEdge(palaceRows[row][col], palaceRows[row + 1][col], "rail");

  const interfaces: Record<BoardSeat, Record<number, string>> = {
    north: { 0: "palace-ne", 2: "palace-n", 4: "palace-nw" },
    east: { 0: "palace-se", 2: "palace-e", 4: "palace-ne" },
    south: { 0: "palace-sw", 2: "palace-s", 4: "palace-se" },
    west: { 0: "palace-nw", 2: "palace-w", 4: "palace-sw" },
  };
  for (const seat of seats) for (const [col, palaceID] of Object.entries(interfaces[seat])) addEdge(localID(seat, 1, Number(col)), palaceID, "rail");
  addEdge(localID("south", 1, 4), localID("east", 1, 0), "rail");
  addEdge(localID("east", 1, 4), localID("north", 1, 0), "rail");
  addEdge(localID("north", 1, 4), localID("west", 1, 0), "rail");
  addEdge(localID("west", 1, 4), localID("south", 1, 0), "rail");

  return { version: "board.v2", width: 1000, height: 1000, nodes, edges };
}

export const boardDefinition = buildBoard();

function buildOneVsOneBoard(): BoardDefinition {
  const nodes: Record<string, BoardNode> = {};
  const edges: BoardEdge[] = [];
  const edgeKeys = new Set<string>();
  const columns = ["1L", "2L", "3", "2R", "1R"];

  function addEdge(from: string, to: string, type: BoardEdgeType) {
    const key = [from, to].sort().join("|");
    if (edgeKeys.has(key)) return;
    edgeKeys.add(key);
    edges.push({ from, to, type });
  }

  function addZone(seat: "north" | "south", startY: number) {
    for (let row = 1; row <= 6; row += 1) {
      for (let col = 0; col < columns.length; col += 1) {
        const id = `${seat}-r${row}-${columns[col]}`;
        const camp = (row === 2 || row === 4) && (col === 1 || col === 3) || row === 3 && col === 2;
        const headquarters = row === 6 && (col === 1 || col === 3);
        nodes[id] = {
          id, x: 100 + col * 140, y: startY + (row - 1) * 75,
          type: camp ? "camp" : headquarters ? "headquarters" : "station",
          deployFor: seat, row, column: columns[col],
        };
      }
    }

    for (const row of [2, 3, 4, 6]) for (let col = 0; col < 4; col += 1) addEdge(`${seat}-r${row}-${columns[col]}`, `${seat}-r${row}-${columns[col + 1]}`, "road");
    for (const col of [1, 2, 3]) for (let row = 2; row < 6; row += 1) addEdge(`${seat}-r${row}-${columns[col]}`, `${seat}-r${row + 1}-${columns[col]}`, "road");
    for (const col of [0, 4]) addEdge(`${seat}-r5-${columns[col]}`, `${seat}-r6-${columns[col]}`, "road");
    for (const row of [1, 5]) for (let col = 0; col < 4; col += 1) addEdge(`${seat}-r${row}-${columns[col]}`, `${seat}-r${row}-${columns[col + 1]}`, "rail");
    for (const col of [0, 4]) for (let row = 1; row < 5; row += 1) addEdge(`${seat}-r${row}-${columns[col]}`, `${seat}-r${row + 1}-${columns[col]}`, "rail");

    for (const [row, col] of [[2, 1], [2, 3], [3, 2], [4, 1], [4, 3]]) {
      for (const [deltaRow, deltaCol] of [[-1, -1], [-1, 1], [1, -1], [1, 1]]) {
        addEdge(`${seat}-r${row}-${columns[col]}`, `${seat}-r${row + deltaRow}-${columns[col + deltaCol]}`, "road");
      }
    }
  }

  addZone("north", 100);
  addZone("south", 785);
  for (const [id, x, type] of [
    ["frontline-1L", 100, "frontline"], ["mountain-2L", 240, "mountain"],
    ["frontline-3", 380, "frontline"], ["mountain-2R", 520, "mountain"],
    ["frontline-1R", 660, "frontline"],
  ] as const) nodes[id] = { id, x, y: 630, type };
  for (const [top, frontline, bottom] of [
    ["north-r1-1L", "frontline-1L", "south-r1-1L"],
    ["north-r1-3", "frontline-3", "south-r1-3"],
    ["north-r1-1R", "frontline-1R", "south-r1-1R"],
  ]) {
    addEdge(top, frontline, "rail");
    addEdge(frontline, bottom, "rail");
  }
  return { version: "board.1v1", width: 760, height: 1260, nodes, edges };
}

export const oneVsOneBoardDefinition = buildOneVsOneBoard();

export function boardForMatchMode(matchMode?: "two_vs_two" | "one_vs_one"): BoardDefinition {
  return matchMode === "one_vs_one" ? oneVsOneBoardDefinition : boardDefinition;
}
