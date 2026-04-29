package chatutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	webUserAgent     = "GPTerminal (github.com/cycl0o0/GPTerminal)"
	webTimeout       = 15 * time.Second
	maxResponseBytes = 1 << 20 // 1 MB
	maxSearchResults = 10
)

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func webSearch(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("search query cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, webTimeout)
	defer cancel()

	form := url.Values{"q": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build search request: %w", err)
	}
	req.Header.Set("User-Agent", webUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	body := io.LimitReader(resp.Body, maxResponseBytes)
	results, err := parseDuckDuckGoHTML(body)
	if err != nil {
		return "", fmt.Errorf("parse search results: %w", err)
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Web search results for: %s\n\n", query))
	for i, r := range results {
		if i >= maxSearchResults {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet))
	}
	return strings.TrimSpace(b.String()), nil
}

func parseDuckDuckGoHTML(r io.Reader) ([]searchResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var results []searchResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			cls := attrVal(n, "class")
			if strings.Contains(cls, "result__a") {
				href := attrVal(n, "href")
				title := extractText(n)
				if parsed, err := url.Parse(href); err == nil {
					if u := parsed.Query().Get("uddg"); u != "" {
						href = u
					}
				}
				results = append(results, searchResult{
					Title: strings.TrimSpace(title),
					URL:   href,
				})
			}
			if strings.Contains(cls, "result__snippet") && len(results) > 0 {
				results[len(results)-1].Snippet = strings.TrimSpace(extractText(n))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results, nil
}

func fetchURL(ctx context.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme == "" {
		rawURL = "https://" + rawURL
	} else if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("only http and https URLs are supported")
	}

	ctx, cancel := context.WithTimeout(ctx, webTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", webUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}

	body := io.LimitReader(resp.Body, maxResponseBytes)

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/plain") || strings.Contains(ct, "application/json") {
		data, err := io.ReadAll(body)
		if err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}
		return fmt.Sprintf("URL: %s\n\n%s", rawURL, string(data)), nil
	}

	text, err := htmlToText(body)
	if err != nil {
		return "", fmt.Errorf("extract text: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Sprintf("URL: %s\n\n(no readable text content)", rawURL), nil
	}
	return fmt.Sprintf("URL: %s\n\n%s", rawURL, text), nil
}

func htmlToText(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	skipTags := map[atom.Atom]bool{
		atom.Script:   true,
		atom.Style:    true,
		atom.Nav:      true,
		atom.Header:   true,
		atom.Footer:   true,
		atom.Noscript: true,
		atom.Svg:      true,
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.DataAtom] {
			return
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				b.WriteString(text)
				b.WriteString(" ")
			}
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.P, atom.Div, atom.Br, atom.Li, atom.H1, atom.H2,
				atom.H3, atom.H4, atom.H5, atom.H6, atom.Tr, atom.Blockquote,
				atom.Pre, atom.Section, atom.Article:
				b.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.P, atom.Div, atom.Li, atom.H1, atom.H2,
				atom.H3, atom.H4, atom.H5, atom.H6, atom.Tr,
				atom.Blockquote, atom.Section, atom.Article:
				b.WriteString("\n")
			}
		}
	}
	walk(doc)

	text := b.String()
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return text, nil
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
	}
	return sb.String()
}

func attrVal(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
