-- New saved layouts and matches use the executable classic board. Existing
-- records retain their original version so historical replays are not
-- silently reinterpreted.
ALTER TABLE saved_layouts ALTER COLUMN board_version SET DEFAULT 'board.v2';
ALTER TABLE matches ALTER COLUMN board_version SET DEFAULT 'board.v2';
