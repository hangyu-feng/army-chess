# 1v1 陆战棋规则

This document defines the project’s canonical two-player form of Army Chess,
also called **陆战棋**, **陆军棋**, **军棋**, or **Luzhanqi**. It is a complete
rules and board specification for a two-player match, including the board
topology, piece inventory, deployment, movement, combat, information model,
and victory conditions.

The canonical 1v1 board in this document is the traditional **5 × 13 board**:
two six-row country zones separated by a five-space central band containing
three frontlines and two mountains. This is the board shown in the supplied
references. A simplified 12 × 5 board used by one sample implementation is
documented as an alternate at the end; it is not the canonical layout here.

## 1. Game summary

- There are two opposing players: top and bottom.
- Each player controls one country and 25 pieces.
- Each player arranges their pieces privately before the first turn.
- Players take exactly one move per turn.
- The referee or server resolves hidden combat and announces only its result.
- The primary objective is to capture the opponent’s flag.
- A player also loses when they have no legal move at the beginning of their
  turn.

The normal hidden-information form is called **暗棋**. The open-information
form is called **明棋**. A face-down flipping form, **翻棋**, is a separate
variant with additional randomness and is not the canonical room ruleset.

## 2. Canonical board

### 2.1 Overall shape

The board has 65 marked spaces:

- 30 spaces in the top country’s six-row zone;
- 30 spaces in the bottom country’s six-row zone; and
- five central-band spaces: three **frontlines** (前线) and two **mountains**
  (山界).

The mountains are printed board spaces but are not occupiable in the canonical
rules. Therefore, 63 spaces can contain a piece during play: 60 country-zone
positions and three frontlines.

```text
                    TOP COUNTRY
                 rear / headquarters

T6       [station] [ HQ ] [station] [ HQ ] [station]
T5       [station] [station] [station] [station] [station]
T4       [station] [camp] [station] [camp]    [station]
T3       [station] [station] [camp]    [station] [station]
T2       [station] [camp] [station] [camp]    [station]
T1       [station] [station] [station] [station] [station]
                         front

CENTRAL  [frontline] [mountain] [frontline] [mountain] [frontline]

                         front
B1       [station] [station] [station] [station] [station]
B2       [station] [camp] [station] [camp]    [station]
B3       [station] [station] [camp]    [station] [station]
B4       [station] [camp] [station] [camp]    [station]
B5       [station] [station] [station] [station] [station]
B6       [station] [ HQ ] [station] [ HQ ] [station]

                 rear / headquarters
                    BOTTOM COUNTRY
```

`T1` and `B1` face the central band. `T6` and `B6` are the rear rows. The
columns are numbered left to right as `1L`, `2L`, `3`, `2R`, `1R`; the names
describe the local formation rather than a player’s physical left/right after
the board is rotated.

### 2.2 Country-zone positions

Each six-row zone contains exactly:

- 23 ordinary stations (兵站);
- five camps (行营); and
- two headquarters (大本营).

The five camps are at local coordinates `r2-2L`, `r2-2R`, `r3-3`, `r4-2L`,
and `r4-2R`. The two headquarters are `r6-2L` and `r6-2R`. Camps are safe
positions but are not deployment positions. Headquarters are deployment
positions and are the only places where a flag may start.

### 2.3 Central band

The three frontlines are the only cross-country stopping points. They connect
the corresponding front stations in the two countries:

```text
top T1-1L ── frontline-1 ── bottom B1-1L
top T1-3  ── frontline-3  ── bottom B1-3
top T1-1R ── frontline-5 ── bottom B1-1R
```

These are railway routes. The two mountains are not legal destinations and do
not block a railway route because no railway runs through them. There are no
connections between the three frontlines, and no piece may enter or cross a
mountain under the canonical rules.

### 2.4 Roads and railways

The board graph is explicit. Screen distance or rectangular adjacency must not
be used to infer a legal move.

Within each country zone:

- railway edges run horizontally across local rows 1 and 5;
- railway edges run vertically through the two outer columns, from row 1
  through row 5;
- road edges run horizontally across local rows 2, 3, 4, and 6;
- road edges run vertically through the three inner columns from row 2 through
  row 6;
- road edges connect the two outer columns from row 5 to row 6; and
- each camp has diagonal road edges to its four surrounding stations.

Across the central band, the three frontline routes continue the three
railways from top row 1 to bottom row 1. A railway route may not turn through
a mountain or jump from one frontline to another.

### 2.5 Special positions

#### Camps (行营)

- A piece may enter an empty camp.
- A piece may leave a camp on a later turn.
- An enemy may not attack a piece occupying a camp.
- A camp does not become a deployment position and cannot hold a piece during
  initial setup.
- An occupied camp blocks railway and road movement like any other occupied
  space.

#### Headquarters (大本营)

- A flag must be deployed in one of the two headquarters.
- Any other piece may also be deployed in a headquarters.
- A piece that enters a headquarters can never leave it.
- An enemy may enter a headquarters and attack the piece there.
- Entering the false headquarters does not win; if the attacking piece survives
  the combat, it remains permanently stuck there.
