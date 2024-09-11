-- +goose Up
-- +goose StatementBegin
ALTER TABLE "photos"
-- Add new columns
ADD COLUMN "name" varchar NOT NULL,
ADD COLUMN "width" integer NOT NULL,
ADD COLUMN "height" integer NOT NULL,
ADD COLUMN "created_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
ADD COLUMN "updated_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Drop old columns
ALTER TABLE "photos"
DROP COLUMN "original_path",
DROP COLUMN "optimized_path",
DROP COLUMN "thumbnail_path",
DROP COLUMN "createdat",
DROP COLUMN "updatedat";

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
SELECT
  'down SQL query';

-- +goose StatementEnd
