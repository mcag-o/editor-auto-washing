package service

import (
	"content-hub/domain"
	"content-hub/pkg/repo"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"
)

type RSSFeedFetcher interface {
	Fetch(ctx context.Context, feedURL string) ([]byte, error)
}

type rssArticleIntaker interface {
	Intake(ctx context.Context, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error)
	IntakeIntoWorkspace(ctx context.Context, workspaceArticleID string, article domain.IntakeArticle) (*domain.ArticleWorkspaceRecord, error)
}

type RSSPullResult struct {
	Run           *domain.RSSPullRun
	FetchedItems  int
	ImportedItems int
	SkippedItems  int
	FailedItems   int
}

type RSSPullService struct {
	feeds  RSSFeedFetcher
	runs   repo.RSSPullRunRepo
	items  repo.RSSItemRepo
	intake rssArticleIntaker
}

func NewRSSPullService(feeds RSSFeedFetcher, runs repo.RSSPullRunRepo, items repo.RSSItemRepo, intake rssArticleIntaker) *RSSPullService {
	return &RSSPullService{feeds: feeds, runs: runs, items: items, intake: intake}
}

func (s *RSSPullService) RunOnce(ctx context.Context, sub domain.RSSSubscription) (*RSSPullResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := sub.Validate(); err != nil {
		return nil, err
	}

	run := domain.NewRSSPullRun(sub.ID)
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create rss pull run: %w", err)
	}

	result := &RSSPullResult{Run: run}
	body, err := s.feeds.Fetch(ctx, sub.FeedURL)
	if err != nil {
		return result, s.failRun(ctx, run, result, fmt.Errorf("fetch rss feed: %w", err))
	}

	parsedItems, err := parseRSSFeed(body)
	if err != nil {
		return result, s.failRun(ctx, run, result, fmt.Errorf("parse rss feed: %w", err))
	}
	result.FetchedItems = len(parsedItems)

	var itemErrors []string
	for _, parsedItem := range parsedItems {
		itemLabel := firstNonEmpty(strings.TrimSpace(parsedItem.GUID), strings.TrimSpace(parsedItem.Link), strings.TrimSpace(parsedItem.Title), "unknown-item")
		earlyContentHash := hashRSSItem(parsedItem)
		earlyDuplicate, earlyDuplicateErr := s.findRetryableFailedDuplicate(ctx, domain.RSSDuplicateKey{
			SubscriptionID: sub.ID,
			GUID:           strings.TrimSpace(parsedItem.GUID),
			Link:           strings.TrimSpace(parsedItem.Link),
			ContentHash:    earlyContentHash,
		})
		normalized, err := NormalizeRSSItem(sub.ID, sub.TargetType, sub.SourceProfile, sub.RewriteProfileVersion, parsedItem)
		if err != nil {
			effectiveErr := fmt.Errorf("normalize rss item: %w", err)
			if earlyDuplicateErr != nil {
				effectiveErr = fmt.Errorf("failed-row lookup: %w; %w", earlyDuplicateErr, effectiveErr)
			}
			itemErrors = append(itemErrors, fmt.Sprintf("%s: %v", itemLabel, effectiveErr))
			if recordErr := s.recordFailedRSSItem(ctx, run, parsedItem, earlyContentHash, effectiveErr, earlyDuplicate); recordErr != nil {
				return result, s.failRun(ctx, run, result, recordErr)
			}
			result.FailedItems++
			continue
		}
		normalized.PublishedAt = parsedItem.PublishedAt
		earlyLookupWarning := ""
		if earlyDuplicateErr != nil {
			earlyLookupWarning = fmt.Sprintf("failed-row lookup: %v", earlyDuplicateErr)
		}

		rawPayloadJSON, err := json.Marshal(parsedItem)
		if err != nil {
			effectiveErr := fmt.Errorf("marshal rss item payload: %w", err)
			if earlyDuplicateErr != nil {
				effectiveErr = fmt.Errorf("failed-row lookup: %w; %w", earlyDuplicateErr, effectiveErr)
			}
			itemErrors = append(itemErrors, fmt.Sprintf("%s: %v", itemLabel, effectiveErr))
			if recordErr := s.recordFailedRSSItem(ctx, run, parsedItem, earlyContentHash, effectiveErr, earlyDuplicate); recordErr != nil {
				return result, s.failRun(ctx, run, result, recordErr)
			}
			result.FailedItems++
			continue
		}

		contentHash := hashRSSItem(parsedItem)
		duplicateKey := domain.RSSDuplicateKey{
			SubscriptionID: sub.ID,
			GUID:           strings.TrimSpace(parsedItem.GUID),
			Link:           strings.TrimSpace(parsedItem.Link),
			ContentHash:    contentHash,
		}
		retryableDuplicate, err := s.items.FindRetryableDuplicate(ctx, duplicateKey)
		if err != nil {
			itemErrors = append(itemErrors, fmt.Sprintf("%s: find retryable duplicate rss item: %v", itemLabel, err))
			if recordErr := s.recordFailedRSSItem(ctx, run, parsedItem, contentHash, fmt.Errorf("find retryable duplicate rss item: %w", err), earlyDuplicate); recordErr != nil {
				return result, s.failRun(ctx, run, result, recordErr)
			}
			result.FailedItems++
			continue
		}

		duplicate := retryableDuplicate
		if duplicate == nil {
			duplicate, err = s.items.FindDuplicate(ctx, duplicateKey)
		}
		if err != nil {
			itemErrors = append(itemErrors, fmt.Sprintf("%s: find duplicate rss item: %v", itemLabel, err))
			if recordErr := s.recordFailedRSSItem(ctx, run, parsedItem, contentHash, fmt.Errorf("find duplicate rss item: %w", err), earlyDuplicate); recordErr != nil {
				return result, s.failRun(ctx, run, result, recordErr)
			}
			result.FailedItems++
			continue
		}

		item := domain.NewRSSItemRecord(sub.ID, run.ID, parsedItem.GUID, parsedItem.Link, contentHash, parsedItem.Title)
		if duplicate != nil && isRetryableRSSItemStatus(duplicate.Status) {
			item.ID = duplicate.ID
			item.CreatedAt = duplicate.CreatedAt
			item.WorkspaceArticleID = duplicate.WorkspaceArticleID
		}
		item.PublishedAt = parsedItem.PublishedAt
		item.RawPayloadJSON = rawPayloadJSON
		appendRSSItemWarning(item, earlyLookupWarning)

		if duplicate != nil && !isRetryableRSSItemStatus(duplicate.Status) {
			item.Status = domain.RSSItemStatusSkippedDuplicate
			item.Metadata["duplicate_item_id"] = duplicate.ID
			if err := s.items.Create(ctx, item); err != nil {
				return result, s.failRun(ctx, run, result, fmt.Errorf("create duplicate rss item record: %w", err))
			}
			result.SkippedItems++
			continue
		}

		if duplicate != nil && isRetryableRSSItemStatus(duplicate.Status) {
			if err := s.items.Update(ctx, item); err != nil {
				itemErrors = append(itemErrors, fmt.Sprintf("%s: reset failed rss item record: %v", itemLabel, err))
				result.FailedItems++
				continue
			}
		} else {
			if err := s.items.Create(ctx, item); err != nil {
				itemErrors = append(itemErrors, fmt.Sprintf("%s: create rss item record: %v", itemLabel, err))
				result.FailedItems++
				continue
			}
		}

		workspace, err := s.runRSSIntake(ctx, normalized, duplicate)
		if err != nil {
			effectiveErr := fmt.Errorf("intake rss item: %w", err)
			if earlyLookupWarning != "" {
				effectiveErr = fmt.Errorf("%s; %w", earlyLookupWarning, effectiveErr)
			}
			itemErrors = append(itemErrors, fmt.Sprintf("%s: %v", itemLabel, effectiveErr))
			item.Status = domain.RSSItemStatusFailed
			item.UpdatedAt = time.Now().UTC()
			if workspace != nil {
				item.WorkspaceArticleID = strings.TrimSpace(workspace.ID)
			}
			item.Metadata["error"] = effectiveErr.Error()
			if updateErr := s.items.Update(ctx, item); updateErr != nil {
				return result, s.failRun(ctx, run, result, fmt.Errorf("intake rss item: %w: mark item failed: %v", effectiveErr, updateErr))
			}
			result.FailedItems++
			continue
		}

		importedAt := time.Now().UTC()
		item.Status = domain.RSSItemStatusImported
		item.ImportedAt = &importedAt
		if workspace != nil {
			item.WorkspaceArticleID = strings.TrimSpace(workspace.ID)
		}
		item.UpdatedAt = importedAt
		if err := s.items.Update(ctx, item); err != nil {
			item.Status = domain.RSSItemStatusImportDiverged
			item.Metadata["error"] = fmt.Sprintf("mark rss item imported: %v", err)
			item.UpdatedAt = time.Now().UTC()
			if markErr := s.items.Update(ctx, item); markErr != nil {
				return result, s.failRun(ctx, run, result, fmt.Errorf("mark rss item imported: %w: mark divergence: %v", err, markErr))
			}
			return result, s.failRun(ctx, run, result, fmt.Errorf("mark rss item imported: %w", err))
		}
		result.ImportedItems++
	}

	if len(itemErrors) > 0 {
		result.Run.ErrorSummary = strings.Join(itemErrors, "; ")
	}
	if result.ImportedItems == 0 && result.FailedItems > 0 {
		return result, s.failRun(ctx, run, result, errors.New(result.Run.ErrorSummary))
	}

	return result, s.completeRun(ctx, run, result)
}

