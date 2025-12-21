# Chat Application - TODO List

## 🔴 CRITICAL - Fix These First (App Won't Run)

### Client - Method Signature Bug
- [ ] **Fix `handle_user_input()` method**
  - Currently: Method expects `raw_content: str` parameter
  - Currently: Called with `asyncio.create_task(self.handle_user_input())` - no parameter!
  - Solution: Either remove it as a background task OR make it take input from a different source
  - Recommended: Rename to `send_message(content: str)` and call it directly from Textual's input handler

### Client - Remove Broken Architecture
- [ ] **Delete the `run_tui()` stub method**
  - It just logs and does nothing
  - Textual will manage its own event loop, not run as a subtask

- [ ] **Refactor `start()` method**
  - Don't create 3 concurrent tasks (receiver, tui, input)
  - Only receiver should be a background task
  - Textual app should call client methods directly

---

## 🟠 HIGH PRIORITY - Core Functionality

### Client - Textual Integration
- [ ] **Restructure initialization flow**
  - Create separate methods: `connect_to_server()`, `perform_handshake()`
  - Make these return boolean (True/False for success)
  - Call them BEFORE starting Textual app

- [ ] **Add `start_receiver()` method**
  - Starts the `receive_messages()` background task
  - Call this from Textual's `on_mount()` event

- [ ] **Fix event queue data structure**
  - Your type hint says: `Tuple[str, str, str]` for messages
  - You're actually putting: `Tuple[str, dict]`
  - Make them consistent - recommend using dict for flexibility

### Client - Event Processing
- [ ] **Create event consumer in Textual app**
  - Make a `process_client_events()` async method
  - Continuously pull from `client.event_queue`
  - Update UI based on event type
  - Run this as a background task in Textual

### Server - Security Basics
- [ ] **Add packet size limits**
  - Currently using `reader.readline()` with no limit
  - Malicious client could send gigabytes
  - Add `MAX_PACKET_SIZE` constant (suggest 100KB)
  - Read with size checks

- [ ] **Fix handshake timeout**
  - Remove the `while True` loop with timeout that catches ALL exceptions
  - Use single `asyncio.wait_for(packet, timeout=10.0)`
  - Handle `asyncio.TimeoutError` specifically
  - Don't break silently on timeout

---

## 🟡 MEDIUM PRIORITY - Stability & UX

### Server - Better Error Handling
- [ ] **Stop using bare `except: pass`**
  - You have 5+ places with silent exception swallowing
  - At minimum: log the exception before passing
  - Better: handle specific exceptions differently

- [ ] **Fix nickname collision logic**
  - Current: Tries once with random number
  - Problem: What if `Alice#123` already exists?
  - Add: While loop with counter, max 9999 attempts

### Client - Message Display
- [ ] **Actually display received messages**
  - Messages go into `event_queue` but are never consumed
  - Textual app needs to pull from queue and update UI
  - Add proper formatting (sender, timestamp, etc.)

### Server - Message Loop Improvements
- [ ] **Add timeout for idle connections**
  - Wrap `_receive_packet()` in `asyncio.wait_for()`
  - Suggest 5 minute timeout
  - Prevents dead connections from hanging

- [ ] **Validate packet types**
  - Check that message loop only receives "M" type packets
  - Log/reject unexpected types
  - Prevents confusion from misrouted packets

---

## 🟢 LOW PRIORITY - Polish & Features

### Server - Rate Limiting
- [ ] **Add message rate limiting**
  - Track message counts per user
  - Limit to X messages per minute (suggest 60)
  - Send error packet if exceeded
  - Clean up old timestamps

### Client - Input Validation
- [ ] **Strengthen nickname validation**
  - Already checks `isalnum()` - good
  - Add: Check for max length BEFORE connection
  - Add: Better error messages

### Server - Cleanup Improvements  
- [ ] **Use `list()` when iterating dict you're modifying**
  - In `_cleanup()`, you iterate `connected_users` while others might modify it
  - Wrap in `list()`: `for user in list(self.connected_users.keys())`

### Client - Reconnection Logic
- [ ] **Add auto-reconnect on disconnect**
  - Detect when connection is lost
  - Attempt to reconnect with exponential backoff
  - Show reconnection status in UI

### Both - Better Logging
- [ ] **Replace print statements with logger**
  - Server has `logger` but still uses `print()` in cleanup
  - Use appropriate log levels (debug, info, warning, error)
  - Helps with debugging production issues

---

