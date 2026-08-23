# 四国军棋规则

This document defines the standard four-player, two-versus-two form of
Chinese Army Chess (四国军棋) targeted by this project. It is the game-rule
specification, not a description of a particular UI or hosting platform.

The game has three common visibility modes—four-dark, double-visible, and
fully visible—but the board, movement, combat, and victory rules below are the
same in all three modes.

## 1. Objective and teams

There are four seats around the board: north, east, south, and west.

- North and south are teammates.
- East and west are teammates.
- Each seat controls one country and one set of 25 pieces.
- Turns proceed clockwise, one seat at a time. The room chooses the opening
  seat; after that, play skips eliminated seats.

The objective is to eliminate both countries on the opposing team. A country
is eliminated when its flag is captured or when it has no legal move. Capturing
one country’s flag does not end the match by itself: the surviving teammate on
that team may continue playing.

## 2. Board

See the dedicated [board layout reference](board-layout.md) for the complete
top-down diagram, local coordinates, nine-palace graph, and road/rail edge
inventory.

The board has:

- 30 stopping points in each country’s area;
- 9 stopping points in the middle, called the **nine-palace** (九宫);
- 5 camps (行营) in each country’s area;
- 2 headquarters (大本营) in each country’s area; and
- ordinary stations (兵站) everywhere else.

Each country places its 25 pieces on the 25 non-camp stopping points. The five
camps are not deployment positions and are empty at the start of the game.

### 2.1 Local country layout

For every country, rows are numbered from the front (row 1) toward the rear
(row 6). The five positions in each row are ordered as

`1-left, 2-left, 3-center, 2-right, 1-right`.

The local layout is:

```text
             front / opponent

row 1       [station] = [station] = [station] = [station] = [station]
row 2       [station] - [camp]    - [station] - [camp]    - [station]
row 3       [station] - [station] - [camp]    - [station] - [station]
row 4       [station] - [camp]    - [station] - [camp]    - [station]
row 5       [station] = [station] = [station] = [station] = [station]
row 6       [station] - [HQ]      - [station] - [HQ]      - [station]

             rear / own side
```

`=` marks railway edges; `-` marks ordinary road edges in the local layout.
The board also has the following railway connections:

- the outer left and right tracks from row 1 through row 5;
- the complete horizontal railway across rows 1 and 5;
- the railway connections from the four countries into the nine-palace;
- the railway connections between neighboring countries at the outer top
  stations; and
- the railway edges inside the nine-palace.

The diagonal connections around camps are ordinary roads. The board topology,
including the exact nine-palace connections, is fixed; pieces may not invent
connections merely because two positions look close together.

### 2.2 Camps and headquarters

An occupied camp is a safe position:

- an enemy piece may move into an empty camp;
- an enemy piece may not attack a piece occupying a camp;
- a friendly or allied piece cannot be attacked; and
- an occupied camp blocks a route just like any other occupied position.

Headquarters are ordinary stopping points for combat, but have one permanent
movement restriction: once a piece is in a headquarters, it cannot move out.
The two headquarters are also the only legal locations for a flag during
deployment.

## 3. Pieces and inventory

Each country has exactly 25 pieces:

| Piece | Chinese name | Count | Rank / special rule |
| --- | --- | ---: | --- |
| Flag | 军旗 | 1 | Objective; may only be placed in HQ; cannot move |
| Commander | 司令 | 1 | Highest rank; its loss reveals the flag |
| Marshal | 军长 | 1 | Rank 8 |
| Division commander | 师长 | 2 | Rank 7 |
| Brigade commander | 旅长 | 2 | Rank 6 |
| Regiment commander | 团长 | 2 | Rank 5 |
| Battalion commander | 营长 | 2 | Rank 4 |
| Bomb | 炸弹 | 2 | Special combat piece; cannot be placed in row 1 |
| Company commander | 连长 | 3 | Rank 3 |
| Platoon commander | 排长 | 3 | Rank 2 |
| Engineer | 工兵 | 3 | Rank 1; can clear mines and turn on railways |
| Mine | 地雷 | 3 | Special defensive piece; only in rear two rows; cannot move |