func (s *RSSPullService) validate() error {
	if s.feeds == nil || s.runs == nil || s.items == nil || s.intake == nil {
		return domain.NewInternalErr("rss pull service is not configured", nil)
	}
	return nil
}

func (s *RSSPullService) completeRun(ctx context.Context, run *domain.RSSPullRun, result *RSSPullResult) error {
	completedAt := time.Now().UTC()
	run.Status = domain.RSSPullRunStatusSucceeded
	run.CompletedAt = &completedAt
	run.Metadata = buildRSSPullRunMetadata(result)
	if err := s.runs.Update(ctx, run); err != nil {
		return fmt.Errorf("update rss pull run: %w", err)
	}
	return nil
}

func (s *RSSPullService) failRun(ctx context.Context, run *domain.RSSPullRun, result *RSSPullResult, runErr error) error {
	completedAt := time.Now().UTC()
	run.Status = domain.RSSPullRunStatusFailed
	run.CompletedAt = &completedAt
	run.ErrorSummary = runErr.Error()
	run.Metadata = buildRSSPullRunMetadata(result)
	if err := s.runs.Update(ctx, run); err != nil {
		return fmt.Errorf("%w: update rss pull run: %v", runErr, err)
	}
	return runErr
}

func buildRSSPullRunMetadata(result *RSSPullResult) map[string]any {
	return map[string]any{
		"fetched_items":  float64(result.FetchedItems),
		"imported_items": float64(result.ImportedItems),
		"skipped_items":  float64(result.SkippedItems),
		"failed_items":   float64(result.FailedItems),
	}
}

