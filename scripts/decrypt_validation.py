#!/usr/bin/env python3
"""
Decryption and validation script for encrypted database data.

This script:
1. Retrieves encrypted data from the database
2. Decrypts sensitive fields for validation
3. Verifies blind indexes
4. Exports decrypted data for review

Requirements:
    pip install psycopg2-binary python-dotenv cryptography

Usage:
    python scripts/decrypt_validation.py [options]

Options:
    --user-id ID        Decrypt specific user by ID
    --user-email EMAIL  Decrypt user by email (searches blind index)
    --entry-id ID       Decrypt specific journal entry by ID
    --all-users         Decrypt all users
    --all-entries       Decrypt all journal entries
    --limit N           Limit number of entries (default: 100)

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
from typing import Optional, Dict, Any, List
from dotenv import load_dotenv
import psycopg2
from psycopg2.extras import RealDictCursor
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

# Load environment variables
load_dotenv()


class DecryptionService:
    """Python implementation of the Go decryption service"""

    def __init__(self, encryption_key: bytes, blind_index_key: bytes):
        if len(encryption_key) != 32:
            raise ValueError("Encryption key must be 32 bytes")
        if len(blind_index_key) != 32:
            raise ValueError("Blind index key must be 32 bytes")

        self.aesgcm = AESGCM(encryption_key)
        self.blind_index_key = blind_index_key

    def decrypt(self, ciphertext: str) -> str:
        """Decrypt ciphertext using AES-256-GCM"""
        if not ciphertext:
            return ""

        try:
            # Base64 decode
            data = base64.b64decode(ciphertext)

            # Extract nonce and ciphertext
            nonce = data[:12]
            encrypted = data[12:]

            # Decrypt
            plaintext = self.aesgcm.decrypt(nonce, encrypted, None)
            return plaintext.decode('utf-8')
        except Exception as e:
            raise ValueError(f"Decryption failed: {e}")

    def generate_blind_index(self, plaintext: str) -> str:
        """Generate HMAC-SHA256 blind index for searching"""
        if not plaintext:
            return ""

        h = hmac.new(self.blind_index_key, plaintext.encode('utf-8'), hashlib.sha256)
        return base64.b64encode(h.digest()).decode('utf-8')

    def verify_blind_index(self, plaintext: str, blind_index: str) -> bool:
        """Verify that a blind index matches the plaintext"""
        generated_index = self.generate_blind_index(plaintext)
        return generated_index == blind_index


def get_db_connection():
    """Create database connection"""
    database_url = os.getenv('DATABASE_URL')
    if not database_url:
        raise ValueError("DATABASE_URL environment variable is required")

    return psycopg2.connect(database_url)


def decrypt_user(conn, dec_service: DecryptionService, user_id: Optional[int] = None,
                 email: Optional[str] = None) -> Optional[Dict[str, Any]]:
    """Decrypt a single user's data"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)

    if user_id:
        cursor.execute("SELECT * FROM users WHERE id = %s", (user_id,))
    elif email:
        # Search by blind index
        blind_index = dec_service.generate_blind_index(email)
        cursor.execute("SELECT * FROM users WHERE email_blind_index = %s", (blind_index,))
    else:
        raise ValueError("Must provide either user_id or email")

    user = cursor.fetchone()
    cursor.close()

    if not user:
        return None

    # Decrypt sensitive fields
    decrypted_user = dict(user)

    try:
        decrypted_user['email_decrypted'] = dec_service.decrypt(user['email'])

        # Verify blind index
        if user['email_blind_index']:
            is_valid = dec_service.verify_blind_index(
                decrypted_user['email_decrypted'],
                user['email_blind_index']
            )
            decrypted_user['blind_index_valid'] = is_valid

        if user['goal']:
            decrypted_user['goal_decrypted'] = dec_service.decrypt(user['goal'])

    except Exception as e:
        decrypted_user['decryption_error'] = str(e)

    return decrypted_user


def decrypt_all_users(conn, dec_service: DecryptionService) -> List[Dict[str, Any]]:
    """Decrypt all users' data"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)
    cursor.execute("SELECT * FROM users ORDER BY id")
    users = cursor.fetchall()
    cursor.close()

    decrypted_users = []

    for user in users:
        decrypted_user = dict(user)

        try:
            decrypted_user['email_decrypted'] = dec_service.decrypt(user['email'])

            # Verify blind index
            if user['email_blind_index']:
                is_valid = dec_service.verify_blind_index(
                    decrypted_user['email_decrypted'],
                    user['email_blind_index']
                )
                decrypted_user['blind_index_valid'] = is_valid

            if user['goal']:
                decrypted_user['goal_decrypted'] = dec_service.decrypt(user['goal'])

        except Exception as e:
            decrypted_user['decryption_error'] = str(e)

        decrypted_users.append(decrypted_user)

    return decrypted_users


def decrypt_journal_entry(conn, dec_service: DecryptionService, entry_id: int) -> Optional[Dict[str, Any]]:
    """Decrypt a single journal entry's data"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)
    cursor.execute("SELECT * FROM journal_entries WHERE id = %s", (entry_id,))
    entry = cursor.fetchone()
    cursor.close()

    if not entry:
        return None

    decrypted_entry = dict(entry)

    try:
        if entry['topics']:
            decrypted_entry['topics_decrypted'] = dec_service.decrypt(entry['topics'])
    except Exception as e:
        decrypted_entry['decryption_error'] = str(e)

    return decrypted_entry


