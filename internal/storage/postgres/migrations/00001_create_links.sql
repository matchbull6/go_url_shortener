-- +goose Up
CREATE TABLE links (
    code       text        PRIMARY KEY,
    url        text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE links;
