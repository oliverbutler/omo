-- +goose Up
-- +goose StatementBegin
CREATE TABLE visits (page VARCHAR(255), visits INT);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE visits;

-- +goose StatementEnd