The normal rank order, from strongest to weakest, is:

`Commander > Marshal > Division > Brigade > Regiment > Battalion > Company > Platoon > Engineer`

Bombs, mines, flags, camps, and headquarters are not part of that ordinary
rank comparison. Their special rules take precedence.

## 4. Deployment

Before the first turn, each player privately arranges their own 25 pieces in
their country’s 30 positions.

The deployment must satisfy all of these conditions:

1. Every piece belongs to the country whose area contains it.
2. All 25 pieces appear exactly once, with the inventory above.
3. Camps are empty.
4. Exactly one flag is placed in one of the two headquarters.
5. Mines are placed only in the rear two rows (rows 5 or 6).
6. Bombs are not placed in the front row (row 1).
7. Other pieces may use any remaining non-camp station or headquarters,
   subject to the restrictions above.

The headquarters containing the flag is the real headquarters. The other is
often called the **false headquarters**, but it has the same movement lock as
the real one.

Deployment is complete only when every player has submitted a valid position.
The normal four-dark and double-visible modes keep enemy deployments hidden
after submission.

## 5. Turn procedure

On a player’s turn:

1. The player selects one of their surviving, movable pieces.
2. The player selects a legal destination reachable by that piece.
3. If the destination is empty, the piece moves there.
4. If the destination contains an enemy piece and is not a camp, combat is
   resolved immediately.
5. The turn passes clockwise to the next non-eliminated seat.

A player may not move a teammate’s piece, move an eliminated country’s piece,
attack an ally, or pass over an occupied position.

If a player has no legal move when their turn is reached, that country is
eliminated. Its remaining pieces are removed and its turns are skipped.

## 6. Movement

Movement uses the fixed road and railway graph on the board.

### 6.1 Ordinary road movement

An ordinary road move traverses exactly one adjacent road edge. A piece may
not continue along a second road edge in the same move.

### 6.2 Railway movement

On a railway, a piece may move any number of consecutive railway edges while:

- every intermediate stopping point is empty;
- the destination is empty or contains an attackable enemy piece; and
- the route obeys the turning restriction for that piece.

Non-engineer pieces may travel a straight railway route only. They may not
turn through a railway corner. The route may not switch from railway to road
to gain extra distance.

An engineer may travel through any connected, unobstructed railway route and
may turn at railway corners. It still cannot pass through an occupied
position, and it does not get to travel multiple road edges.

### 6.3 Pieces that cannot move

- A flag can never move.
- A mine can never move.
- No piece can leave a headquarters after entering it.
- A piece may not move into an occupied camp or capture a piece in a camp.

The standard rules in this document do not use the optional “one railway turn
for every piece” variant. That variant must not be enabled implicitly.

## 7. Combat

Combat occurs when an attacking piece moves onto an enemy piece in an
attackable station or headquarters. The attacker and defender are then
removed or retained according to the following table.

| Situation | Result |
| --- | --- |
| Attacker has a higher ordinary rank | Defender is removed; attacker occupies the destination |
| Attacker has a lower ordinary rank | Attacker is removed; defender remains |
| Equal ordinary ranks | Both pieces are removed |
| Engineer attacks a mine | Mine is removed; engineer occupies the destination |
| Any non-engineer, non-bomb piece attacks a mine | Attacker is removed; mine remains |
| A bomb attacks any piece | Both pieces are removed |
| Any piece attacks a bomb | Both pieces are removed |

The bomb rule includes bomb-versus-bomb and bomb-versus-mine. A bomb may also
attack a flag; both pieces are removed, the flag is considered captured, and
the flag’s country is eliminated.

A flag does not participate in ordinary rank comparison. Any successful
attack on it captures it. A move into an empty enemy headquarters does not
capture a flag unless the flag is actually there.

### 7.1 Commander and flag reveal

When a country’s commander is removed, that country must reveal the location
of its surviving flag to all players. This applies whether the commander was
attacking, defending, or removed together with another piece. The flag stays
in place and remains immobile until captured.

