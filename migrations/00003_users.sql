-- +goose Up
-- +goose StatementBegin
CREATE TABLE "users" (
  "id" varchar NOT NULL,
  "given_name" varchar NOT NULL,
  "family_name" varchar NOT NULL,
  "email" varchar NOT NULL,
  "profile_picture_url" varchar,
  "created_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "users_email_unique" UNIQUE ("email")
);

CREATE TABLE "github_connections" (
  "github_id" int NOT NULL,
  "user_id" varchar NOT NULL,
  "github_access_token" varchar NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("github_id"),
  CONSTRAINT "github_connection_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "users" ("id")
);

CREATE TABLE "user_sessions" (
  "id" varchar NOT NULL,
  "user_id" varchar NOT NULL,
  "refresh_token_hash" varchar NOT NULL,
  "expires_at" timestamp NOT NULL,
  "invalidated_at" timestamp,
  -- Family ID is used to group sessions together. When a user refreshes their session,
  -- all other sessions with the same family ID are invalidated.
  "family_id" varchar NOT NULL,
  "created_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT "user_sessions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "users" ("id"),
  PRIMARY KEY ("id")
);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
DROP TABLE "user_sessions";

DROP TABLE "github_connections";

DROP TABLE "users";

-- +goose StatementEnd
