-- +goose Up
CREATE TABLE environments (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE environments;