If the flag was already captured, there is nothing left to reveal.

## 8. Victory and draws

### 8.1 Team victory

A country is eliminated by either:

- capture of its flag; or
- having no legal move when its turn begins.

When both countries on one team are eliminated, the other team wins. A single
country being eliminated does not end the match.

### 8.2 Draw

The standard rule permits a draw when both teams agree that neither can force
a win. The application may also provide an explicit draw offer and acceptance
workflow, but a unilateral draw offer does not end a match.

The 70-consecutive-non-capture move rule used by some online rooms is a house
rule, not a universal board-game rule. If enabled for a room, it should be
displayed as part of that room’s rules before the match starts.

## 9. Visibility modes

Visibility controls which ranks a player may see; it does not change the legal
moves or combat results.

### Four-dark (四暗)

Each player sees the ranks of their own surviving pieces. The ranks of both
enemy countries and the teammate’s country remain hidden, except for flags
that have been revealed after a commander is lost.

### Double-visible (双明)

Each player sees the ranks of their own country and their teammate’s country.
The two enemy countries remain hidden, except for revealed flags.

### Fully visible (全明)

All surviving pieces are visible to all active players from the start of the
match.

Spectators do not receive hidden rank information during a live match. A
completed replay may expose the full position, including ranks that were
hidden during play.

## 10. Digital room policy

The following are application policies rather than traditional board rules:

- the server is authoritative for the board, turn, combat, and clock;
- a room starts only after all four seats submit valid deployments and ready;
- the room may choose a turn clock and a deployment clock;
- a missed turn may skip the player, and repeated missed deadlines may
  eliminate the player if the room enables that policy;
- resignation eliminates the resigning country; and
- reconnection does not reveal ranks that the reconnecting player was not
  allowed to see before disconnecting.

The selected clock, timeout, resignation, and draw policy should be visible in
the room before players ready up. They must not be confused with the core
movement and combat rules above.

## 11. Terms used in the game

These common terms are useful in communication and replay notation:

- **对家**: the teammate seated opposite you.
- **上家 / 下家**: the previous / next seat in turn order.
- **九宫**: the nine central railway stations.
- **行营**: a camp; an occupied camp cannot be attacked.
- **大本营**: a headquarters; a piece entering one cannot leave.
- **真旗 / 假旗**: the real flag headquarters / the other headquarters.
- **亮旗**: revealing the flag after the commander is lost.
- **空炸**: using a bomb without first confirming the target’s rank.
- **控盘**: occupying or controlling the nine-palace to restrict the enemy.
- **闪电**: a rapid railway-led offensive intended to capture a flag quickly.

## 12. Reference sources and implementation notes

The standard rules in this document were consolidated from:

- [四国军棋 on Chinese Wikipedia](https://zh.wikipedia.org/zh-hans/%E5%9B%9B%E5%9B%BD%E5%86%9B%E6%A3%8B), especially its board, deployment, movement, combat, and victory sections;
- [联众游戏的四国军棋介绍](https://www.ourgame.com/game/game-intro/junqi_ocx.html), using only its game rules and excluding platform-specific rooms, downloads, ranking, and UI behavior; and
- the board and movement implementation in [910JQK/GwanKei](https://github.com/910JQK/GwanKei), particularly [`core.cpp`](https://github.com/910JQK/GwanKei/blob/master/core.cpp), [`game.cpp`](https://github.com/910JQK/GwanKei/blob/master/game.cpp), and [`game.hpp`](https://github.com/910JQK/GwanKei/blob/master/game.hpp).

The GwanKei source is used as a topology and movement reference. Where its
compact rank-number comparison differs from the standard bomb rule, this
document follows the explicit standard rule: a bomb removes itself and any
piece it attacks.

## 13. Executable board contract

The executable implementation uses the topology in
[`contracts/board.v2.json`](../contracts/board.v2.json). The older `board.v1`
12×12 orthogonal grid is retained only as a historical contract and is not a
playable ruleset.
