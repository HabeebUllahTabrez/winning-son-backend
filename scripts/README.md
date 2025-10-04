# Database Encryption Migration

## Overview

This directory contains scripts to migrate existing plaintext data to encrypted format.

## Prerequisites

Install Python dependencies:

```bash
pip install psycopg2-binary python-dotenv cryptography
```

## Environment Setup

Create a `.env` file in the project root (or ensure these variables are set):

```bash
DATABASE_URL="postgresql://user:password@localhost:5432/dbname"
ENCRYPTION_KEY="your-32-byte-encryption-key-here"
BLIND_INDEX_KEY="your-32-byte-blind-index-key-12"
```

**Important**: The `ENCRYPTION_KEY` and `BLIND_INDEX_KEY` must be **exactly 32 bytes** and must match the keys used in your Go application.

### Generating Keys

You can generate secure 32-byte keys using:

```bash
# Using OpenSSL
openssl rand -base64 32 | cut -c1-32

# Using Python
python3 -c "import os; print(os.urandom(32).hex()[:32])"

# Using Go
go run -c "package main; import (\"crypto/rand\"; \"encoding/base64\"; \"fmt\"); func main() { b := make([]byte, 32); rand.Read(b); fmt.Println(string(b[:32])) }"
```

## Usage

### Dry Run (Recommended First)

Test the migration without making changes:

```bash
python scripts/migrate_to_encryption.py --dry-run
```

This will show you what changes would be made without actually modifying the database.

### Run Migration

Once you've verified the dry run output, run the actual migration:

```bash
python scripts/migrate_to_encryption.py
```

## What Gets Encrypted

The script encrypts the following fields:

### Users Table
- `email` - Encrypted with AES-256-GCM, blind index generated for searching
- `goal` - Encrypted with AES-256-GCM

### Journal Entries Table
- `topics` - Encrypted with AES-256-GCM

## Migration Process

1. **Users Migration**:
   - Fetches all users where `email_blind_index` is NULL
   - Encrypts email and generates HMAC-SHA256 blind index
   - Encrypts goal field if present
   - Updates database with encrypted values

2. **Journal Entries Migration**:
   - Fetches all journal entries
   - Encrypts topics field
   - Updates database with encrypted values

3. **Verification**:
   - Samples encrypted data from database
   - Attempts to decrypt and verify blind indexes
   - Reports any issues

## Safety Features

- **Dry run mode**: Test before applying changes
- **Duplicate detection**: Skips already-encrypted data
- **Transaction safety**: All changes in a transaction (rollback on error)
- **Error handling**: Reports errors but continues processing
- **Verification**: Automatically verifies encryption after migration

## Rollback

If something goes wrong, you can restore from your database backup. **Always backup your database before running this migration!**

```bash
# Backup before migration
pg_dump -U user -d dbname > backup_before_encryption.sql

# Restore if needed
psql -U user -d dbname < backup_before_encryption.sql
```

## Example Output

```
============================================================
Database Encryption Migration Script
============================================================

Encryption key: abcdefgh... (32 bytes)
Blind index key: 12345678... (32 bytes)
✓ Encryption service initialized
✓ Database connection established

=== Migrating Users ===
Found 150 users to migrate
  ✓ User 1 migrated (email: user1@example.com)
  ✓ User 2 migrated (email: user2@example.com)
  ...

✓ Successfully migrated 150 users

=== Migrating Journal Entries ===
Found 1250 journal entries to check
  ✓ Migrated 100 entries...
  ✓ Migrated 200 entries...
  ...

✓ Successfully migrated 1200 journal entries
  Skipped 50 entries (already encrypted or empty)

=== Verifying Encryption ===
  ✓ User 1: Email decrypts to user1@example.com
    ✓ Blind index matches
  ...

✓ Verification complete

============================================================
MIGRATION COMPLETE
============================================================
```

## Troubleshooting

### "ENCRYPTION_KEY must be exactly 32 bytes"
Your encryption key is not the correct length. Generate a new 32-byte key.

### "Error connecting to database"
Check your `DATABASE_URL` is correct and the database is accessible.

### "Blind index mismatch"
This indicates the encryption/blind index key may have changed. Verify your keys are correct.

### "Email appears already encrypted, skipping"
The script detected the data is already encrypted. This is safe to ignore.

## Post-Migration

After successful migration:

1. Start your Go application with the same encryption keys
2. Test login and data retrieval
3. Verify encrypted data appears correctly in the application
4. Keep your database backup for at least 1 week

## Security Notes

- **Never commit encryption keys to version control**
- Store keys securely (use a secrets manager in production)
- The same keys must be used in both the migration script and Go application
- Backup keys securely - if lost, encrypted data cannot be recovered
