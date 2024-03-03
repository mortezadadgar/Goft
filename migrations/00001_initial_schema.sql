-- +goose Up
-- +goose StatementBegin

CREATE TABLE users(
	id                      int       GENERATED ALWAYS AS IDENTITY,
	name                    text      NOT NULL UNIQUE,
	hashed_password         bytea     NOT NULL,

	PRIMARY KEY(id)
);
CREATE UNIQUE INDEX users_name_idx ON users (name);
CREATE INDEX users_id_idx ON users (name);

CREATE TABLE rooms(
	id             int       GENERATED ALWAYS AS IDENTITY,
	name           text      NOT NULL UNIQUE,
	description    text      NOT NULL,

	PRIMARY KEY(id)
);
CREATE INDEX rooms_idx ON rooms USING GIN (to_tsvector('english', name));

CREATE TABLE sessions(
	uuid    uuid           NOT NULL,
	user_id int            NOT NULL,
	expiry  timestamp      NOT NULL,

	FOREIGN KEY(user_id)   REFERENCES users(id) ON DELETE CASCADE,
	PRIMARY KEY(uuid)
);

CREATE TABLE messages(
	id            int       GENERATED ALWAYS AS IDENTITY,
	text          text      NOT NULL,
	user_id       int       NOT NULL,
	room_id       int       NOT NULL,

	FOREIGN KEY(user_id)        REFERENCES users(id) ON DELETE CASCADE,
	FOREIGN KEY(room_id)        REFERENCES rooms(id) ON DELETE CASCADE,
	PRIMARY KEY(id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE rooms;
DROP TABLE sessions;
DROP TABLE messages;
DROP TABLE users;

-- +goose StatementEnd