## 📋 TEXTUAL INTEGRATION CHECKLIST

### Textual App Structure
- [ ] **Create Textual App class**
  - Accept `Client` instance in `__init__`
  - Don't try to create client inside Textual

- [ ] **Setup UI in `compose()` method**
  - Add `RichLog` for messages
  - Add `Input` widget for typing
  - Add `Static` widget for user list (optional)

- [ ] **Initialize in `on_mount()` event**
  - Call `client.start_receiver()`
  - Start your `process_client_events()` task
  - Focus the input widget

- [ ] **Handle input in `on_input_submitted()` event**
  - Get message from `event.value`
  - Clear input immediately
  - Call `await client.send_message(message)`

- [ ] **Process events from queue**
  - Create async loop: `await client.event_queue.get()`
  - Match on event type: "message", "join", "leave", "error", etc.
  - Update UI widgets accordingly

- [ ] **Cleanup on exit**
  - Override `action_quit()` or similar
  - Call `await client.disconnect()`
  - Let Textual handle the rest

### Main Function
- [ ] **Get nickname BEFORE Textual starts**
  - Use regular `input()` call
  - Validate it
  - Don't try to do this inside Textual

- [ ] **Initialize and connect client**
  - Create Client instance
  - `await client.connect_to_server()`
  - `await client.perform_handshake()`
  - Check return values - exit if failed

- [ ] **Start Textual with connected client**
  - Pass client to your App class
  - Call `await app.run_async()`

---

## 🎯 RECOMMENDED ORDER OF IMPLEMENTATION

1. **Fix the critical `handle_user_input()` bug** (5 min)
   - Just change it to `send_message(content: str)`
   - Remove the `create_task()` call in `start()`

2. **Restructure client initialization** (15 min)
   - Split `start()` into separate methods
   - Make connection/handshake return booleans

3. **Create basic Textual app** (30 min)
   - Just get messages displaying
   - Don't worry about fancy UI yet

4. **Wire up event processing** (20 min)
   - Connect queue to UI updates
   - Handle all event types

5. **Test with 2+ clients** (15 min)
   - Verify messages work
   - Check join/leave notifications

6. **Apply server security fixes** (20 min)
   - Packet size limits
   - Handshake timeout fix

7. **Polish and add features** (ongoing)
   - Rate limiting
   - Better error messages
   - Reconnection logic

---

## 🧪 TESTING CHECKLIST

After each change, verify:
- [ ] Server starts without errors
- [ ] Client connects successfully
- [ ] Can send/receive messages
- [ ] Multiple clients work
- [ ] Disconnection is handled
- [ ] Errors display properly
- [ ] UI updates correctly

---

## 📚 KEY CONCEPTS TO UNDERSTAND

### Event Queue Pattern
- Client puts events in queue: `await self.event_queue.put(("message", data))`
- Textual pulls from queue: `event = await client.event_queue.get()`
- This decouples networking from UI

### Async Task Management
- Background tasks: `asyncio.create_task(coro())`
- Wait for completion: `await task`
- Cancel: `task.cancel()`
- Textual has special `@work` decorator for tasks

### Textual Event Handlers
- `on_mount()` - when widget/app is added to DOM
- `on_input_submitted()` - when user presses Enter
- `action_quit()` - when user presses Ctrl+C
- All can be async methods

### Connection Lifecycle
1. Create client instance (sync)
2. Connect to server (async)
3. Perform handshake (async)
4. Start receiver task (async)
5. Run UI (async)
6. Disconnect on exit (async)

---

## ⚠️ COMMON PITFALLS TO AVOID

- **DON'T** try to run Textual as a subtask of your client
- **DON'T** call blocking I/O inside Textual event handlers
- **DON'T** forget to await async calls
- **DON'T** modify dicts while iterating them
- **DON'T** swallow exceptions silently
- **DO** use `await` for all async operations
- **DO** handle connection failures gracefully
- **DO** validate user input before sending to network
- **DO** update UI from the main thread (via event queue)

---

## 🆘 IF YOU GET STUCK

**Error: "Reader not initialized"**
→ Call `connect_to_server()` before using client

**Error: "Missing 1 required positional argument"**
→ You forgot to fix `handle_user_input()` signature

**UI not updating**
→ Check that `process_client_events()` task is running

**Messages not appearing**
→ Verify events are being put in queue (add debug prints)

**Can't send messages**
→ Make sure you're calling `send_message()` not the old method

**Type errors with event tuple**
→ Make sure event format is consistent: `(str, dict)`
