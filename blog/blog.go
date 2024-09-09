package blog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"gopkg.in/yaml.v2"
)

type Post struct {
	Title       string  `yaml:"title"`
	Description string  `yaml:"description"`
	PubDate     IsoDate `yaml:"pubDate"`
	HeroImage   string  `yaml:"heroImage"`
	Slug        string
	Content     string
}

type IsoDate string

func (d IsoDate) String() string {
	return string(d)
}

func (d IsoDate) After(other IsoDate) bool {
	return d > other
}

func (d IsoDate) FormattedString() string {
	t, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return string(d) // Return original string if parsing fails
	}
	return t.Format("January 2, 2006")
}

func GetAllPosts() ([]Post, error) {
	blogDir := "./static/blog"
	posts := []Post{}

	dirs, err := os.ReadDir(blogDir)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			post, err := readPost(filepath.Join(blogDir, dir.Name()))
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

func GetPost(slug string) (Post, error) {
	postDir := filepath.Join("./static/blog", slug)
	return readPost(postDir)
}

func readPost(postDir string) (Post, error) {
	mdFile := filepath.Join(postDir, "post.md")
	content, err := os.ReadFile(mdFile)
	if err != nil {
		return Post{}, err
	}

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

	// Handle relative paths for heroImage
	if strings.HasPrefix(post.HeroImage, "./") {
		post.HeroImage = filepath.Join(filepath.Base(postDir), post.HeroImage[2:])
	}

	return post, nil
}
