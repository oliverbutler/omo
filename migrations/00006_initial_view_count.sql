-- +goose Up
-- +goose StatementBegin
INSERT INTO visits (page, visits) VALUES ('home', 0);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
