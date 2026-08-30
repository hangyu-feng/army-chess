import { describe, expect, it } from "vitest";
import { boardDefinition, boardForMatchMode, oneVsOneBoardDefinition } from "../src/board";

describe("classic Army Chess board", () => {
  it("contains four 30-position zones and the nine-palace", () => {
    expect(Object.keys(boardDefinition.nodes)).toHaveLength(129);
    expect(Object.values(boardDefinition.nodes).filter((node) => node.type === "camp")).toHaveLength(20);
    expect(Object.values(boardDefinition.nodes).filter((node) => node.type === "headquarters")).toHaveLength(8);
    for (const seat of ["north", "east", "south", "west"] as const) {
      const zone = Object.values(boardDefinition.nodes).filter((node) => node.deployFor === seat);
      expect(zone).toHaveLength(30);
      expect(zone.filter((node) => node.type !== "camp")).toHaveLength(25);
    }
    expect(Object.keys(boardDefinition.nodes).filter((id) => id.startsWith("palace-")).sort()).toEqual([
      "palace-c", "palace-e", "palace-n", "palace-ne", "palace-nw", "palace-s", "palace-se", "palace-sw", "palace-w",
    ]);
  });

  it("keeps both road and railway edges", () => {
    expect(boardDefinition.edges.some((edge) => edge.type === "road")).toBe(true);
    expect(boardDefinition.edges.some((edge) => edge.type === "rail")).toBe(true);
    expect(boardDefinition.edges).toHaveLength(276);
    expect(boardDefinition.edges.filter((edge) => edge.type === "road")).toHaveLength(184);
    expect(boardDefinition.edges.filter((edge) => edge.type === "rail")).toHaveLength(92);
    expect(new Set(boardDefinition.edges.map((edge) => [edge.from, edge.to].sort().join("|"))).size).toBe(276);
  });

  it("rotates each country zone around the shared center", () => {
    expect(boardDefinition.nodes["north-r1-1L"]).toMatchObject({ x: 594, y: 352, deployFor: "north" });
    expect(boardDefinition.nodes["east-r1-1L"]).toMatchObject({ x: 648, y: 594, deployFor: "east" });
    expect(boardDefinition.nodes["south-r1-1L"]).toMatchObject({ x: 406, y: 648, deployFor: "south" });
    expect(boardDefinition.nodes["west-r1-1L"]).toMatchObject({ x: 352, y: 406, deployFor: "west" });
  });

  it("connects the palace and the four outer rail corners", () => {
    const edgeKeys = new Set(boardDefinition.edges.map((edge) => [edge.from, edge.to].sort().join("|")));
    for (const edge of [
      ["north-r1-1L", "palace-ne"], ["east-r1-1L", "palace-se"],
      ["south-r1-1L", "palace-sw"], ["west-r1-1L", "palace-nw"],
      ["south-r1-1R", "east-r1-1L"], ["east-r1-1R", "north-r1-1L"],
      ["north-r1-1R", "west-r1-1L"], ["west-r1-1R", "south-r1-1L"],
    ]) {
      expect(edgeKeys.has(edge.slice().sort().join("|"))).toBe(true);
    }
  });
});

describe("1v1 Army Chess board", () => {
  it("uses the dedicated 5x13 topology instead of the four-country board", () => {
    expect(boardForMatchMode("one_vs_one")).toBe(oneVsOneBoardDefinition);
    expect(oneVsOneBoardDefinition.version).toBe("board.1v1");
    expect(oneVsOneBoardDefinition.width).toBe(760);
    expect(oneVsOneBoardDefinition.height).toBe(1260);
    expect(Object.keys(oneVsOneBoardDefinition.nodes)).toHaveLength(65);
    expect(Object.values(oneVsOneBoardDefinition.nodes).filter((node) => node.type === "frontline")).toHaveLength(3);
    expect(Object.values(oneVsOneBoardDefinition.nodes).filter((node) => node.type === "mountain")).toHaveLength(2);
    for (const seat of ["north", "south"] as const) {
      const zone = Object.values(oneVsOneBoardDefinition.nodes).filter((node) => node.deployFor === seat);
      expect(zone).toHaveLength(30);
      expect(zone.filter((node) => node.type !== "camp")).toHaveLength(25);
    }
  });

  it("has three separate cross-country railways and no mountain edges", () => {
    const edgeKeys = new Set(oneVsOneBoardDefinition.edges.map((edge) => [edge.from, edge.to].sort().join("|")));
    for (const edge of [
      ["north-r1-1L", "frontline-1L"], ["frontline-1L", "south-r1-1L"],
      ["north-r1-3", "frontline-3"], ["frontline-3", "south-r1-3"],
      ["north-r1-1R", "frontline-1R"], ["frontline-1R", "south-r1-1R"],
    ]) expect(edgeKeys.has(edge.slice().sort().join("|"))).toBe(true);
    for (const mountain of ["mountain-2L", "mountain-2R"]) {
      expect(oneVsOneBoardDefinition.edges.some((edge) => edge.from === mountain || edge.to === mountain)).toBe(false);
    }
    expect(oneVsOneBoardDefinition.edges).toHaveLength(130);
  });

  it("keeps the 2v2 board as the default", () => {
    expect(boardForMatchMode("two_vs_two")).toBe(boardDefinition);
    expect(boardForMatchMode()).toBe(boardDefinition);
  });
});
