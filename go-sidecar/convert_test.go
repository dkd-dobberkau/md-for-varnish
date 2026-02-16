package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// loadFixture reads the TYPO3-like test page from tests/fixtures/page.html.
func loadFixture(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	fixturePath := filepath.Join(filepath.Dir(thisFile), "..", "tests", "fixtures", "page.html")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// extractMetadata
// ---------------------------------------------------------------------------

func TestExtractMetadata_Title(t *testing.T) {
	html := loadFixture(t)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	meta := extractMetadata(doc)
	if meta.Title != "Testseite – TYPO3 Demo" {
		t.Errorf("expected title 'Testseite – TYPO3 Demo', got %q", meta.Title)
	}
}

func TestExtractMetadata_Description(t *testing.T) {
	html := loadFixture(t)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	meta := extractMetadata(doc)
	if meta.Description != "Eine Testseite für den Markdown Sidecar" {
		t.Errorf("expected description, got %q", meta.Description)
	}
}

func TestExtractMetadata_Author(t *testing.T) {
	html := loadFixture(t)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	meta := extractMetadata(doc)
	if meta.Author != "Test Author" {
		t.Errorf("expected author 'Test Author', got %q", meta.Author)
	}
}

func TestExtractMetadata_Keywords(t *testing.T) {
	html := loadFixture(t)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	meta := extractMetadata(doc)
	if meta.Keywords != "test, markdown, sidecar" {
		t.Errorf("expected keywords, got %q", meta.Keywords)
	}
}

func TestExtractMetadata_OGImage(t *testing.T) {
	html := loadFixture(t)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	meta := extractMetadata(doc)
	if meta.Image != "https://example.com/image.jpg" {
		t.Errorf("expected og:image, got %q", meta.Image)
	}
}

func TestExtractMetadata_Empty(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body></body></html>"))
	meta := extractMetadata(doc)
	if meta.Title != "" || meta.Description != "" || meta.Author != "" || meta.Keywords != "" || meta.Image != "" {
		t.Errorf("expected empty metadata, got %+v", meta)
	}
}

func TestExtractMetadata_Partial(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(
		`<html><head><title>Only Title</title></head><body></body></html>`,
	))
	meta := extractMetadata(doc)
	if meta.Title != "Only Title" {
		t.Errorf("expected 'Only Title', got %q", meta.Title)
	}
	if meta.Description != "" || meta.Author != "" {
		t.Errorf("expected empty description/author, got %+v", meta)
	}
}

// ---------------------------------------------------------------------------
// stripNonContent
// ---------------------------------------------------------------------------

func TestStripNonContent_RemovesNav(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(
		"<main><nav>Gone</nav><p>Stay</p></main>",
	))
	sel := doc.Find("main")
	stripNonContent(sel, nil)
	if sel.Find("nav").Length() != 0 {
		t.Error("nav should have been removed")
	}
	if sel.Find("p").Length() == 0 {
		t.Error("p should remain")
	}
}

func TestStripNonContent_RemovesScriptAndStyle(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(
		`<div><script>alert(1)</script><style>.x{}</style><p>Keep</p></div>`,
	))
	stripNonContent(doc.Selection, nil)
	if doc.Find("script").Length() != 0 {
		t.Error("script should have been removed")
	}
	if doc.Find("style").Length() != 0 {
		t.Error("style should have been removed")
	}
	if !strings.Contains(doc.Text(), "Keep") {
		t.Error("p text should remain")
	}
}

func TestStripNonContent_RemovesCookieBanner(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(
		`<div><div class="cookie-banner">Cookies!</div><p>Content</p></div>`,
	))
	stripNonContent(doc.Selection, nil)
	if doc.Find(".cookie-banner").Length() != 0 {
		t.Error("cookie-banner should have been removed")
	}
	if !strings.Contains(doc.Text(), "Content") {
		t.Error("content text should remain")
	}
}

func TestStripNonContent_RemovesSidebar(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(
		"<div><aside>Sidebar</aside><p>Main</p></div>",
	))
	stripNonContent(doc.Selection, nil)
	if doc.Find("aside").Length() != 0 {
		t.Error("aside should have been removed")
	}
}

func TestStripNonContent_RemovesForm(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(
		"<div><form><input></form><p>Text</p></div>",
	))
	stripNonContent(doc.Selection, nil)
	if doc.Find("form").Length() != 0 {
		t.Error("form should have been removed")
	}
}

// ---------------------------------------------------------------------------
// htmlToMarkdown (integration)
// ---------------------------------------------------------------------------

