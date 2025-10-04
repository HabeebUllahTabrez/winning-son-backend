#!/usr/bin/env python3
"""
Generate secure 32-byte encryption keys for the application.

Usage:
    python scripts/generate_keys.py
"""

import os
import secrets

def generate_key() -> str:
    """Generate a secure 32-byte key"""
    # Generate 32 random bytes
    key_bytes = secrets.token_bytes(32)
    # Convert to a string (keeping as raw bytes in string form)
    # We'll use the first 32 characters of the hex representation
    return key_bytes.hex()[:32]

def main():
    print("=" * 60)
    print("Encryption Key Generator")
    print("=" * 60)
    print()
    print("Generating secure 32-byte keys for encryption...\n")

    encryption_key = generate_key()
    blind_index_key = generate_key()

    print(f"ENCRYPTION_KEY=\"{encryption_key}\"")
    print(f"BLIND_INDEX_KEY=\"{blind_index_key}\"")
    print()
    print("=" * 60)
    print("⚠️  IMPORTANT SECURITY NOTES:")
    print("=" * 60)
    print("1. Copy these keys to your .env file")
    print("2. NEVER commit these keys to version control")
    print("3. Store these keys securely (use a password manager)")
    print("4. If you lose these keys, encrypted data CANNOT be recovered")
    print("5. Use the same keys for both the Go app and migration script")
    print()
    print("Add to your .env file:")
    print()
    print(f"ENCRYPTION_KEY={encryption_key}")
    print(f"BLIND_INDEX_KEY={blind_index_key}")
    print()

if __name__ == '__main__':
    main()
