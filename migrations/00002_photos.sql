-- +goose Up
-- +goose StatementBegin
CREATE TABLE photos (
  id VARCHAR PRIMARY KEY,
  original_path VARCHAR NOT NULL,
  optimized_Path VARCHAR,
  thumbnail_path VARCHAR,
  blur_hash VARCHAR,
  createdAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE photos;

-- +goose StatementEnd