func TestHtmlToMarkdown_FrontMatter(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if !strings.HasPrefix(md, "---") {
		t.Error("expected front matter at start")
	}
	if !strings.Contains(md, `title: "Testseite`) {
		t.Error("expected title in front matter")
	}
	if !strings.Contains(md, `description: "Eine Testseite`) {
		t.Error("expected description in front matter")
	}
}

func TestHtmlToMarkdown_ContainsHeading(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if !strings.Contains(md, "# Willkommen auf der Testseite") {
		t.Errorf("expected heading, got:\n%s", md)
	}
}

func TestHtmlToMarkdown_ContainsLink(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if !strings.Contains(md, "[einem Link](https://example.com)") {
		t.Errorf("expected link, got:\n%s", md)
	}
}

func TestHtmlToMarkdown_ContainsListItems(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if !strings.Contains(md, "- Markdown-Konvertierung") {
		t.Errorf("expected list item, got:\n%s", md)
	}
	if !strings.Contains(md, "- Token-Counting") {
		t.Errorf("expected list item, got:\n%s", md)
	}
}

func TestHtmlToMarkdown_ContainsCodeBlock(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if !strings.Contains(md, `print("Hello, World!")`) {
		t.Errorf("expected code block, got:\n%s", md)
	}
}

func TestHtmlToMarkdown_StripsImages(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if strings.Contains(md, "photo.jpg") {
		t.Error("images should have been stripped")
	}
	if strings.Contains(md, "Testbild") {
		t.Error("image alt text should have been stripped")
	}
}

func TestHtmlToMarkdown_StripsNavAndFooter(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if strings.Contains(md, "Home") {
		t.Error("nav links should have been stripped")
	}
	if strings.Contains(md, "Über uns") {
		t.Error("nav links should have been stripped")
	}
	if strings.Contains(md, "© 2025") || strings.Contains(md, "\u00a9 2025") {
		t.Error("footer should have been stripped")
	}
}

func TestHtmlToMarkdown_StripsCookieBanner(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if strings.Contains(md, "Cookies") {
		t.Error("cookie banner should have been stripped")
	}
}

func TestHtmlToMarkdown_StripsSidebar(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if strings.Contains(md, "Sidebar-Inhalt") {
		t.Error("sidebar should have been stripped")
	}
}

func TestHtmlToMarkdown_NoExcessiveBlankLines(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if strings.Contains(md, "\n\n\n\n") {
		t.Error("should not have 4+ consecutive blank lines")
	}
}

func TestHtmlToMarkdown_Minimal(t *testing.T) {
	md, meta := htmlToMarkdown("<html><body><p>Hello World</p></body></html>", nil)
	if !strings.Contains(md, "Hello World") {
		t.Error("expected 'Hello World' in output")
	}
	if meta.Title != "" {
		t.Error("expected empty metadata")
	}
	if strings.Contains(md, "---") {
		t.Error("no front matter without metadata")
	}
}

func TestHtmlToMarkdown_ContentDivFallback(t *testing.T) {
	html := `
	<html>
	<head><title>No Main</title></head>
	<body>
		<div id="content">
			<h1>Fallback Content</h1>
			<p>Found via #content div.</p>
		</div>
	</body>
	</html>`
	md, meta := htmlToMarkdown(html, nil)
	if !strings.Contains(md, "Fallback Content") {
		t.Errorf("expected fallback content, got:\n%s", md)
	}
	if meta.Title != "No Main" {
		t.Errorf("expected title 'No Main', got %q", meta.Title)
	}
}

func TestHtmlToMarkdown_BlockquotePreserved(t *testing.T) {
	html := loadFixture(t)
	md, _ := htmlToMarkdown(html, nil)
	if !strings.Contains(md, "Ein Zitat zur Demonstration") {
		t.Error("blockquote content should be preserved")
	}
}

// ---------------------------------------------------------------------------
// countTokens
// ---------------------------------------------------------------------------

func TestCountTokens_ReturnsPositiveInt(t *testing.T) {
	count := countTokens("Hello, this is a test sentence.", "cl100k_base")
	if count <= 0 {
		t.Errorf("expected positive token count, got %d", count)
	}
}

func TestCountTokens_EmptyString(t *testing.T) {
	count := countTokens("", "cl100k_base")
	if count != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", count)
	}
}

func TestCountTokens_LongerTextMoreTokens(t *testing.T) {
	short := countTokens("Hi", "cl100k_base")
	long := countTokens("This is a much longer sentence with many more tokens in it.", "cl100k_base")
	if long <= short {
		t.Errorf("expected longer text to have more tokens: short=%d, long=%d", short, long)
	}
}
