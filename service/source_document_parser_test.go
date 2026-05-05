package service

import (
	"archive/zip"
	"bytes"
	"content-hub/domain"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMarkdownExtractsTitleAndBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.md")
	require.NoError(t, os.WriteFile(path, []byte("# Title\n\nBody text"), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.NoError(t, err)
	require.Equal(t, "Title", parsed.Title)
	require.Contains(t, parsed.Body, "Body text")
	require.Empty(t, parsed.Summary)
	require.Empty(t, parsed.Tags)
}

func TestParseSourceDocumentBytesMatchesFilePathForMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.md")
	raw := []byte("# Title\n\nBody text")
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	fromPath, err := ParseSourceDocument(path)
	require.NoError(t, err)

	fromBytes, err := ParseSourceDocumentBytes("article.md", raw)
	require.NoError(t, err)

	require.Equal(t, fromPath.Title, fromBytes.Title)
	require.Equal(t, fromPath.Body, fromBytes.Body)
	require.Equal(t, fromPath.Summary, fromBytes.Summary)
	require.Equal(t, fromPath.Tags, fromBytes.Tags)
}

func TestParseTextUsesFilenameAsFallbackTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("Plain text body"), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.NoError(t, err)
	require.Equal(t, "notes", parsed.Title)
	require.Equal(t, "Plain text body", parsed.Body)
}

func TestParseJSONExtractsStructuredFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"title":"Title","content":"Body","summary":"Summary","tags":["a","b"]}`), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.NoError(t, err)
	require.Equal(t, "Title", parsed.Title)
	require.Equal(t, "Body", parsed.Body)
	require.Equal(t, "Summary", parsed.Summary)
	require.Equal(t, []string{"a", "b"}, parsed.Tags)
}

func TestParseJSONMissingContentFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"title":"Title"}`), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.Nil(t, parsed)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Code)
	require.ErrorContains(t, err, "source document content is required")
}

func TestParseJSONWhitespaceOnlyContentFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"title":"Title","content":"   \n\t  "}`), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.Nil(t, parsed)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Code)
	require.ErrorContains(t, err, "source document content is required")
}

func TestParseJSONMissingTitleFallsBackToFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"content":"Body"}`), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.NoError(t, err)
	require.Equal(t, "article", parsed.Title)
	require.Equal(t, "Body", parsed.Body)
}

func TestParseJSONWhitespaceOnlyTitleFallsBackToFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"title":"   \n\t  ","content":"Body"}`), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.NoError(t, err)
	require.Equal(t, "article", parsed.Title)
	require.Equal(t, "Body", parsed.Body)
}

func TestParseSourceDocumentBytesMatchesFilePathForTitlelessJSONFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.json")
	raw := []byte(`{"content":"Body"}`)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	fromPath, err := ParseSourceDocument(path)
	require.NoError(t, err)

	fromBytes, err := ParseSourceDocumentBytes("article.json", raw)
	require.NoError(t, err)

	require.Equal(t, fromPath.Title, fromBytes.Title)
	require.Equal(t, fromPath.Body, fromBytes.Body)
	require.Equal(t, fromPath.Summary, fromBytes.Summary)
	require.Equal(t, fromPath.Tags, fromBytes.Tags)
}

func TestParseDOCXExtractsPlainText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.docx")
	require.NoError(t, os.WriteFile(path, buildDOCXFixture(t, []string{"Docx Title", "Docx body text"}), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.NoError(t, err)
	require.Equal(t, "report", parsed.Title)
	require.Contains(t, parsed.Body, "Docx Title")
	require.Contains(t, parsed.Body, "Docx body text")
}

func TestParseSourceDocumentReturnsValidationErrorForUnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "article.pdf")
	require.NoError(t, os.WriteFile(path, []byte("not supported"), 0o644))

	parsed, err := ParseSourceDocument(path)

	require.Nil(t, parsed)
	appErr, ok := err.(*domain.AppError)
	require.True(t, ok)
	require.Equal(t, domain.ErrValidation, appErr.Code)
	require.ErrorContains(t, err, "unsupported source document type")
}

func buildDOCXFixture(t *testing.T, paragraphs []string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	writeZipFile := func(name, contents string) {
		fileWriter, err := writer.Create(name)
		require.NoError(t, err)
		_, err = fileWriter.Write([]byte(contents))
		require.NoError(t, err)
	}

	writeZipFile("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`)
	writeZipFile("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`)

	documentXML := `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`
	for _, paragraph := range paragraphs {
		documentXML += `<w:p><w:r><w:t>` + paragraph + `</w:t></w:r></w:p>`
	}
	documentXML += `</w:body></w:document>`
	writeZipFile("word/document.xml", documentXML)

	require.NoError(t, writer.Close())
	return buf.Bytes()
}
