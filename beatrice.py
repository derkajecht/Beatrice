from textual.app import App, ComposeResult
from textual.widgets import Header, Static, Input, Button, Footer
from client import send_messages, receive_messages, HOST, PORT
from server import broadcast, handle_client, receive_connections

class Beatrice(App):
    """A TUI messenger named Beatrice."""
    CSS_PATH = "styling.css"


    def compose(self) -> ComposeResult:
        yield Header(icon="💬")

        yield Static(id="message_log")

        yield Input(placeholder="Enter your message...", id="message_input")

        yield Button("Send", id="send_button")

        yield Footer()


    def on_mount(self) -> None:
        self.message_log = self.query_one("#message_log", Static)
        self.message_input = self.query_one("#message_input", Input)
        self.send_button = self.query_one("#send_button", Button)

        self.title = "BEATRICE"

if __name__ == "__main__":
    app = Beatrice()
    app.run()
