import json
import os
import timeit

# All crypto imports
# /--- PyCryptodome
from Crypto.Cipher import AES, PKCS1_OAEP
from Crypto.Hash import SHA256
from Crypto.PublicKey import RSA
# /--- Cryptography
from cryptography.hazmat import backends
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding, rsa
from cryptography.hazmat.primitives.asymmetric.rsa import (RSAPrivateKey,
                                                           RSAPublicKey)
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def bench_cryptography_rsa_aes():

    RSA_KEY_SIZE = 2048
    AES_KEY_SIZE = 256

    # Private key setup
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=RSA_KEY_SIZE)

    # Public key setup
    public_key = private_key.public_key()

    # AES setup
    aes_key = AESGCM.generate_key(bit_length=AES_KEY_SIZE)
    aesgcm = AESGCM(aes_key)
    iv_nonce = os.urandom(12)

    # Dummy json packet
    dummy_payload = {
        "sender": "Bob",
        "content": "Hello, world!",
    }
    dummy_payload_packet = json.dumps(dummy_payload).encode("utf-8")

    # Encrypt packet with AES
    encrypted_content_bytes = aesgcm.encrypt(iv_nonce, dummy_payload_packet, None)

    # Encrypt AES with RSA key
    encrypted_aes_key = public_key.encrypt(
        aes_key,
        padding.OAEP(
            mgf=padding.MGF1(algorithm=hashes.SHA256()),
            algorithm=hashes.SHA256(),
            label=None,
        )
    )

    # Decrypt AES key with private key
    decrypted_aes_key = private_key.decrypt(
        encrypted_aes_key,
        padding.OAEP(
            mgf=padding.MGF1(algorithm=hashes.SHA256()),
            algorithm=hashes.SHA256(),
            label=None,
        ),
    )

    # Decrypt payload with AES
    aesgcm_dec = AESGCM(decrypted_aes_key)
    decrypted_content_bytes = aesgcm_dec.decrypt(
        iv_nonce, encrypted_content_bytes, None
    )

def bench_pycryptodome_rsa_aes():

    RSA_KEY_SIZE = 2048
    AES_KEY_SIZE = 32 # bytes (256 bits)


    # Private/Public key setup
    private_key = RSA.generate(RSA_KEY_SIZE)
    public_key = private_key.public_key()

    # AES setup
    aes_key = os.urandom(AES_KEY_SIZE)
    iv_nonce = os.urandom(12)

    # Dummy json packet
    dummy_payload = {
        "sender": "Bob",
        "content": "Hello, world!",
    }
    dummy_payload_packet = json.dumps(dummy_payload).encode("utf-8")

    # Encrypt with AES
    aes_cipher = AES.new(aes_key, AES.MODE_GCM, nonce=iv_nonce)
    ciphertext, mac_tag = aes_cipher.encrypt_and_digest(dummy_payload_packet)
    # Concatenating to match cryptography's behavior of a single byte string
    encrypted_content_bytes = ciphertext + mac_tag

    # Encrypt AES key with RSA key
    rsa_cipher = PKCS1_OAEP.new(public_key, hashAlgo=SHA256.new())
    encrypted_aes_key = rsa_cipher.encrypt(aes_key)

    # Decrypt AES key with Private key
    rsa_cipher_dec = PKCS1_OAEP.new(private_key, hashAlgo=SHA256.new())
    decrypted_aes_key = rsa_cipher_dec.decrypt(encrypted_aes_key)

    # Decrypt Payload with AES
    # Splitting the ciphertext and tag back out
    ct_len = len(encrypted_content_bytes) - 16
    ct, tag = encrypted_content_bytes[:ct_len], encrypted_content_bytes[ct_len:]

    aes_cipher_dec = AES.new(decrypted_aes_key, AES.MODE_GCM, nonce=iv_nonce)
    decrypted_content_bytes = aes_cipher_dec.decrypt_and_verify(ct, tag)

if __name__ == "__main__":
    # Running 100 iterations for a more stable benchmark average
    iterations = 10

    print(f"Running benchmark ({iterations} iterations)...")

    crypto_time = timeit.timeit(bench_cryptography_rsa_aes, number=iterations)
    print(f"Cryptography:   {crypto_time:.4f} seconds")

    pycrypto_time = timeit.timeit(bench_pycryptodome_rsa_aes, number=iterations)
    print(f"PyCryptodome:  {pycrypto_time:.4f} seconds")


