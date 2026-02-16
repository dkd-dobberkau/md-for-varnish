"""Tests for the HTML → Markdown conversion pipeline."""

from bs4 import BeautifulSoup

from app.main import extract_metadata, strip_non_content, html_to_markdown, count_tokens


# ---------------------------------------------------------------------------
# extract_metadata
# ---------------------------------------------------------------------------

class TestExtractMetadata:
    def test_extracts_title(self, sample_html):
        soup = BeautifulSoup(sample_html, "html.parser")
        meta = extract_metadata(soup)
        assert meta["title"] == "Testseite – TYPO3 Demo"

    def test_extracts_description(self, sample_html):
        soup = BeautifulSoup(sample_html, "html.parser")
        meta = extract_metadata(soup)
        assert meta["description"] == "Eine Testseite für den Markdown Sidecar"

    def test_extracts_author(self, sample_html):
        soup = BeautifulSoup(sample_html, "html.parser")
        meta = extract_metadata(soup)
        assert meta["author"] == "Test Author"

    def test_extracts_keywords(self, sample_html):
        soup = BeautifulSoup(sample_html, "html.parser")
        meta = extract_metadata(soup)
        assert meta["keywords"] == "test, markdown, sidecar"

    def test_extracts_og_image(self, sample_html):
        soup = BeautifulSoup(sample_html, "html.parser")
        meta = extract_metadata(soup)
        assert meta["image"] == "https://example.com/image.jpg"

    def test_missing_metadata_returns_empty(self):
        soup = BeautifulSoup("<html><body></body></html>", "html.parser")
        meta = extract_metadata(soup)
        assert meta == {}

    def test_partial_metadata(self):
        html = '<html><head><title>Only Title</title></head><body></body></html>'
        soup = BeautifulSoup(html, "html.parser")
        meta = extract_metadata(soup)
        assert meta == {"title": "Only Title"}


# ---------------------------------------------------------------------------
# strip_non_content
# ---------------------------------------------------------------------------

class TestStripNonContent:
    def test_removes_nav(self, sample_html):
        soup = BeautifulSoup(sample_html, "html.parser")
        main = soup.find("main")
        # Add a nav inside main to verify stripping
        soup_with_nav = BeautifulSoup(
            "<main><nav>Gone</nav><p>Stay</p></main>", "html.parser"
        )
        result = strip_non_content(soup_with_nav.find("main"))
        assert result.find("nav") is None
        assert result.find("p") is not None

    def test_removes_script_and_style(self):
        html = "<div><script>alert(1)</script><style>.x{}</style><p>Keep</p></div>"
        soup = BeautifulSoup(html, "html.parser")
        strip_non_content(soup)
        assert soup.find("script") is None
        assert soup.find("style") is None
        assert soup.find("p").text == "Keep"

    def test_removes_cookie_banner(self):
        html = '<div><div class="cookie-banner">Cookies!</div><p>Content</p></div>'
        soup = BeautifulSoup(html, "html.parser")
        strip_non_content(soup)
        assert soup.find(class_="cookie-banner") is None
        assert "Content" in soup.text

    def test_removes_sidebar(self):
        html = "<div><aside>Sidebar</aside><p>Main</p></div>"
        soup = BeautifulSoup(html, "html.parser")
        strip_non_content(soup)
        assert soup.find("aside") is None

    def test_removes_form(self):
        html = "<div><form><input></form><p>Text</p></div>"
        soup = BeautifulSoup(html, "html.parser")
        strip_non_content(soup)
        assert soup.find("form") is None


# ---------------------------------------------------------------------------
# html_to_markdown (integration)
# ---------------------------------------------------------------------------

class TestHtmlToMarkdown:
    def test_produces_front_matter(self, sample_html):
        md_text, meta = html_to_markdown(sample_html)
        assert md_text.startswith("---")
        assert 'title: "Testseite' in md_text
        assert 'description: "Eine Testseite' in md_text

    def test_contains_heading(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "# Willkommen auf der Testseite" in md_text

    def test_contains_link(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "[einem Link](https://example.com)" in md_text

    def test_contains_list_items(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "- Markdown-Konvertierung" in md_text
        assert "- Token-Counting" in md_text

    def test_contains_code_block(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert 'print("Hello, World!")' in md_text

    def test_strips_images(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "photo.jpg" not in md_text
        assert "Testbild" not in md_text

    def test_strips_nav_and_footer(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "Home" not in md_text
        assert "Über uns" not in md_text
        assert "© 2025" not in md_text

    def test_strips_cookie_banner(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "Cookies" not in md_text

    def test_strips_sidebar(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "Sidebar-Inhalt" not in md_text

    def test_no_excessive_blank_lines(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "\n\n\n\n" not in md_text

    def test_minimal_html(self, minimal_html):
        md_text, meta = html_to_markdown(minimal_html)
        assert "Hello World" in md_text
        assert meta == {}
        assert "---" not in md_text  # No front matter without metadata

    def test_content_div_fallback(self, html_no_main):
        md_text, meta = html_to_markdown(html_no_main)
        assert "Fallback Content" in md_text
        assert meta["title"] == "No Main"

    def test_blockquote_preserved(self, sample_html):
        md_text, _ = html_to_markdown(sample_html)
        assert "Ein Zitat zur Demonstration" in md_text


# ---------------------------------------------------------------------------
# count_tokens
# ---------------------------------------------------------------------------

class TestCountTokens:
    def test_returns_positive_int(self):
        count = count_tokens("Hello, this is a test sentence.")
        assert isinstance(count, int)
        assert count > 0

    def test_empty_string(self):
        assert count_tokens("") == 0

    def test_longer_text_more_tokens(self):
        short = count_tokens("Hi")
        long = count_tokens("This is a much longer sentence with many more tokens in it.")
        assert long > short