func (s *RSSPullService) recordFailedRSSItem(ctx context.Context, run *domain.RSSPullRun, parsedItem RSSFeedItem, contentHash string, itemErr error, duplicate *domain.RSSItemRecord) error {
	title := firstNonEmpty(parsedItem.Title, parsedItem.GUID, parsedItem.Link, "untitled")
	item := domain.NewRSSItemRecord(run.SubscriptionID, run.ID, parsedItem.GUID, parsedItem.Link, contentHash, title)
	if duplicate != nil && isRetryableRSSItemStatus(duplicate.Status) {
		item.ID = duplicate.ID
		item.CreatedAt = duplicate.CreatedAt
		item.WorkspaceArticleID = duplicate.WorkspaceArticleID
	}
	item.Status = domain.RSSItemStatusFailed
	item.PublishedAt = parsedItem.PublishedAt
	item.Metadata["error"] = itemErr.Error()
	item.UpdatedAt = time.Now().UTC()
	if duplicate != nil && isRetryableRSSItemStatus(duplicate.Status) {
		return s.items.Update(ctx, item)
	}
	return s.items.Create(ctx, item)
}

func (s *RSSPullService) findRetryableFailedDuplicate(ctx context.Context, key domain.RSSDuplicateKey) (*domain.RSSItemRecord, error) {
	duplicate, err := s.items.FindDuplicate(ctx, key)
	if err != nil {
		return nil, err
	}
	if duplicate == nil {
		return nil, nil
	}
	if !isRetryableRSSItemStatus(duplicate.Status) {
		return nil, nil
	}
	return duplicate, nil
}

func isRetryableRSSItemStatus(status string) bool {
	return status == domain.RSSItemStatusFailed || status == domain.RSSItemStatusImportDiverged
}

func (s *RSSPullService) runRSSIntake(ctx context.Context, article domain.IntakeArticle, duplicate *domain.RSSItemRecord) (*domain.ArticleWorkspaceRecord, error) {
	if duplicate != nil && isRetryableRSSItemStatus(duplicate.Status) && strings.TrimSpace(duplicate.WorkspaceArticleID) != "" {
		return s.intake.IntakeIntoWorkspace(ctx, duplicate.WorkspaceArticleID, article)
	}
	return s.intake.Intake(ctx, article)
}

