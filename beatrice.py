from textual.app import App, ComposeResult
from textual.containers import VerticalScroll, Container
from textual.widgets import (
    Header,
    ListView,
    Static,
    Input,
    Button,
    Footer,
    LoadingIndicator,
    Label,
    ListItem,
)
import textual.validation
from client import Client
from server import BeatriceServer
from datetime import datetime


# Nickname must be between 3 and 20 characters long
def validate_nickname(value: str) -> bool:
    return 3 <= len(value) <= 20 and value.isalnum()


# Class that gets the connected_users list from the server and displays it inside its own section on the rh side of the screen
class GetOnlineUsers:
    """Populate the rh sidebar with current online users"""

    def __init__(self, client):
        self.client = client

    async def get_online_users(self):
        if self.client is not None:
            await self.client.display_connected_users()


class TimestampLabel(Label):
    """Label with automatic timestamp prepended"""

    last_date = None  # Class variable

    def __init__(self, message: str, prefix: str = "", *args, **kwargs):
        timestamp = self.get_timestamp()
        full_message = (
            f"[{timestamp}] {prefix}{message}" if prefix else f"[{timestamp}] {message}"
        )
        super().__init__(full_message, *args, **kwargs)

    @classmethod
    def get_timestamp(cls):
        now = datetime.now()
        current_date = now.strftime("%d/%m/%y")
        current_time = now.strftime("%H:%M")

        if cls.last_date is None or cls.last_date != current_date:
            cls.last_date = current_date
            return f"{current_date} {current_time}"
        else:
            return current_time


