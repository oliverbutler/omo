-- +goose Up
-- +goose StatementBegin
ALTER TABLE photos
ADD COLUMN lens VARCHAR(255),
ADD COLUMN aperature VARCHAR(50),
ADD COLUMN shutter_speed VARCHAR(50),
ADD COLUMN iso VARCHAR(50),
ADD COLUMN focal_length VARCHAR(50),
ADD COLUMN focal_length_35mm VARCHAR(50);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