- Entering the headquarters containing the flag captures the flag and ends the
  game immediately.

#### Mountains (山界)

- Mountains cannot be entered, occupied, attacked, or crossed.
- They are board landmarks separating the three railway frontlines.

Some regional rules allow a piece, sometimes only an engineer, to enter a
mountain as a protected position. That is not part of this project’s canonical
rules and must be treated as an explicit room variant if ever added.

## 3. Pieces and inventory

Each player has exactly 25 pieces:

| Piece | Chinese | Count | Ordinary rank / special rule |
| --- | --- | ---: | --- |
| Commander | 司令 | 1 | Highest ordinary rank; its loss reveals the flag |
| Marshal | 军长 | 1 | Rank 8 |
| Division commander | 师长 | 2 | Rank 7 |
| Brigade commander | 旅长 | 2 | Rank 6 |
| Regiment commander | 团长 | 2 | Rank 5 |
| Battalion commander | 营长 | 2 | Rank 4 |
| Company commander | 连长 | 3 | Rank 3 |
| Platoon commander | 排长 | 3 | Rank 2 |
| Engineer | 工兵 | 3 | Rank 1; clears mines and turns on railways |
| Bomb | 炸弹 | 2 | Special; both combat pieces are removed |
| Mine | 地雷 | 3 | Special; immobile defensive piece |
| Flag | 军旗 | 1 | Special objective; immobile and starts in HQ |

The ordinary rank order, strongest to weakest, is:

```text
司令 > 军长 > 师长 > 旅长 > 团长 > 营长 > 连长 > 排长 > 工兵
```

Bombs, mines, and flags are not part of the ordinary rank ladder. Their
special combat rules take precedence.

## 4. Setup and deployment

1. Each player receives one complete 25-piece inventory.
2. Each player places all pieces in their own six-row country zone.
3. Every non-camp position in that zone must contain exactly one piece.
4. All camps and the central band remain empty.
5. The flag must occupy one of the two headquarters.
6. Mines may occupy only rows 5 or 6, the rear two rows.
7. Bombs may not occupy row 1, the front row.
8. All other pieces may occupy any remaining station or headquarters.
9. The arrangement is submitted privately and cannot be changed after setup
   completes.

The project’s canonical room uses hidden setup: each player sees their own
piece identities, while the referee/server sees both inventories. A setup is
valid only when it contains the complete inventory exactly once and satisfies
all location restrictions.

The room chooses the opening player. For a physical game using this document,
the bottom player starts by convention unless both players agree otherwise.

## 5. Turn procedure

On a turn, the active player:

1. selects one surviving piece they own;
2. selects one legal destination;
3. moves to an empty destination, or attacks an enemy piece on an attackable
   station or headquarters; and
4. passes the turn to the other player.

A player may not:

- move the opponent’s piece;
- move a flag or mine;
- move a piece out of a headquarters;
- move through an occupied space;
- attack their own piece;
- attack an occupied camp; or
- enter a mountain.

If a player has no legal move when their turn begins, that player loses by
immobility and the opponent wins.

## 6. Movement

### 6.1 Road movement

A road move traverses exactly one connected road edge. A piece cannot chain
multiple road edges in one turn.

### 6.2 Railway movement

A movable piece starting on a railway may travel through any number of
consecutive railway edges in one move if:

- every intermediate space is empty;
- the destination is empty or contains an attackable enemy piece; and
- the route obeys the piece’s railway turning restriction.

Every ordinary piece other than an engineer must travel in a straight railway
line. It may not turn at a railway corner or change to a different railway
line. The project does not use the regional three-space railway limit.

An engineer may follow any connected, unobstructed railway route. It may turn
at corners, change direction at junctions, and use the three cross-country
frontline routes. It still cannot pass through an occupied space or chain a
road move into the same turn.

### 6.3 Immobile pieces

- A flag never moves.
- A mine never moves.
- No piece can leave a headquarters after entering it.
- A bomb is movable unless it is in a headquarters.

## 7. Combat

Combat occurs only when the attacker moves onto an enemy piece in an
attackable station or headquarters. The referee/server resolves the identities
and reports the result.

| Attack | Result |
| --- | --- |
| Higher ordinary rank attacks lower ordinary rank | Defender is removed; attacker occupies the destination |
| Lower ordinary rank attacks higher ordinary rank | Attacker is removed; defender remains |
| Equal ordinary ranks meet | Both pieces are removed |
| Engineer attacks a mine | Mine is removed; engineer survives on the destination |
| Any non-engineer, non-bomb piece attacks a mine | Attacker is removed; mine remains |
| Bomb attacks any enemy piece | Bomb and defender are both removed |
| Any enemy piece attacks a bomb | Attacker and bomb are both removed |
| Any piece attacks a flag | Flag is captured; the defender’s country is eliminated |

The bomb rule includes bomb-versus-bomb, bomb-versus-mine, and bomb-versus-flag.
When a bomb attacks a flag, both pieces are removed, but the flag capture still
ends the game.