def decrypt_all_entries(conn, dec_service: DecryptionService, limit: int = 100) -> List[Dict[str, Any]]:
    """Decrypt journal entries (limited for performance)"""
    cursor = conn.cursor(cursor_factory=RealDictCursor)
    cursor.execute(f"SELECT * FROM journal_entries ORDER BY id DESC LIMIT %s", (limit,))
    entries = cursor.fetchall()
    cursor.close()

    decrypted_entries = []

    for entry in entries:
        decrypted_entry = dict(entry)

        try:
            if entry['topics']:
                decrypted_entry['topics_decrypted'] = dec_service.decrypt(entry['topics'])
        except Exception as e:
            decrypted_entry['decryption_error'] = str(e)

        decrypted_entries.append(decrypted_entry)

    return decrypted_entries


def print_user(user: Dict[str, Any]):
    """Pretty print user data"""
    print(f"\n{'='*60}")
    print(f"User ID: {user.get('id')}")
    print(f"Username: {user.get('username')}")
    print(f"Email (encrypted): {user.get('email', '')[:50]}...")
    print(f"Email (decrypted): {user.get('email_decrypted', 'N/A')}")
    print(f"Blind Index Valid: {user.get('blind_index_valid', 'N/A')}")

    if user.get('goal'):
        print(f"Goal (encrypted): {user.get('goal', '')[:50]}...")
        print(f"Goal (decrypted): {user.get('goal_decrypted', 'N/A')}")

    if user.get('decryption_error'):
        print(f"⚠️  Decryption Error: {user['decryption_error']}")

    print(f"Created: {user.get('created_at')}")
    print(f"{'='*60}")


def print_entry(entry: Dict[str, Any]):
    """Pretty print journal entry data"""
    print(f"\n{'='*60}")
    print(f"Entry ID: {entry.get('id')}")
    print(f"User ID: {entry.get('user_id')}")

    if entry.get('topics'):
        print(f"Topics (encrypted): {entry.get('topics', '')[:50]}...")
        print(f"Topics (decrypted): {entry.get('topics_decrypted', 'N/A')}")
    else:
        print("Topics: None")

    if entry.get('decryption_error'):
        print(f"⚠️  Decryption Error: {entry['decryption_error']}")

    print(f"Created: {entry.get('created_at')}")
    print(f"{'='*60}")


def main():
    """Main validation function"""
    print("=" * 60)
    print("Database Decryption & Validation Script")
    print("=" * 60)

    # Get encryption keys from environment
    encryption_key = os.getenv('ENCRYPTION_KEY')
    blind_index_key = os.getenv('BLIND_INDEX_KEY')

    if not encryption_key or not blind_index_key:
        print("Error: ENCRYPTION_KEY and BLIND_INDEX_KEY environment variables are required")
        sys.exit(1)

    # Convert keys to bytes
    encryption_key_bytes = encryption_key.encode('utf-8')
    blind_index_key_bytes = blind_index_key.encode('utf-8')

    if len(encryption_key_bytes) != 32 or len(blind_index_key_bytes) != 32:
        print("Error: Keys must be exactly 32 bytes")
        sys.exit(1)

    # Initialize decryption service
    try:
        dec_service = DecryptionService(encryption_key_bytes, blind_index_key_bytes)
        print("✓ Decryption service initialized\n")
    except Exception as e:
        print(f"Error initializing decryption service: {e}")
        sys.exit(1)

    # Connect to database
    try:
        conn = get_db_connection()
        print("✓ Database connection established\n")
    except Exception as e:
        print(f"Error connecting to database: {e}")
        sys.exit(1)

    try:
        # Parse command line arguments
        if '--user-id' in sys.argv:
            idx = sys.argv.index('--user-id')
            user_id = int(sys.argv[idx + 1])
            user = decrypt_user(conn, dec_service, user_id=user_id)
            if user:
                print_user(user)
            else:
                print(f"User {user_id} not found")

        elif '--user-email' in sys.argv:
            idx = sys.argv.index('--user-email')
            email = sys.argv[idx + 1]
            user = decrypt_user(conn, dec_service, email=email)
            if user:
                print_user(user)
            else:
                print(f"User with email {email} not found")

        elif '--entry-id' in sys.argv:
            idx = sys.argv.index('--entry-id')
            entry_id = int(sys.argv[idx + 1])
            entry = decrypt_journal_entry(conn, dec_service, entry_id)
            if entry:
                print_entry(entry)
            else:
                print(f"Journal entry {entry_id} not found")

        elif '--all-users' in sys.argv:
            users = decrypt_all_users(conn, dec_service)
            print(f"Decrypted {len(users)} users:\n")
            for user in users:
                print_user(user)

        elif '--all-entries' in sys.argv:
            limit = 100
            if '--limit' in sys.argv:
                idx = sys.argv.index('--limit')
                limit = int(sys.argv[idx + 1])

            entries = decrypt_all_entries(conn, dec_service, limit)
            print(f"Decrypted {len(entries)} journal entries (limit: {limit}):\n")
            for entry in entries:
                print_entry(entry)

        else:
            print("Usage:")
            print("  --user-id ID                Decrypt specific user by ID")
            print("  --user-email EMAIL          Decrypt user by email")
            print("  --entry-id ID               Decrypt specific journal entry by ID")
            print("  --all-users                 Decrypt all users")
            print("  --all-entries               Decrypt all journal entries")
            print("  --limit N                   Limit number of entries (default: 100)")
            print("\nExamples:")
            print("  python scripts/decrypt_validation.py --user-id 1")
            print("  python scripts/decrypt_validation.py --user-email user@example.com")
            print("  python scripts/decrypt_validation.py --all-users")
            print("  python scripts/decrypt_validation.py --all-entries --limit 50")

        print("\n" + "=" * 60)
        print("VALIDATION COMPLETE")
        print("=" * 60)

    except Exception as e:
        print(f"\nError during validation: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)

    finally:
        conn.close()


if __name__ == '__main__':
    main()