class Beatrice(App):

    # A TUI messenger named Beatrice
    CSS_PATH = "styling.tcss"

    # timestamp = datetime.now().strftime("%d-%m-%y %H:%M:%S")

    def compose(self) -> ComposeResult:
        # Nickname input field
        yield Container(
            Input(
                placeholder="Enter your nickname",
                validators=[
                    textual.validation.Function(
                        validate_nickname,
                        "Nickname must be between 3 and 20 characters long and contain only letters or nummers.",
                    )
                ],
                id="nickname_input",
                classes="nickname_input",
            ),
            id="login_screen",
        )

        # Chat display area
        yield VerticalScroll(id="chat_log", classes="vertical_scroll hidden")

        yield Container(
            Label("Online Users", id="header"),
            ListView(id="online_list"),
            id="online_user_log",
            classes="online_user_list hidden",
        )

    async def on_mount(self):
        self.title = "BEATRICE"
        self.client = None
        self.processor_task = None
        self.receiver_task = None

        self.chat_text = ""

        self.nickname_input = self.query_one("#nickname_input")

        self.online_users = None

    async def on_input_submitted(self, event: Input.Submitted):

        chat_log = self.query_one("#chat_log")
        online_user_log = self.query_one("#online_user_log")

        # /--- if the input is a nickname input, validate it and start the program
        if event.input.id == "nickname_input":

            # Nickname validation
            # if the nickname is not valid let the user know.
            if event.validation_result is not None:
                if not event.validation_result.is_valid:
                    self.notify(
                        "Nickname must be between 3 and 20 characters and contain only letters and numbers.",
                        severity="warning",
                        timeout=10,
                    )
                # if the nickname is valid, set it as the user's nickname and start the program
                else:
                    self.nickname: str = self.nickname_input.value.strip()

                    # remove the input field from the screen
                    self.query_one("#nickname_input").remove()

                    # /-- Make the chat log and online user log appear only after a valid nickname is inputted
                    chat_log = self.query_one("#chat_log")
                    chat_log.remove_class("hidden")

                    online_user_log = self.query_one("#online_user_log")
                    online_user_log.remove_class("hidden")
                    # --/

                    await chat_log.mount(
                        Label(
                            f"Welcome to Beatrice, {self.nickname}!",
                            classes="system_message",
                        )
                    )

                    chat_log.scroll_end(animate=False)

                    # init the vertical scroll and input widgets for the ui
                    await self.mount(
                        Input(
                            placeholder="Enter your message here...",
                            id="message_input",
                            classes="message_input",
                        )
                    )

                    chat_log.scroll_end(animate=False)

                    # Create a new client and connect to the server
                    self.client = Client("127.0.0.1", 55556, self.nickname)

                    # Start server connection after nickname is submitted
                    await self.client.connect_to_server(
                        self.client.host, self.client.port
                    )

                    # perform handshake
                    await self.client.check_handshake()

                    # start receive_messages indefinately
                    self.receiver_task = self.run_worker(self.client.receive_messages())

                    # start process to look at queue
                    self.processor_task = self.run_worker(self.process_events())

                    # await self.update_online_users_display()
            # ---/

        # If the input is a message, send it by calling send_message method from client.py
        elif event.input.id == "message_input":
            message = event.value
            if self.client is not None:
                await self.client.send_message(message)
            event.input.value = ""
        # ---/

        await self.update_online_users_display()

    # Method to get things from the event queue and do something with them depending on what it is.
    async def process_events(self):

        while True:

            # Initialize variables to hold the type of event and its contents
            event_type = None
            content = None

            # Get the events from the queue and handle them appropriately
            if self.client is not None:
                event_type, content = await self.client.event_queue.get()

            # await self.update_online_users_display()

            # Get reference to the chat_log and online_user_log from the DOM
            chat_log = self.query_one("#chat_log")
            online_user_log = self.query_one("#online_user_log")

            if event_type is not None:

                # if the event packet begins with "M", then it is a message from someone. We then call receive_message
                if event_type == "message":
                    sender = content.get("sender")
                    message = content.get("content")

                    fingerprint = self.client.get_fingerprint(sender)

                    await chat_log.mount(
                        Container(
                            TimestampLabel(
                                message,
                                prefix=f"{sender} ({fingerprint}): \n",
                                classes="bubble",
                            ),
                            classes="message_row received",
                        )
                    )

                # If the event_type is my_message, display it on the screen
                elif event_type == "my_message":
                    message = content.get("content")

                    # get the chat log info and send the information to it
                    chat_log = self.query_one("#chat_log")
                    await chat_log.mount(
                        Container(
                            TimestampLabel(message, prefix="Me: \n", classes="bubble"),
                            classes="message_row sent",
                        )
                    )

                # if the event type is the join packet notify the users that someone has connected
                elif event_type == "join_packet":

                    # get the chat log info and send the information to it
                    chat_log = self.query_one("#chat_log")
                    await chat_log.mount(
                        Container(
                            Label(f"{content}", classes="system_message"),
                            classes="join_packet",
                        )
                    )

                    await self.update_online_users_display()

                # if the event type is dir notify the users how many people are connected
                elif event_type == "dir":

                    # Display the number of connected users
                    chat_log = self.query_one("#chat_log")
                    await chat_log.mount(
                        Container(
                            Label(f"{content}", classes="system_message"),
                            classes="dir_packet",
                        )
                    )

                    await self.update_online_users_display()

                # if event type is leave packet notify the users who left the chat
                elif event_type == "leave_packet":

                    # get the chat log info and send the information to it
                    chat_log = self.query_one("#chat_log")
                    await chat_log.mount(
                        Container(
                            Label(f"{content}", classes="system_message"),
                            classes="leave_packet",
                        )
                    )

                    await self.update_online_users_display()

                elif event_type == "self_message_error":
                    chat_log = self.query_one("#chat_log")
                    self.notify(
                        "You cannot send a message to yourself.",
                        severity="error",
                        timeout=10,
                    )

            chat_log.scroll_end(animate=False)

    async def update_online_users_display(self):
        """Update the display of online users."""
        # Get the usernames from client
        users = await self.client.display_connected_users()

        # Get the online list from the DOM
        online_list = self.query_one("#online_list")

        # Clear existing items
        online_list.clear()

        # Add new items
        for user in users:
            online_list.append(ListItem(Label(f"🟢 {user}")))


if __name__ == "__main__":
    app = Beatrice()
    app.run()
