# Code Review: client.py Lines 1-171

**Overall Structure:**
- Client class with methods for connection, handshake, sending/receiving messages.
- Uses Fernet for encryption, JSON for packets.
- Generator-based (yields for TUI integration).
- Main block starts client and threads.
Strengths:
- Modular design with separate methods for handshake, receive, send.
- Proper error handling in network ops (recv, send).
- Encryption/decryption implemented for DMs.
- Threading for concurrent receive/send.

## Major Issues & Fixes:
1. Generator Methods in __init__ (Lines 30-31): create_socket, check_handshake, set_nickname yield values, but __init__ doesn't consume them. This causes unhandled generators. Fix: Change yields to prints or return values; handle in main/TUI.

2. set_nickname (Lines 42-49): Recurses on invalid input without return, causing infinite loop if empty. Yields instead of returning. Fix: Return nickname on valid; raise/return error on invalid. Use loop instead of recursion.

3. create_socket (Lines 33-40): Yields "Starting up..." but exits on failure. Fix: Return bool or raise exception; let caller handle.

4. check_handshake (Lines 51-75): Yields on errors but returns bool. Inconsistent with other methods. Fix: Standardize to return status + message, or always yield.

5. Cipher Setup (Lines 87-88 in receive_packet): Switches self.cipher to received key, but should use client's key for symmetry. Fix: Remove cipher update; keep self.cipher as client's key.

6. send_message (Lines 130-134): Incomplete—only checks self.connected, no send logic. Fix: Add encryption and send code like the commented send_messages.

7. receive_messages (Lines 93-127): Good generator, but InvalidToken except has no body (line 115-117). all() check (line 121) uses message_str (raw), but should check packet fields. Fix: Add pass or log in except; refine validation.

8. Commented Code (Lines 136-164): send_messages is commented, but called in main (line 171). Fix: Uncomment and adapt to class method, or implement in main loop.

9. Main Block (Lines 167-171): Calls send_messages() which is commented. Fix: Implement send loop in main or uncomment method.

10. Type Hints: Missing on most methods (e.g., -> Generator[str, None, None] for yielding methods).

11. Security: Nickname input without validation; key exchange assumes trust.

12. Timestamp Format (Line 27): "%d-%m-%Y %H:%M" is non-ISO. Fix: Use "%Y-%m-%d %H:%M:%S".

13. Packet Dict (Lines 23-29): Unused in code; type commented. Fix: Remove or integrate.

**Recommendations:**
- Standardize error handling: use exceptions or consistent return/yield.
- Test handshake, encryption, disconnects.
- Refactor for TUI: ensure yields are consumed.
- Add logging instead of yields for non-TUI output.
- Run mypy for types, flake8 for style.
Notes: Code is ~70% complete but needs fixes for stability. Focus on generator handling and send logic next. Server/beatrice refactoring will likely reveal integration issues.
