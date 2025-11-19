# Client Class Refactoring TODO List
## Preparing for Textual Integration with Asyncio and Hybrid RSA-AES Encryption

---

## Phase 1: Clean Up Current Code

1. **Remove all old socket references** - Find every instance of `self.client` and related socket methods that are no longer valid with asyncio
2. **Remove Fernet imports and usage** - You're moving to RSA/AES hybrid, so clean out the old symmetric encryption code
3. **Fix the `receive_messages` method** - It still uses old socket `recv()` calls instead of asyncio reader
4. **Remove or complete `receive_packet` method** - Decide if you need this as a separate method or if its logic should be in `receive_messages`

---

## Phase 2: Fix Async Structure Issues

5. **Move `create_socket` and `check_handshake` outside of `initialize()`** - They're currently nested functions inside `initialize()`, make them proper async methods of the class
6. **Fix `create_socket` to accept host and port as parameters** - Don't hardcode, pass them in
7. **Remove `__init__` calls to connection methods** - `__init__` should not call `create_socket()` or `check_handshake()` since they're now async
8. **Fix the `reader` property setter** - The decorator is wrong (should be `@reader.setter` not `@_reader.setter`)
9. **Convert all `yield` statements that should be exceptions** - In `check_handshake`, you're yielding True/False when you should raise exceptions or return values
10. **Remove `self.client.send()` calls** - In `check_handshake`, you're using old socket syntax instead of `self.writer.write()`

---

## Phase 3: Implement Hybrid RSA-AES Encryption

11. **Create `encrypt_message()` method** - Should take message and recipient, generate AES key, encrypt message with AES, encrypt AES key with recipient's RSA public key
12. **Create `decrypt_message()` method** - Should take encrypted package, decrypt AES key with your RSA private key, decrypt message with AES key
13. **Research AES-CBC or AES-GCM** - Look into `cryptography.hazmat.primitives.ciphers` for AES implementation
14. **Handle IV (Initialization Vector) for AES** - You'll need to generate random IV and include it in your encrypted packet
15. **Research symmetric padding** - AES requires message padding, look into `cryptography.hazmat.primitives.padding.PKCS7`

---

## Phase 4: Message Protocol Updates

16. **Update packet structure for hybrid encryption** - Encrypted packets need to contain: encrypted_aes_key, iv, ciphertext (all base64 encoded)
17. **Integrate encryption into `send_message`** - Call your encrypt method for DMs before sending
18. **Integrate decryption into `receive_messages`** - Call your decrypt method when receiving DMs
19. **Handle KEY packet type properly** - When receiving KEY packets, call `store_user_public_key()` instead of old Fernet logic
20. **Add error handling for missing public keys** - What happens if you try to send a DM to someone whose key you don't have?

---

## Phase 5: Server Configuration

21. **Create a configuration system** - Look into using a config file (JSON, YAML, or .env) or command-line arguments for host/port
22. **Research `argparse` library** - For accepting host/port as command-line arguments
23. **Research `configparser` or `python-dotenv`** - For loading configuration from files
24. **Pass host/port to `initialize()` method** - Have it accept these as parameters
25. **Add validation for host/port** - Check they're valid before attempting connection

---

## Phase 6: Secure Key Storage

26. **Research where to store private keys** - Look into storing in memory only (never write to disk for a chat client)
27. **Consider key serialization for persistence** - If you want keys to persist across sessions, research `cryptography.hazmat.primitives.serialization` for encrypting private keys with a password
28. **Research `getpass.getpass()`** - For securely getting a password to encrypt the private key (if storing to disk)
29. **Decide on key lifecycle** - Generate new keys each session (ephemeral) or persist them?
30. **Never log or print private keys** - Audit your code to ensure private keys never appear in logs/console

---

## Phase 7: Async Message Receiving

31. **Convert `receive_messages` to async generator** - Change to `async def` and use `async for` pattern
32. **Use `await self.reader.read()` instead of `recv()`** - Replace all blocking socket calls
33. **Handle connection closure** - Check for empty bytes from reader and raise appropriate exception
34. **Add proper exception types** - Create custom exceptions like `ConnectionClosedError`, `DecryptionError`

---

## Phase 8: Complete send_message Implementation

35. **Finish the `send_message` method** - Currently just checks `if self.connected:` and does nothing
36. **Accept message and recipient parameters** - Make it flexible for DMs vs broadcasts
37. **Build appropriate packet structure** - Include type, sender, recipient, timestamp, content
38. **Use `self.writer.write()` and `await self.writer.drain()`** - For actually sending data
39. **Add message type logic** - Encrypt for DMs, leave plaintext for broadcasts

---

## Phase 9: Textual Integration Prep

40. **Ensure all user-facing messages are yielded** - Status updates, errors, chat messages should all be yielded from async generators
41. **Make `initialize()` an async generator** - Yield connection status updates that Textual can display
42. **Add a `disconnect()` method** - Async method to cleanly close writer and reader
43. **Add connection status property** - So Textual can check if connected before allowing sends
44. **Remove the `if __name__ == "__main__"` block** - Or make it a simple test harness, since Textual will be the entry point

---

## Phase 10: Error Handling and Edge Cases

45. **Handle late-joining clients** - What happens if you try to send DM before receiving their public key?
46. **Add timeout handling** - Research `asyncio.wait_for()` for connection timeouts
47. **Handle malformed packets gracefully** - Don't crash on invalid JSON or missing fields
48. **Add logging** - Research Python's `logging` module for debugging
49. **Handle reconnection logic** - Can the client reconnect after disconnection?

---

## Critical Path (Do These First)

- **Items 1-4** (remove old code)
- **Items 5-10** (fix async structure)
- **Items 21-24** (server configuration)
- **Items 31-34** (fix receive_messages)
- **Items 35-39** (complete send_message)

---

## Research Resources to Look Into

- **asyncio documentation** - Streams, event loops, coroutines
- **cryptography library docs** - Hybrid encryption examples, AES modes
- **Textual documentation** - Workers, message passing, async integration
- **Base64 encoding** - For sending binary encrypted data as JSON
- **JSON packet design** - Best practices for message protocols

---

## Notes

Once you complete these TODOs, your Client will be a clean async API ready for Textual integration with proper hybrid encryption.
