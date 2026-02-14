CREATE TABLE agencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    url TEXT,
    timezone TEXT DEFAULT 'Europe/Madrid',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stop_id TEXT NOT NULL,
    agency_id UUID REFERENCES agencies(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    name_vector tsvector GENERATED ALWAYS AS (to_tsvector('spanish', name)) STORED,
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    platform_code TEXT,
    wheelchair_accessible BOOLEAN DEFAULT false,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(agency_id, stop_id)
);

CREATE INDEX idx_stops_location ON stops USING GIST(location);
CREATE INDEX idx_stops_name_search ON stops USING GIN(name_vector);
CREATE INDEX idx_stops_name_trgm ON stops USING GIN(name gin_trgm_ops);

CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id TEXT NOT NULL,
    agency_id UUID REFERENCES agencies(id) ON DELETE CASCADE,
    short_name TEXT,
    long_name TEXT NOT NULL,
    route_type INT NOT NULL,
    color TEXT DEFAULT '000000',
    text_color TEXT DEFAULT 'FFFFFF',
    shape GEOGRAPHY(LINESTRING, 4326),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(agency_id, route_id)
);

CREATE INDEX idx_routes_shape ON routes USING GIST(shape);

CREATE TABLE trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id TEXT NOT NULL,
    route_id UUID REFERENCES routes(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL,
    headsign TEXT,
    direction_id INT,
    shape_id TEXT,
    wheelchair_accessible BOOLEAN DEFAULT false,
    bikes_allowed BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(route_id, trip_id)
);

CREATE TABLE stop_times (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id UUID REFERENCES trips(id) ON DELETE CASCADE,
    stop_id UUID REFERENCES stops(id) ON DELETE CASCADE,
    arrival_time INTERVAL NOT NULL,
    departure_time INTERVAL NOT NULL,
    stop_sequence INT NOT NULL,
    pickup_type INT DEFAULT 0,
    drop_off_type INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_stop_times_trip ON stop_times(trip_id, stop_sequence);
CREATE INDEX idx_stop_times_stop ON stop_times(stop_id);

CREATE TABLE vehicle_positions (
    time TIMESTAMPTZ NOT NULL,
    vehicle_id TEXT NOT NULL,
    trip_id UUID REFERENCES trips(id),
    route_id UUID REFERENCES routes(id),
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    bearing FLOAT,
    speed FLOAT,
    congestion_level INT,
    occupancy_status INT,
    metadata JSONB DEFAULT '{}'
);

SELECT create_hypertable('vehicle_positions', 'time');

SELECT add_retention_policy('vehicle_positions', INTERVAL '7 days');

CREATE MATERIALIZED VIEW vehicle_positions_1min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    vehicle_id,
    route_id,
    AVG(speed) as avg_speed,
    COUNT(*) as position_count
FROM vehicle_positions
GROUP BY bucket, vehicle_id, route_id;

CREATE TABLE delay_events (
    id UUID DEFAULT gen_random_uuid(),
    time TIMESTAMPTZ NOT NULL,
    trip_id UUID REFERENCES trips(id),
    stop_id UUID REFERENCES stops(id),
    scheduled_arrival TIMESTAMPTZ NOT NULL,
    actual_arrival TIMESTAMPTZ NOT NULL,
    delay_seconds INT NOT NULL,
    is_compensated BOOLEAN DEFAULT false,
    compensation_sent_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    PRIMARY KEY (id, time)
);

SELECT create_hypertable('delay_events', 'time');

CREATE TABLE affiliates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    address TEXT,
    offer_text TEXT NOT NULL,
    offer_value DECIMAL(10,2),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_affiliates_location ON affiliates USING GIST(location);
CREATE INDEX idx_affiliates_active ON affiliates(active) WHERE active = true;

CREATE TABLE compensations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    delay_event_id UUID, -- references delay_events(id) logically; FK not supported on hypertable
    affiliate_id UUID REFERENCES affiliates(id),
    code TEXT UNIQUE NOT NULL,
    issued_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    redeemed_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_compensations_user ON compensations(user_id);
CREATE INDEX idx_compensations_code ON compensations(code);