A camp cannot be the target of combat while occupied. An empty camp may be
entered normally.

## 8. Hidden information and reveals

### 8.1 Referee information

The referee/server knows every piece identity. In a digital game it must not
leak the opponent’s rank merely because a combat was resolved. The opponent
should receive the combat outcome, such as “attacker survives”, “defender
survives”, or “both removed”, without the hidden identities.

### 8.2 Commander reveal

When a player’s Commander (司令) is removed, that player must reveal the
location of their surviving flag to both players. The flag remains in place
and remains immobile. If it was already captured, there is nothing to reveal.

This project uses the Commander-only reveal rule. Some regional rules wait
until both the Commander and Marshal have been removed; that is not the
canonical rule here.

### 8.3 Open and flipping variants

- **暗棋 / four-dark:** each player sees their own ranks; opponent ranks are
  hidden except for the revealed flag and completed-game replay information.
- **明棋:** both players arrange secretly, then all pieces are revealed before
  the first turn. Movement and combat are otherwise unchanged.
- **翻棋:** pieces are placed face down and are revealed as the game proceeds.
  This adds a random reveal mechanic and requires separate setup and turn
  rules; it is not the canonical 1v1 mode.

## 9. Victory, elimination, and draws

A player is eliminated when either condition occurs:

- their flag is captured; or
- they have no legal move when their turn begins.

The opponent wins immediately when the player is eliminated. A player may
also resign, which has the same result as elimination for match purposes.

The game may end in a draw only by mutual agreement or an explicitly enabled
room draw rule. A unilateral draw offer does not end the game. Automatic move
limits, repetition rules, and timeout behavior are room policies and must be
announced before the game starts.

## 10. Digital implementation requirements

The server implementation should model the board as an explicit graph rather
than a rectangular grid:

- use stable coordinates for `T1`–`T6`, `B1`–`B6`, and the five central-band
  positions;
- mark station, camp, headquarters, frontline, and mountain types explicitly;
- store road and railway edges separately;
- store railway direction/heading so ordinary pieces cannot turn while
  engineers can;
- validate the full 25-piece inventory and all setup restrictions on the
  server;
- keep hidden piece kinds out of opponent views;
- resolve combat atomically on the server; and
- record the combat result and any flag reveal in the match event stream.

The 1v1 mode has no teammates, no allied visibility, no nine-palace, and no
four-player curved railway links. Its turn sequence contains only the top and
bottom players.

## 11. Variant boundary and source comparison

There is no single universally enforced Luzhanqi rulebook. The following
choices are therefore explicit project decisions:

| Rule point | Canonical project choice | Common alternate |
| --- | --- | --- |
| Board | 5 × 13 with three frontlines and two impassable mountains | 12 × 5 board with directly connected front rows |
| Mine placement | Rows 5 and 6 only | Some rules allow mines in any row |
| Ordinary railway distance | Unlimited while unobstructed | Some rules cap non-engineers at three spaces |
| Mountains | Never enterable | Some rules allow entry, shelter, or engineer-only crossing |
| Commander loss | Immediately reveals the flag | Some rules wait for Commander and Marshal |
| Field Marshal and flag | Field Marshal may capture the flag | Some rules forbid it or declare a special draw |
| Mine attacked by ordinary piece | Attacker dies and mine remains | Some rules remove both pieces |

The two supplied board images depict the 5 × 13 form: six rows for each
country and a central band with three frontlines and two mountains. The
canonical rules follow that layout rather than silently mixing it with the
simplified 12 × 5 implementation layout.

## 12. References and implementation examples

The rules were consolidated from these sources:

- [陸軍棋 — Chinese Wikipedia](https://zh.wikipedia.org/zh-hans/%E9%99%B8%E8%BB%8D%E6%A3%8B): Chinese rules, board spaces, setup, combat variants, and dark/open/flipping forms.
- [Luzhanqi — English Wikipedia](https://en.wikipedia.org/wiki/Luzhanqi): English terminology, piece inventory, rank order, railway movement, and 1v1 variant rules.
- [online-junqi](https://github.com/samuelyuan/online-junqi): a browser implementation explicitly described as 1v1, with setup swapping, server-side board validation, direct front-line connectivity, a graph, and railway traversal.
- [luzhanqi-web](https://github.com/chinese-board-games/luzhanqi-web): a separate web implementation used as an engineering comparison for setup flow, client/server structure, and live play. Its public README is primarily a development/deployment guide rather than a complete rules specification.
- [Land Battle Chess reference implementation](https://chine.in/tools/land-battle-chess/): a cross-check for the 5 × 13 board, 65 marked spaces, three frontlines, two mountains, and the selected mine/railway/commander conventions.

The existing 2v2 rules document remains authoritative for the four-country
board. This file and [`contracts/board.1v1.json`](../contracts/board.1v1.json)
are authoritative for the implemented 1v1 board; the 2v2 graph is intentionally
not reused for this topology.
