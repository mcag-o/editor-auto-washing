package htmlparser

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

type Document struct {
	root *html.Node
}

func Parse(body []byte) (*Document, error) {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	return &Document{root: root}, nil
}

func (d *Document) FindAll(tag, className string) []*html.Node {
	return findAll(d.root, tag, className)
}

func FindFirst(root *html.Node, tag, className string) *html.Node {
	matches := findAll(root, tag, className)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

func FindFirstNode(nodes []*html.Node) *html.Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

func Attr(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if attr.Key == key {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func Text(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	collectText(&builder, node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func findAll(root *html.Node, tag, className string) []*html.Node {
	if root == nil {
		return nil
	}
	var matches []*html.Node
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && matchesSelector(node, tag, className) {
			matches = append(matches, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return matches
}

func matchesSelector(node *html.Node, tag, className string) bool {
	if tag != "" && node.Data != tag {
		return false
	}
	if className == "" {
		return true
	}
	classes := strings.Fields(Attr(node, "class"))
	for _, class := range classes {
		if class == className {
			return true
		}
	}
	return false
}

func collectText(builder *strings.Builder, node *html.Node) {
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
		builder.WriteString(" ")
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectText(builder, child)
	}
}
