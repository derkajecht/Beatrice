from textual.app import App, ComposeResult
from textual.containers import VerticalScroll
from textual.widgets import Header, Static, Input, Button, Footer, LoadingIndicator
import textual.validation
from client import Client
from server import BeatriceServer
import asyncio


def validate_nickname(value: str) -> bool:
    return (
        3 <= len(value) <= 20 and value.isalnum()
    )  # Nickname must be between 3 and 20 characters long


class Beatrice(App):
    # A TUI messenger named Beatrice
    CSS_PATH = "styling.css"

    def compose(self) -> ComposeResult:

        # Nickname input field
        yield Input(
            placeholder="Enter you nickname...",
            validators=[
                textual.validation.Function(
                    validate_nickname,
                    "Nickname must be between 3 and 20 characters long and contain only letters or nummers.",
                )
            ],
            id="nickname_input",
        )

        # Chat display area
        yield VerticalScroll(id="chat_log")

        yield Header()

    async def on_mount(self):
        self.title = "BEATRICE"
        self.client = None
        self.processor_task = None
        self.receiver_task = None

        self.chat_text = ""

        self.nickname_input = self.query_one("#nickname_input")

        # self.server = BeatriceServer("127.0.0.1", 55556)
        # self.run_worker(self.server.start_server())

    async def on_input_submitted(self, event: Input.Submitted) -> None:

        # /--- if the input is a nickname input, validate it and start the program
        if event.input.id == "nickname_input":

            # nickname validation

            # if the nickname is not valid let the user know.
            # TODO: Does this re-prompt them to input a valid nickname?
            if not event.validation_result.is_valid:
                self.notify(
                    "Nickname must be between 3 and 20 characters and contain only letters and numbers"
                )
            # if the nickname is valid, set it as the user's nickname and start the program
            else:
                self.nickname: str = self.nickname_input.value.strip()
                self.notify(
                    f"Welcome to Beatrice {self.nickname}! Server is loading..."
                )
                # remove the input field from the screen
                self.query_one("#nickname_input").remove()

                # init the vertical scroll and input widgets for the ui
                # await self.mount(VerticalScroll(id="chat_log"))
                await self.mount(
                    Input(placeholder="Enter your message here...", id="message_input")
                )

                # Create a new client and connect to the server
                self.client: Client = Client("127.0.0.1", 55556, self.nickname)

                # Start server connection after nickname is submitted
                await self.client.connect_to_server(self.client.host, self.client.port)

                # perform handshake
                await self.client.check_handshake()

                # start receive_messages indefinately
                self.receiver_task = self.run_worker(self.client.receive_messages())

                # start process to look at queue
                self.processor_task = self.run_worker(self.process_events())
        # ---/

        # If the input is a message, send it by calling send_message method from client.py
        elif event.input.id == "message_input":
            message = event.value
            await self.client.send_message(message)
            event.input.value = ""
        # ---/

    # Method to get things from the event queue and do something with them depending on what it is.
    async def process_events(self):
        while True:

            # Get the events from the queue and handle them appropriately
            event_type, content = await self.client.event_queue.get()

            self.notify(f"Received event: {event_type}")

            # if the event packet begins with "M", then it is a message from someone. We then call receive_message
            if event_type == "message":
                user = content.get("sender")
                message = content.get("content")

                chat_log = self.query_one("#chat_log")

                await chat_log.mount(Static(f"{user}: {message}"))

            # If the event_type is my_message, display it on the screen
            elif event_type == "my_message":

                # get the chat log info and send the information to it
                chat_log = self.query_one("#chat_log")
                await chat_log.mount(Static(f"Me: {content.get("content")}"))

            # if the event type is the join packet notify the users that someone has connected
            elif event_type == "join_packet":

                # get the chat log info and send the information to it
                chat_log = self.query_one("#chat_log")
                await chat_log.mount(Static(f"{content}"))

            # if the event type is dir notify the users how many people are connected
            elif event_type == "dir":

                # Display the number of connected users
                chat_log = self.query_one("#chat_log")
                await chat_log.mount(Static(f"{content}"))

            # if event type is leave packet notify the users who left the chat
            elif event_type == "leave_packet":

                # get the chat log info and send the information to it
                chat_log = self.query_one("#chat_log")
                await chat_log.mount(Static(f"{content}"))


if __name__ == "__main__":
    app = Beatrice()
    app.run()
