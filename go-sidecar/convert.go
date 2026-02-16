package main

import (
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	tiktoken "github.com/pkoukk/tiktoken-go"
)

// Metadata holds page metadata extracted from <head>.
type Metadata struct {
	Title       string
	Description string
	Author      string
	Keywords    string
	Image       string
}

// stripSelectors are CSS selectors for non-content elements to remove.
var stripSelectors = []string{
	"nav", "header", "footer",
	"script", "style", "noscript",
	"iframe", "object", "embed",
	".cookie-banner", ".cookie-consent",
	"#cookie-banner", "#cookie-consent",
	"[role='navigation']", "[role='banner']", "[role='contentinfo']",
	".breadcrumb", ".pagination",
	".sidebar", "aside",
	"form",
}

// extractMetadata pulls title, description, author, keywords, og:image from <head>.
func extractMetadata(doc *goquery.Document) Metadata {
	var meta Metadata

	if title := doc.Find("title").First().Text(); strings.TrimSpace(title) != "" {
		meta.Title = strings.TrimSpace(title)
	}

	for _, name := range []string{"description", "author", "keywords"} {
		doc.Find(fmt.Sprintf(`meta[name="%s"]`, name)).Each(func(_ int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists && strings.TrimSpace(content) != "" {
				switch name {
				case "description":
					meta.Description = strings.TrimSpace(content)
				case "author":
					meta.Author = strings.TrimSpace(content)
				case "keywords":
					meta.Keywords = strings.TrimSpace(content)
				}
			}
		})
	}

	doc.Find(`meta[property="og:image"]`).Each(func(_ int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists && strings.TrimSpace(content) != "" {
			meta.Image = strings.TrimSpace(content)
		}
	})

	return meta
}

// findContentRoot returns the best content root selection: <main> → <article> → #content → .content → <body>.
func findContentRoot(doc *goquery.Document) *goquery.Selection {
	for _, selector := range []string{"main", "article", "#content", ".content"} {
		sel := doc.Find(selector).First()
		if sel.Length() > 0 {
			return sel
		}
	}
	return doc.Find("body").First()
}

// stripNonContent removes navigation, chrome, scripts and other non-content elements.
func stripNonContent(sel *goquery.Selection, extra []string) {
	allSelectors := make([]string, 0, len(stripSelectors)+len(extra))
	allSelectors = append(allSelectors, stripSelectors...)
	allSelectors = append(allSelectors, extra...)

	for _, s := range allSelectors {
		sel.Find(s).Remove()
	}
}

// removeImages strips all <img> tags from the selection.
func removeImages(sel *goquery.Selection) {
	sel.Find("img").Remove()
}

// htmlToMarkdown converts an HTML document to clean Markdown with YAML front matter.
func htmlToMarkdown(html string, extraSelectors []string) (string, Metadata) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", Metadata{}
	}

	// 1. Extract metadata before stripping
	meta := extractMetadata(doc)

	// 2. Isolate content root
	contentRoot := findContentRoot(doc)

	// 3. Strip non-content elements
	stripNonContent(contentRoot, extraSelectors)

	// 4. Remove images (useless for agents)
	removeImages(contentRoot)

	// 5. Get inner HTML of content root
	contentHTML, err := contentRoot.Html()
	if err != nil {
		return "", meta
	}

	// 6. Convert to Markdown
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(
				commonmark.WithHeadingStyle("atx"),
				commonmark.WithBulletListMarker("-"),
			),
			table.NewTablePlugin(),
		),
	)

	mdBody, err := conv.ConvertString(contentHTML)
	if err != nil {
		return "", meta
	}

	// 7. Clean up excessive blank lines (max 2 consecutive)
	mdBody = cleanBlankLines(mdBody)
	mdBody = strings.TrimSpace(mdBody)

	// 8. Build front matter
	if fm := buildFrontMatter(meta); fm != "" {
		mdBody = fm + "\n\n" + mdBody
	}

	return mdBody, meta
}

// cleanBlankLines collapses runs of >2 blank lines down to 2.
func cleanBlankLines(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount <= 2 {
				cleaned = append(cleaned, "")
			}
		} else {
			blankCount = 0
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

// buildFrontMatter creates YAML front matter from metadata.
func buildFrontMatter(meta Metadata) string {
	var lines []string

	if meta.Title != "" {
		lines = append(lines, fmt.Sprintf(`title: "%s"`, escapeYAML(meta.Title)))
	}
	if meta.Description != "" {
		lines = append(lines, fmt.Sprintf(`description: "%s"`, escapeYAML(meta.Description)))
	}
	if meta.Author != "" {
		lines = append(lines, fmt.Sprintf(`author: "%s"`, escapeYAML(meta.Author)))
	}
	if meta.Keywords != "" {
		lines = append(lines, fmt.Sprintf(`keywords: "%s"`, escapeYAML(meta.Keywords)))
	}
	if meta.Image != "" {
		lines = append(lines, fmt.Sprintf(`image: "%s"`, escapeYAML(meta.Image)))
	}

	if len(lines) == 0 {
		return ""
	}

	return "---\n" + strings.Join(lines, "\n") + "\n---"
}

func escapeYAML(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// countTokens estimates token count using tiktoken cl100k_base.
func countTokens(text string, model string) int {
	enc, err := tiktoken.GetEncoding(model)
	if err != nil {
		// Fallback: ~4 chars per token
		return len(text) / 4
	}
	return len(enc.Encode(text, nil, nil))
}