func appendRSSItemWarning(item *domain.RSSItemRecord, warning string) {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return
	}
	warnings, _ := item.Metadata["warnings"].([]string)
	item.Metadata["warnings"] = append(warnings, warning)
}

type rssEnvelope struct {
	Channel rssChannel    `xml:"channel"`
	Entries []atomXMLItem `xml:"entry"`
}

type rssChannel struct {
	Items []rssXMLItem `xml:"item"`
}

type rssXMLItem struct {
	GUID        string   `xml:"guid"`
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	Content     string   `xml:"encoded"`
	Author      string   `xml:"author"`
	DCCreator   string   `xml:"creator"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category"`
}

type atomXMLItem struct {
	GUID       string     `xml:"id"`
	Title      string     `xml:"title"`
	Summary    string     `xml:"summary"`
	Content    string     `xml:"content"`
	Published  string     `xml:"published"`
	Updated    string     `xml:"updated"`
	Categories []string   `xml:"category"`
	Links      []rssLink  `xml:"link"`
	Author     *rssAuthor `xml:"author"`
}

type rssLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Text string `xml:",chardata"`
}

type rssAuthor struct {
	Name string `xml:"name"`
	Text string `xml:",chardata"`
}

func parseRSSFeed(body []byte) ([]RSSFeedItem, error) {
	var feed rssEnvelope
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	parsed := make([]RSSFeedItem, 0, len(feed.Channel.Items)+len(feed.Entries))
	for _, item := range feed.Channel.Items {
		feedItem := RSSFeedItem{
			GUID:        strings.TrimSpace(item.GUID),
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			Description: strings.TrimSpace(item.Description),
			Content:     strings.TrimSpace(item.Content),
			Author:      firstNonEmpty(item.Author, item.DCCreator),
			Tags:        trimRSSValues(item.Categories),
		}
		publishedAt, err := parseRSSPublishedAt(item)
		if err != nil {
			return nil, err
		}
		feedItem.PublishedAt = publishedAt
		parsed = append(parsed, feedItem)
	}

	for _, entry := range feed.Entries {
		feedItem := RSSFeedItem{
			GUID:        strings.TrimSpace(entry.GUID),
			Title:       strings.TrimSpace(entry.Title),
			Link:        selectAtomLink(entry.Links),
			Description: strings.TrimSpace(entry.Summary),
			Content:     strings.TrimSpace(entry.Content),
			Author:      selectAtomAuthor(entry.Author),
			Tags:        trimRSSValues(entry.Categories),
		}
		publishedAt, err := parseAtomPublishedAt(entry)
		if err != nil {
			return nil, err
		}
		feedItem.PublishedAt = publishedAt
		parsed = append(parsed, feedItem)
	}

	return parsed, nil
}

func selectAtomLink(links []rssLink) string {
	for _, link := range links {
		if strings.TrimSpace(link.Rel) == "alternate" && strings.TrimSpace(link.Href) != "" {
			return strings.TrimSpace(link.Href)
		}
	}
	for _, link := range links {
		if strings.TrimSpace(link.Href) != "" {
			return strings.TrimSpace(link.Href)
		}
	}
	return ""
}

func selectAtomAuthor(author *rssAuthor) string {
	if author == nil {
		return ""
	}
	return firstNonEmpty(author.Name, author.Text)
}

func parseRSSPublishedAt(item rssXMLItem) (*time.Time, error) {
	for _, raw := range []string{item.PubDate} {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, time.RFC3339, time.RFC3339Nano} {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				parsed = parsed.UTC()
				return &parsed, nil
			}
		}
		return nil, nil
	}
	return nil, nil
}

func parseAtomPublishedAt(item atomXMLItem) (*time.Time, error) {
	for _, raw := range []string{item.Published, item.Updated} {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, time.RFC3339, time.RFC3339Nano} {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				parsed = parsed.UTC()
				return &parsed, nil
			}
		}
		return nil, nil
	}
	return nil, nil
}

func trimRSSValues(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func hashRSSItem(item RSSFeedItem) string {
	checksum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(item.Title),
		strings.TrimSpace(item.Link),
		strings.TrimSpace(item.Description),
		strings.TrimSpace(item.Content),
	}, "\n")))
	return hex.EncodeToString(checksum[:])
}
