package db

import (
	"context"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    email_blind_index TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE TABLE IF NOT EXISTS journal_entries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    local_date DATE NOT NULL DEFAULT CURRENT_DATE,
    topics TEXT NOT NULL,
    alignment_rating INTEGER NOT NULL CHECK (alignment_rating BETWEEN 1 AND 10),
    contentment_rating INTEGER NOT NULL CHECK (contentment_rating BETWEEN 1 AND 10),
    karma REAL NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, local_date)
);

CREATE TABLE IF NOT EXISTS goals (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    goal TEXT NOT NULL,
    start_date DATE,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);
`
	_, err := db.ExecContext(context.Background(), schema)
	if err != nil {
		return err
	}

	alters := `
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='journal_entries' AND column_name='rating'
    ) THEN
        ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS alignment_rating INTEGER NOT NULL DEFAULT 1;
        ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS contentment_rating INTEGER NOT NULL DEFAULT 1;

        UPDATE journal_entries SET alignment_rating = rating;
        UPDATE journal_entries SET contentment_rating = rating;

        ALTER TABLE journal_entries DROP COLUMN rating;

        ALTER TABLE journal_entries ALTER COLUMN alignment_rating DROP DEFAULT;
        ALTER TABLE journal_entries ALTER COLUMN contentment_rating DROP DEFAULT;

        ALTER TABLE journal_entries ADD CONSTRAINT alignment_rating_check CHECK (alignment_rating BETWEEN 1 AND 10);
        ALTER TABLE journal_entries ADD CONSTRAINT contentment_rating_check CHECK (contentment_rating BETWEEN 1 AND 10);
    END IF;
END $$;
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
    -- Add email_blind_index column for existing users table
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='email_blind_index'
    ) THEN
        ALTER TABLE users ADD COLUMN email_blind_index TEXT;
        -- Note: After adding this column, you'll need to run a data migration to populate blind indexes
    END IF;
END $$;
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='journal_entries' AND column_name='karma'
    ) THEN
        ALTER TABLE journal_entries ADD COLUMN karma REAL NOT NULL DEFAULT 0;

        -- Backfill karma for existing entries
        UPDATE journal_entries
        SET karma = (alignment_rating + contentment_rating - 2) / 18.0;
    END IF;
END $$;
-- Migrate existing goal data from users to goals table
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='goal'
    ) THEN
        -- Insert existing goals into goals table (only for users with a goal set)
        INSERT INTO goals (user_id, goal, start_date, end_date)
        SELECT id, goal, start_date, end_date
        FROM users
        WHERE goal IS NOT NULL
        ON CONFLICT (user_id) DO NOTHING;

        -- Drop the goal-related columns from users table
        ALTER TABLE users DROP COLUMN IF EXISTS goal;
        ALTER TABLE users DROP COLUMN IF EXISTS start_date;
        ALTER TABLE users DROP COLUMN IF EXISTS end_date;
    END IF;
END $$;`
	_, err = db.ExecContext(context.Background(), alters)
	return err
}
