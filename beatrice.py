from textual import work
from textual.app import App, ComposeResult
from textual.containers import VerticalScroll
from textual.widgets import Header, Static, Input, Button, Footer
from client import send_messages, receive_messages
# from server import broadcast, handle_client, receive_connections

class Beatrice(App):
    # A TUI messenger named Beatrice
    CSS_PATH = "styling.css"

    def compose(self) -> ComposeResult:
        yield Header(icon="💬")

        with VerticalScroll(id="message_container"):
            yield Static(id="message_log")

        yield Input(placeholder="Enter your message...", id="message_input")

        yield Button("Send", id="send_button")

        yield Footer()


    def on_mount(self) -> None:
        self.title = "BEATRICE"
        self.message_log = self.query_one("#message_log")
        self.message_input = self.query_one("#message_input")
        self.send_button = self.query_one("#send_button")

        self.run_worker(receive_messages, thread=True) # runs imported function in a thread, indefinitely to listen for messages

    def on_input_submitted(self, event: Input.Submitted) -> None:
        message = event.value
        self.message_input.value = ""
        self.run_worker(send_messages, message, thread=True) # runs imported function in a thread to send message


if __name__ == "__main__":
    app = Beatrice()
    app.run()
