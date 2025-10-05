#!/usr/bin/env python3
"""
Migration script to encrypt existing data in the database.

This script:
1. Reads all existing users and journal entries from the database
2. Encrypts sensitive fields (email, goal, topics)
3. Generates blind indexes for email fields
4. Updates the database with encrypted data

Requirements:
    pip install psycopg2-binary python-dotenv cryptography

Usage:
    python scripts/migrate_to_encryption.py

Environment variables required:
    DATABASE_URL - PostgreSQL connection string
    ENCRYPTION_KEY - 32-byte encryption key (same as in Go app)
    BLIND_INDEX_KEY - 32-byte blind index key (same as in Go app)
"""

import os
import sys
import base64
import hmac
import hashlib
from typing import Optional, Tuple
from dotenv import load_dotenv
import psycopg2
from psycopg2.extras import RealDictCursor
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.backends import default_backend

# Load environment variables
load_dotenv()

class EncryptionService:
    """Python implementation of the Go encryption service"""

    def __init__(self, encryption_key: bytes, blind_index_key: bytes):
        if len(encryption_key) != 32:
            raise ValueError("Encryption key must be 32 bytes")
        if len(blind_index_key) != 32:
            raise ValueError("Blind index key must be 32 bytes")

        self.aesgcm = AESGCM(encryption_key)
        self.blind_index_key = blind_index_key

    def encrypt(self, plaintext: str) -> str:
        """Encrypt plaintext using AES-256-GCM"""
        if not plaintext:
            return ""

        # Generate a random 12-byte nonce
        nonce = os.urandom(12)

        # Encrypt the data
        ciphertext = self.aesgcm.encrypt(nonce, plaintext.encode('utf-8'), None)

        # Prepend nonce to ciphertext and base64 encode
        result = base64.b64encode(nonce + ciphertext).decode('utf-8')
        return result

    def decrypt(self, ciphertext: str) -> str:
        """Decrypt ciphertext using AES-256-GCM"""
        if not ciphertext:
            return ""

        # Base64 decode
        data = base64.b64decode(ciphertext)

        # Extract nonce and ciphertext
        nonce = data[:12]
        encrypted = data[12:]

        # Decrypt
        plaintext = self.aesgcm.decrypt(nonce, encrypted, None)
        return plaintext.decode('utf-8')

    def generate_blind_index(self, plaintext: str) -> str:
        """Generate HMAC-SHA256 blind index for searching"""
        if not plaintext:
            return ""

        h = hmac.new(self.blind_index_key, plaintext.encode('utf-8'), hashlib.sha256)
        return base64.b64encode(h.digest()).decode('utf-8')

    def encrypt_with_blind_index(self, plaintext: str) -> Tuple[str, str]:
        """Encrypt data and generate blind index"""
        encrypted = self.encrypt(plaintext)
        blind_index = self.generate_blind_index(plaintext)
        return encrypted, blind_index


def get_db_connection():
    """Create database connection"""
    database_url = os.getenv('DATABASE_URL')
    if not database_url:
        raise ValueError("DATABASE_URL environment variable is required")

    return psycopg2.connect(database_url)


