package blog

import (
	"bytes"
	"context"
	"fmt"
	"oliverbutler/lib/tracing"
	"oliverbutler/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"

	chromaHtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/gomarkdown/markdown"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v2"
)

type BlogService struct{}

func NewBlogService() *BlogService {
	return &BlogService{}
}

func (s *BlogService) GetAllPosts(ctx context.Context) ([]Post, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "BlogService.GetAllPosts")
	defer span.End()

	blogDir := "./static/blog"
	posts := []Post{}

	dirs, err := os.ReadDir(blogDir)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			post, err := readPost(ctx, filepath.Join(blogDir, dir.Name()))
			if err != nil {
				return nil, err
			}
			post.Slug = dir.Name()
			posts = append(posts, post)
		}
	}

	// Sort posts by date, latest first
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PubDate.After(posts[j].PubDate)
	})

	return posts, nil
}

func (s *BlogService) GetPost(ctx context.Context, slug string) (Post, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "BlogService.GetPost")
	defer span.End()

	postDir := filepath.Join("./static/blog", slug)
	return readPost(ctx, postDir)
}

func (s *BlogService) GetChromaCSS() string {
	return getChromaCSS()
}

type Post struct {
	Title       string        `yaml:"title"`
	Description string        `yaml:"description"`
	PubDate     utils.IsoDate `yaml:"pubDate"`
	HeroImage   string        `yaml:"heroImage"`
	Slug        string
	Content     string
}

func readPost(ctx context.Context, postDir string) (Post, error) {
	ctx, span := tracing.OmoTracer.Start(ctx, "readPost", trace.WithAttributes(attribute.String("postDir", postDir)))
	defer span.End()

	mdFile := filepath.Join(postDir, "post.md")
	content, err := os.ReadFile(mdFile)
	if err != nil {
		return Post{}, err
	}

	span.AddEvent("Read file content")

	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) != 3 {
		return Post{}, fmt.Errorf("invalid post format")
	}

	var post Post
	err = yaml.Unmarshal([]byte(parts[1]), &post)
	if err != nil {
		return Post{}, err
	}

	// Convert markdown to HTML
	post.Content = string(markdown.ToHTML([]byte(parts[2]), nil, nil))

	span.AddEvent("Convert markdown to HTML")

	// Apply syntax highlighting to code blocks
	post.Content, err = highlightCodeBlocks(post.Content)
	if err != nil {
		return Post{}, err
	}

	span.AddEvent("Highlight code blocks")

	// Handle relative paths for heroImage
	post.HeroImage = "/" + postDir + "/" + post.HeroImage[2:]

	return post, nil
}

func highlightCodeBlocks(content string) (string, error) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	var highlightNode func(*html.Node)
	highlightNode = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "code" {
			// Find the parent <pre> tag
			if n.Parent != nil && n.Parent.Type == html.ElementNode && n.Parent.Data == "pre" {
				// Get the language from the class attribute
				var lang string
				for _, attr := range n.Attr {
					if attr.Key == "class" && strings.HasPrefix(attr.Val, "language-") {
						lang = strings.TrimPrefix(attr.Val, "language-")
						break
					}
				}

				// If no language is specified, default to "go"
				if lang == "" {
					lang = "go"
				}

				// Get the code content
				code := extractText(n)

				// Highlight the code
				highlightedCode, err := highlightCode(code, lang)
				if err == nil {
					// Replace the content of the <code> tag with the highlighted code
					n.FirstChild = &html.Node{
						Type: html.RawNode,
						Data: highlightedCode,
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			highlightNode(c)
		}
	}

	highlightNode(doc)

	var buf bytes.Buffer
	err = html.Render(&buf, doc)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func extractText(n *html.Node) string {
	var text string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			text += c.Data
		}
	}
	return text
}

var styleName = "dracula"

func getChromaCSS() string {
	var buf bytes.Buffer
	formatter := chromaHtml.New(chromaHtml.WithClasses(true))
	err := formatter.WriteCSS(&buf, styles.Get(styleName))
	if err != nil {
		return ""
	}
	return buf.String()
}

func highlightCode(code, lang string) (string, error) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}
	formatter := chromaHtml.New(
		chromaHtml.WithClasses(true),
		chromaHtml.TabWidth(4),
		chromaHtml.LineNumbersInTable(true),
	)
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
