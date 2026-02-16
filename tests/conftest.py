from pathlib import Path

import pytest

FIXTURES_DIR = Path(__file__).parent / "fixtures"


@pytest.fixture
def sample_html() -> str:
    return (FIXTURES_DIR / "page.html").read_text()


@pytest.fixture
def minimal_html() -> str:
    return "<html><body><p>Hello World</p></body></html>"


@pytest.fixture
def html_no_main() -> str:
    return """
    <html>
    <head><title>No Main</title></head>
    <body>
        <div id="content">
            <h1>Fallback Content</h1>
            <p>Found via #content div.</p>
        </div>
    </body>
    </html>
    """
