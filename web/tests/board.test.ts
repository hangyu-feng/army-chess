import { describe, expect, it } from "vitest";
import { boardDefinition } from "../src/board";

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
    expect(boardDefinition.nodes["north-r1-1L"]).toMatchObject({ x: 590, y: 340, deployFor: "north" });
    expect(boardDefinition.nodes["east-r1-1L"]).toMatchObject({ x: 660, y: 590, deployFor: "east" });
    expect(boardDefinition.nodes["south-r1-1L"]).toMatchObject({ x: 410, y: 660, deployFor: "south" });
    expect(boardDefinition.nodes["west-r1-1L"]).toMatchObject({ x: 340, y: 410, deployFor: "west" });
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
