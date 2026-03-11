import os
import sys

from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa


def check_or_create_keys():

    if sys.platform == "win32":
        # Windows
        private_key_path = os.path.expandvars(r"%APPDATA%\beatrice\private_key.pem")
    elif sys.platform == "darwin":
        # macOS
        private_key_path = os.path.expanduser("~/Library/Application Support/beatrice/private_key.pem")
    else:
        # Linux and other Unix
        private_key_path = os.path.expanduser("~/.config/beatrice/private_key.pem")

    # Check if the private key file exists
    if os.path.exists(private_key_path):
        # Load RSA private key
        with open(private_key_path, "rb") as f:
            private_key = serialization.load_pem_private_key(f.read(), password=None)

    else:
        private_key = rsa.generate_private_key(
            public_exponent=65537, key_size=2048, backend=default_backend()
        )

        os.makedirs(os.path.dirname(private_key_path), exist_ok=True)
        with open(private_key_path, 'wb') as f:
            f.write(private_key.private_bytes(
                encoding=serialization.Encoding.PEM,
                format=serialization.PrivateFormat.PKCS8,
                encryption_algorithm=serialization.NoEncryption()
            ))

    public_key = private_key.public_key()

    return public_key, private_key

def get_public_key_bytes(public_key):
    public_key_str = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
    # /--- Serialize public key for transmission
    return public_key_str.decode("utf-8")

