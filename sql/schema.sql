BEGIN;
CREATE TABLE IF NOT EXISTS users (
       id SERIAL NOT NULL,
       name VARCHAR(128) NOT NULL,
       CONSTRAINT users_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS assets (
       id SERIAL NOT NULL,
       name VARCHAR(128) NOT NULL,
       uuid VARCHAR(128) NOT NULL UNIQUE,
       CONSTRAINT assets_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS asset_watches (
       id serial NOT NULL,
       user_id INT REFERENCES assets ON DELETE CASCADE,
       asset_id VARCHAR(128), -- REFERENCES assets ON DELETE CASCADE,
       threshold NUMERIC,
       CONSTRAINT asset_watches_pkey PRIMARY KEY (id),
       CONSTRAINT uniq_user_asset_threshold UNIQUE(user_id, asset_id, threshold)
);

INSERT INTO users(id, name) VALUES
       (1, 'User 1'),
       (2, 'User 2'),
       (3, 'User 3');

INSERT INTO assets(id, name, uuid) VALUES
       (1, 'Asset 1', '123e4567-e89b-12d3-a456-426655440001'),
       (2, 'Asset 2', '223e4567-e89b-12d3-a456-426655440002'),
       (3, 'Asset 3', '323e4567-e89b-12d3-a456-426655440003');

COMMIT;
