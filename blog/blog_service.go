package blog

import (
	"context"
	"fmt"
	"oliverbutler/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gomarkdown/markdown"
	"gopkg.in/yaml.v2"
)

type BlogService struct{}

func NewBlogService() *BlogService {
	return &BlogService{}
}

func (s *BlogService) GetAllPosts(ctx context.Context) ([]Post, error) {
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

func (s *BlogService) GetPost(ctx context.Context, slug string) (Post, error) {
	postDir := filepath.Join("./static/blog", slug)
	return readPost(postDir)
}

type Post struct {
	Title       string        `yaml:"title"`
	Description string        `yaml:"description"`
	PubDate     utils.IsoDate `yaml:"pubDate"`
	HeroImage   string        `yaml:"heroImage"`
	Slug        string
	Content     string
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
	post.HeroImage = "/" + postDir + "/" + post.HeroImage[2:]

	return post, nil
}
