import hashlib
import os

password = "testing"
salt = os.urandom(16).hex()
to_hash = salt + password
hash_hex = hashlib.sha256(to_hash.encode()).hexdigest()

print("Salt:", salt)
print("Password hash:", hash_hex)
