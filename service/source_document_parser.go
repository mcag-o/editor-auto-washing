package service

import (
	"archive/zip"
	"bytes"
	"content-hub/domain"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ParsedIntakeDocument struct {
	Title   string
	Body    string
	Summary string
	Tags    []string
}

func ParseIntakeDocument(path string) (*ParsedIntakeDocument, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, domain.NewValidationErr("input document path is required", nil)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md":
			return nil, fmt.Errorf("read markdown document: %w", err)
		case ".txt":
			return nil, fmt.Errorf("read text document: %w", err)
		case ".json":
			return nil, fmt.Errorf("read json document: %w", err)
		case ".docx":
			return nil, fmt.Errorf("read docx document: %w", err)
		default:
			return nil, domain.NewValidationErr("unsupported input document type", nil)
		}
	}

	return ParseIntakeDocumentBytes(path, body)
}

func ParseIntakeDocumentBytes(filename string, raw []byte) (*ParsedIntakeDocument, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, domain.NewValidationErr("input document filename is required", nil)
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md":
		return parseMarkdownDocumentBytes(filename, raw)
	case ".txt":
		return parseTextDocumentBytes(filename, raw)
	case ".json":
		return parseJSONDocumentBytes(filename, raw)
	case ".docx":
		return parseDOCXDocumentBytes(filename, raw)
	default:
		return nil, domain.NewValidationErr("unsupported input document type", nil)
	}
}

func intakeDocumentMetadata(parsed *ParsedIntakeDocument) map[string]any {
	metadata := map[string]any{}
	if parsed == nil {
		return metadata
	}
	if len(parsed.Tags) > 0 {
		metadata["tags"] = append([]string(nil), parsed.Tags...)
	}
	return metadata
}

func parseMarkdownDocumentBytes(filename string, body []byte) (*ParsedIntakeDocument, error) {
	text := string(body)
	return &ParsedIntakeDocument{
		Title: inferMarkdownTitle(text, filename),
		Body:  text,
	}, nil
}

func parseTextDocumentBytes(filename string, body []byte) (*ParsedIntakeDocument, error) {
	text := string(body)
	return &ParsedIntakeDocument{
		Title: fallbackTitle(filename),
		Body:  text,
	}, nil
}

func parseJSONDocumentBytes(filename string, body []byte) (*ParsedIntakeDocument, error) {
	var payload struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Summary string   `json:"summary"`
		Tags    []string `json:"tags"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, domain.NewValidationErr("decode input document json", err)
	}

	if strings.TrimSpace(payload.Content) == "" {
		return nil, domain.NewValidationErr("input document content is required", nil)
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = fallbackTitle(filename)
	}

	return &ParsedIntakeDocument{
		Title:   title,
		Body:    payload.Content,
		Summary: strings.TrimSpace(payload.Summary),
		Tags:    normalizeTags(payload.Tags),
	}, nil
}

func parseDOCXDocumentBytes(filename string, body []byte) (*ParsedIntakeDocument, error) {
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, domain.NewValidationErr("open docx archive", err)
	}

	text, err := extractDOCXPlainText(reader)
	if err != nil {
		return nil, err
	}

	return &ParsedIntakeDocument{
		Title: fallbackTitle(filename),
		Body:  text,
	}, nil
}

func inferMarkdownTitle(body, path string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			if title != "" {
				return title
			}
		}
	}
	return fallbackTitle(path)
}

func fallbackTitle(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func extractDOCXPlainText(reader *zip.Reader) (string, error) {
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open docx document xml: %w", err)
		}

		text, extractErr := extractDOCXTextFromXML(rc)
		closeErr := rc.Close()
		if extractErr != nil {
			return "", extractErr
		}
		if closeErr != nil {
			return "", fmt.Errorf("close docx document xml: %w", closeErr)
		}
		return text, nil
	}

	return "", domain.NewValidationErr("docx document.xml is missing", nil)
}

func extractDOCXTextFromXML(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var parts []string
	var inText bool

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", domain.NewValidationErr("decode docx document xml", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "t" {
				inText = true
			}
		case xml.EndElement:
			if value.Name.Local == "p" {
				parts = append(parts, "\n")
			}
			if value.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				parts = append(parts, string(value))
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "")), nil
}
