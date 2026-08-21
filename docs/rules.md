# 四国军棋 v1 rules

This implementation fixes the common JJ-style rules for a four-player,
two-versus-two match. North and South are teammates; East and West are
teammates. The server is authoritative for every rule and clock.

## Setup

Each seat receives 25 pieces: one flag, one commander, one marshal, two each
of division, brigade, regiment, battalion, and bomb, and three each of
company, platoon, engineer, and mine. The flag occupies one headquarters;
mines occupy the final two rows; bombs do not occupy the front row. Camps are
empty at the start. A setup is not accepted until all inventory and placement
constraints pass validation.

## Movement

Road movement is one adjacent edge. A clear railway route may be traversed in
a straight line by ordinary pieces. Engineers may follow any connected clear
railway route, including turns. Flags and mines never move, and a piece that
starts in headquarters cannot move. A piece in a camp cannot be attacked.
Friendly pieces block routes and cannot be attacked.

The executable v1 board is a 12-by-12 orthogonal topology. The four 5-by-5
corner zones are the deployment areas, with the first and last sorted nodes in
each zone designated as headquarters. The shared middle contains four camps.
The stable contract is [board.v1.json](../contracts/board.v1.json).

## Combat

Rank order is commander > marshal > division > brigade > regiment > battalion
> company > platoon > engineer. The higher rank survives, equal ranks remove
both, engineers remove mines, other attackers are removed by mines, and bombs
remove both combatants. Capturing a flag eliminates that player and reveals its
location. The server sends only the rank information allowed by the room mode.

## Victory, clocks, and draws

A player is eliminated by flag capture, resignation, no legal move at the start
of a turn, or five cumulative missed deadlines. Remaining pieces of an
eliminated player are removed and their turns are skipped. A team wins when the
other team has no active player. Fast, standard, and relaxed clocks are 60/20,
120/60, and 300/120 seconds for setup/turn. A move timeout skips the turn and
increments that player's miss count. A unanimous draw offer ends the match;
rejection or a move cancels it. Seventy consecutive non-removal moves also
produce a draw.

## Visibility

`four_dark` exposes ranks to the owning player, `double_visible` also exposes
the teammate's ranks, and `fully_visible` exposes all ranks to active players.
Live spectators always receive the public hidden projection. Completed replay
views may disclose full truth.
