package db

import (
	"context"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE TABLE IF NOT EXISTS journal_entries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    local_date DATE NOT NULL DEFAULT CURRENT_DATE,
    topics TEXT NOT NULL,
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, local_date)
);
`
	_, err := db.ExecContext(context.Background(), schema)
	if err != nil {
		return err
	}

	alters := `
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='first_name'
    ) THEN
        ALTER TABLE users ADD COLUMN first_name TEXT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='last_name'
    ) THEN
        ALTER TABLE users ADD COLUMN last_name TEXT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='avatar_id'
    ) THEN
        ALTER TABLE users ADD COLUMN avatar_id INTEGER;
    END IF;
    -- Ensure default for avatar_id is 1 and backfill existing NULLs
    ALTER TABLE users ALTER COLUMN avatar_id SET DEFAULT 1;
    UPDATE users SET avatar_id = 1 WHERE avatar_id IS NULL;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='goal'
    ) THEN
        ALTER TABLE users ADD COLUMN goal TEXT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='start_date'
    ) THEN
        ALTER TABLE users ADD COLUMN start_date DATE;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='end_date'
    ) THEN
        ALTER TABLE users ADD COLUMN end_date DATE;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='is_admin'
    ) THEN
        ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;
    END IF;
END $$;`
	_, err = db.ExecContext(context.Background(), alters)
	return err
}