def migrate_users(conn, enc_service: EncryptionService, dry_run: bool = False):
    """Migrate user data to encrypted format"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)

    print("\n=== Migrating Users ===")

    # Fetch all users that need encryption (where email_blind_index is NULL)
    cursor.execute("""
        SELECT id, email, goal
        FROM users
        WHERE email_blind_index IS NULL OR email_blind_index = ''
    """)

    users = cursor.fetchall()
    print(f"Found {len(users)} users to migrate")

    if len(users) == 0:
        print("No users to migrate (all already encrypted or email_blind_index already set)")
        return

    migrated = 0
    errors = 0

    for user in users:
        user_id = user['id']
        email = user['email']
        goal = user['goal']

        try:
            # Check if email is already encrypted (contains base64 chars and is long)
            is_encrypted = len(email) > 50 and '/' in email or '+' in email or email.endswith('==')

            if is_encrypted:
                print(f"  User {user_id}: Email appears already encrypted, skipping")
                continue

            # Encrypt email and generate blind index
            encrypted_email, email_blind_index = enc_service.encrypt_with_blind_index(email)

            # Encrypt goal if present
            encrypted_goal = None
            if goal:
                encrypted_goal = enc_service.encrypt(goal)

            if dry_run:
                print(f"  [DRY RUN] User {user_id}:")
                print(f"    Email: {email[:20]}... -> {encrypted_email[:40]}...")
                print(f"    Blind Index: {email_blind_index[:40]}...")
                if goal:
                    print(f"    Goal: {goal[:30]}... -> {encrypted_goal[:40]}...")
            else:
                # Update database
                update_cursor = conn.cursor()
                update_cursor.execute("""
                    UPDATE users
                    SET email = %s,
                        email_blind_index = %s,
                        goal = %s
                    WHERE id = %s
                """, (encrypted_email, email_blind_index, encrypted_goal, user_id))
                update_cursor.close()

                migrated += 1
                print(f"  ✓ User {user_id} migrated (email: {email})")

        except Exception as e:
            errors += 1
            print(f"  ✗ Error migrating user {user_id}: {e}")

    cursor.close()

    if not dry_run:
        conn.commit()
        print(f"\n✓ Successfully migrated {migrated} users")
    else:
        print(f"\n[DRY RUN] Would migrate {migrated} users")

    if errors > 0:
        print(f"✗ {errors} errors occurred")


def migrate_journal_entries(conn, enc_service: EncryptionService, dry_run: bool = False):
    """Migrate journal entry topics to encrypted format"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)

    print("\n=== Migrating Journal Entries ===")

    # Fetch all journal entries
    cursor.execute("""
        SELECT id, topics
        FROM journal_entries
    """)

    entries = cursor.fetchall()
    print(f"Found {len(entries)} journal entries to check")

    migrated = 0
    errors = 0
    skipped = 0

    for entry in entries:
        entry_id = entry['id']
        topics = entry['topics']

        try:
            # Skip empty topics
            if not topics:
                skipped += 1
                continue

            # Check if topics is already encrypted (contains base64 chars and is long)
            is_encrypted = len(topics) > 50 and ('/' in topics or '+' in topics or topics.endswith('=='))

            if is_encrypted:
                skipped += 1
                continue

            # Encrypt topics
            encrypted_topics = enc_service.encrypt(topics)

            if dry_run:
                print(f"  [DRY RUN] Entry {entry_id}:")
                print(f"    Topics: {topics[:30]}... -> {encrypted_topics[:40]}...")
            else:
                # Update database
                update_cursor = conn.cursor()
                update_cursor.execute("""
                    UPDATE journal_entries
                    SET topics = %s
                    WHERE id = %s
                """, (encrypted_topics, entry_id))
                update_cursor.close()

                migrated += 1
                if migrated % 100 == 0:
                    print(f"  ✓ Migrated {migrated} entries...")

        except Exception as e:
            errors += 1
            print(f"  ✗ Error migrating entry {entry_id}: {e}")

    cursor.close()

    if not dry_run:
        conn.commit()
        print(f"\n✓ Successfully migrated {migrated} journal entries")
    else:
        print(f"\n[DRY RUN] Would migrate {migrated} journal entries")

    print(f"  Skipped {skipped} entries (already encrypted or empty)")

    if errors > 0:
        print(f"✗ {errors} errors occurred")


