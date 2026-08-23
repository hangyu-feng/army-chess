CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash BYTEA NOT NULL UNIQUE,
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS saved_layouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    ruleset_version TEXT NOT NULL DEFAULT 'rules.v1',
    board_version TEXT NOT NULL DEFAULT 'board.v2',
    deployment JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(profile_id, name)
);

CREATE TABLE IF NOT EXISTS rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invite_code TEXT NOT NULL UNIQUE,
    phase TEXT NOT NULL,
    host_profile_id UUID REFERENCES profiles(id),
    visibility_mode TEXT NOT NULL,
    clock_preset TEXT NOT NULL,
    spectator_cap INTEGER NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS room_participants (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    seat TEXT,
    connected_at TIMESTAMPTZ,
    ready BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY(room_id, profile_id)
);

CREATE TABLE IF NOT EXISTS matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    ruleset_version TEXT NOT NULL DEFAULT 'rules.v1',
    board_version TEXT NOT NULL DEFAULT 'board.v2',
    visibility_mode TEXT NOT NULL,
    clock_preset TEXT NOT NULL,
    opening_seat TEXT NOT NULL,
    outcome TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS match_seats (
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    seat TEXT NOT NULL,
    profile_id UUID REFERENCES profiles(id),
    team TEXT NOT NULL,
    eliminated BOOLEAN NOT NULL DEFAULT false,
    elimination_reason TEXT,
    PRIMARY KEY(match_id, seat)
);

CREATE TABLE IF NOT EXISTS match_events (
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(match_id, sequence)
);

CREATE TABLE IF NOT EXISTS match_snapshots (
    match_id UUID NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL,
    state JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(match_id, sequence)
);