def verify_encryption(conn, enc_service: EncryptionService):
    """Verify that encrypted data can be decrypted"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)

    print("\n=== Verifying Encryption ===")

    # Check a few users
    cursor.execute("SELECT id, email, email_blind_index, goal FROM users LIMIT 3")
    users = cursor.fetchall()

    for user in users:
        try:
            decrypted_email = enc_service.decrypt(user['email'])
            print(f"  ✓ User {user['id']}: Email decrypts to {decrypted_email}")

            # Verify blind index
            regenerated_index = enc_service.generate_blind_index(decrypted_email)
            if regenerated_index == user['email_blind_index']:
                print(f"    ✓ Blind index matches")
            else:
                print(f"    ✗ Blind index mismatch!")

            if user['goal']:
                decrypted_goal = enc_service.decrypt(user['goal'])
                print(f"    ✓ Goal decrypts to: {decrypted_goal[:50]}...")
        except Exception as e:
            print(f"  ✗ Error verifying user {user['id']}: {e}")

    # Check a few journal entries
    cursor.execute("SELECT id, topics FROM journal_entries WHERE topics IS NOT NULL AND topics != '' LIMIT 3")
    entries = cursor.fetchall()

    for entry in entries:
        try:
            decrypted_topics = enc_service.decrypt(entry['topics'])
            print(f"  ✓ Entry {entry['id']}: Topics decrypt to {decrypted_topics[:50]}...")
        except Exception as e:
            print(f"  ✗ Error verifying entry {entry['id']}: {e}")

    cursor.close()
    print("\n✓ Verification complete")


def main():
    """Main migration function"""
    print("=" * 60)
    print("Database Encryption Migration Script")
    print("=" * 60)

    # Check for dry run mode
    dry_run = '--dry-run' in sys.argv or '-n' in sys.argv
    if dry_run:
        print("\n⚠️  DRY RUN MODE - No changes will be made\n")

    # Get encryption keys from environment
    encryption_key = os.getenv('ENCRYPTION_KEY')
    blind_index_key = os.getenv('BLIND_INDEX_KEY')

    if not encryption_key or not blind_index_key:
        print("Error: ENCRYPTION_KEY and BLIND_INDEX_KEY environment variables are required")
        print("\nThese must be the same 32-byte keys used in your Go application")
        sys.exit(1)

    # Convert keys to bytes
    encryption_key_bytes = encryption_key.encode('utf-8')
    blind_index_key_bytes = blind_index_key.encode('utf-8')

    if len(encryption_key_bytes) != 32:
        print(f"Error: ENCRYPTION_KEY must be exactly 32 bytes (got {len(encryption_key_bytes)})")
        sys.exit(1)

    if len(blind_index_key_bytes) != 32:
        print(f"Error: BLIND_INDEX_KEY must be exactly 32 bytes (got {len(blind_index_key_bytes)})")
        sys.exit(1)

    print(f"Encryption key: {encryption_key[:8]}... (32 bytes)")
    print(f"Blind index key: {blind_index_key[:8]}... (32 bytes)")

    # Initialize encryption service
    try:
        enc_service = EncryptionService(encryption_key_bytes, blind_index_key_bytes)
        print("✓ Encryption service initialized")
    except Exception as e:
        print(f"Error initializing encryption service: {e}")
        sys.exit(1)

    # Connect to database
    try:
        conn = get_db_connection()
        print("✓ Database connection established")
    except Exception as e:
        print(f"Error connecting to database: {e}")
        sys.exit(1)

    try:
        # Migrate users
        migrate_users(conn, enc_service, dry_run)

        # Migrate journal entries
        migrate_journal_entries(conn, enc_service, dry_run)

        # Verify encryption (only if not dry run)
        if not dry_run:
            verify_encryption(conn, enc_service)

        print("\n" + "=" * 60)
        if dry_run:
            print("DRY RUN COMPLETE - Run without --dry-run to apply changes")
        else:
            print("MIGRATION COMPLETE")
        print("=" * 60)

    except Exception as e:
        print(f"\nError during migration: {e}")
        conn.rollback()
        sys.exit(1)

    finally:
        conn.close()


if __name__ == '__main__':
    main()
