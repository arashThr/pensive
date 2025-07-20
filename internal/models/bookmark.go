package models

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/internal/validations"
	"github.com/go-shiori/go-readability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/genai"
)

type BookmarkSource = int

const PageSize = 5

const (
	WebSource BookmarkSource = iota
	TelegramSource
	Api
	Pocket
)

var sourceMapping = map[BookmarkSource]string{
	WebSource:      "web",
	TelegramSource: "telegram",
	Api:            "api",
	Pocket:         "pocket",
}

type Bookmark struct {
	BookmarkId    types.BookmarkId
	UserId        types.UserId
	Title         string
	Link          string
	Source        string
	Excerpt       string
	ImageUrl      string
	ArticleLang   string
	SiteName      string
	AISummary     *string
	AIExcerpt     *string
	AITags        *string
	CreatedAt     time.Time
	PublishedTime *time.Time
}

type BookmarkWithContent struct {
	Bookmark
	Content string // Full content of the bookmark
}

type BookmarkModel struct {
	Pool        *pgxpool.Pool
	GenAIClient *genai.Client
}

// TODO: Add validation of the db query inputs (Like Id)
// TODO: When LLMs are inlcuded, don't use them for imports, such as pocket
func (model *BookmarkModel) Create(
	link string,
	userId types.UserId,
	source BookmarkSource,
	subscriptionStatus SubscriptionStatus) (*Bookmark, error) {
	return model.CreateWithContent(link, userId, source, subscriptionStatus, "", "", nil)
}

// CreateWithContent creates a bookmark with provided HTML and text content
// If htmlContent and textContent are provided, they will be used instead of fetching the page
// If title and excerpt are provided, they will be used instead of extracting from content
func (model *BookmarkModel) CreateWithContent(
	link string,
	userId types.UserId,
	source BookmarkSource,
	subscriptionStatus SubscriptionStatus,
	htmlContent string,
	textContent string,
	bookmark *Bookmark,
) (*Bookmark, error) {

	// Check if the link already exists
	existingBookmark, err := model.GetByLink(userId, link)
	if err != nil {
		if !errors.Is(err, errors.ErrNotFound) {
			return nil, fmt.Errorf("failed to collect row: %w", err)
		}
	} else {
		return existingBookmark, nil
	}

	_, err = url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse URL in create bookmark: %w", err)
	}

	var article *readability.Article
	var content string

	// If HTML content is provided, use it instead of fetching the page
	if htmlContent != "" {
		slog.Info("Using provided content from extension", "link", link, "htmlSize", len(htmlContent), "title", bookmark.Title, "excerpt", bookmark.Excerpt)
		textContent := allTagsRemoved(htmlContent)
		excerpt := textContent[:min(200, len(textContent))]
		if bookmark.Excerpt != "" {
			excerpt = bookmark.Excerpt
		}
		title := "Unknown title"
		if bookmark.Title != "" {
			title = bookmark.Title
		}

		article = &readability.Article{
			Title:         title,
			Content:       htmlContent,
			TextContent:   textContent,
			Excerpt:       excerpt,
			Image:         bookmark.ImageUrl,
			Language:      bookmark.ArticleLang,
			SiteName:      bookmark.SiteName,
			PublishedTime: bookmark.PublishedTime,
		}
	} else {
		// Fallback to the original method of fetching the page
		slog.Info("Fetching page content", "link", link)
		resp, err := getPage(link)
		if err != nil {
			return nil, fmt.Errorf("failed to get page: %w", err)
		}
		defer resp.Body.Close()
		finalURL := resp.Request.URL

		// ******
		// TODO: readability.Check
		// ******

		articleValue, err := readability.FromReader(resp.Body, finalURL)
		// TODO: Check for the language
		if err != nil {
			return nil, fmt.Errorf("readability: %w", err)
		}
		article = &articleValue
	}

	content = validations.CleanUpText(article.TextContent)

	bookmarkId := strings.ToLower(rand.Text())[:8]
	bookmark = &Bookmark{
		BookmarkId:    types.BookmarkId(bookmarkId),
		UserId:        userId,
		Title:         validations.CleanUpText(article.Title),
		Link:          link,
		Excerpt:       validations.CleanUpText(article.Excerpt),
		ImageUrl:      article.Image,
		PublishedTime: article.PublishedTime,
		ArticleLang:   article.Language,
		SiteName:      article.SiteName,
		Source:        sourceMapping[source],
	}

	if article.Image == "" {
		bookmark.ImageUrl = ""
	} else if _, err := url.ParseRequestURI(article.Image); err != nil {
		slog.Warn("Failed to parse image URL", "error", err)
		bookmark.ImageUrl = ""
	}

	// Only generate AI content for premium users and not for imports (like Pocket)
	if subscriptionStatus == SubscriptionStatusPremium && source != Pocket && false {
		contentForMarkdown := content
		if htmlContent != "" {
			contentForMarkdown = htmlContent
		}
		// Generate the markdown content using Gemini
		go model.generateAIData(contentForMarkdown, link, bookmarkId)
	}

	// TODO: Add excerpt to bookmarks_content table
	_, err = model.Pool.Exec(context.Background(), `
		WITH inserted_bookmark AS (
			INSERT INTO library_items (
				bookmark_id,
				user_id,
				link,
				title,
				source,
				excerpt,
				image_url,
				article_lang,
				site_name,
				published_time
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		)
		INSERT INTO library_contents (bookmark_id, title, excerpt, content)
		VALUES ($1, $4, $6, $11);`,
		bookmarkId, userId, link, article.Title, sourceMapping[source], bookmark.Excerpt,
		article.Image, article.Language, article.SiteName, article.PublishedTime, content)
	if err != nil {
		return nil, fmt.Errorf("bookmark create: %w", err)
	}

	return bookmark, nil
}

func (model *BookmarkModel) GetById(id types.BookmarkId) (*Bookmark, error) {
	bookmark := Bookmark{
		BookmarkId: id,
	}
	rows, err := model.Pool.Query(context.Background(),
		`SELECT * FROM library_items WHERE bookmark_id = $1;`, id)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by id: %w", err)
	}
	bookmark, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		return nil, fmt.Errorf("collect exactly one row: %w", err)
	}
	return &bookmark, nil
}

func (model *BookmarkModel) GetByUserId(userId types.UserId, page int) ([]Bookmark, bool, error) {
	row := model.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM library_items WHERE user_id = $1`, userId)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return nil, false, fmt.Errorf("count bookmarks to get all by user ID: %w", err)
	}
	if count == 0 {
		return []Bookmark{}, false, nil
	}

	if page <= 0 || page >= 100 {
		return nil, false, fmt.Errorf("page number out of range")
	}
	page -= 1
	rows, err := model.Pool.Query(context.Background(),
		`SELECT bookmark_id, title, link, excerpt, created_at
		FROM library_items
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
		OFFSET $3
		`, userId, PageSize, page*PageSize)
	if err != nil {
		return nil, false, fmt.Errorf("query bookmark by user id: %w", err)
	}
	defer rows.Close()
	// TODO: Get all the row elements
	var bookmarks []Bookmark
	// Iterate through the result set
	for rows.Next() {
		var bookmark Bookmark
		err := rows.Scan(&bookmark.BookmarkId, &bookmark.Title, &bookmark.Link, &bookmark.Excerpt, &bookmark.CreatedAt)
		if err != nil {
			return nil, false, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmark)
	}
	if rows.Err() != nil {
		return nil, false, fmt.Errorf("iterating rows: %w", rows.Err())
	}
	morePages := PageSize+page*PageSize < count

	return bookmarks, morePages, nil
}

// GetRecentBookmarks returns the most recent bookmarks for the home page
func (model *BookmarkModel) GetRecentBookmarks(userId types.UserId, limit int, subscriptionStatus SubscriptionStatus) ([]Bookmark, error) {
	rows, err := model.Pool.Query(context.Background(), `
		SELECT
			user_id,
			bookmark_id,
			title,
			link,
			excerpt,
			source,
			article_lang,
			site_name,
			published_time,
			image_url,
			created_at,
			ai_summary,
			ai_excerpt,
			ai_tags
		FROM library_items
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userId, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent bookmarks: %w", err)
	}
	defer rows.Close()

	bookmarks, err := pgx.CollectRows(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		return nil, fmt.Errorf("collect rows: %w", err)
	}
	return bookmarks, nil
}

func (model *BookmarkModel) GetByLink(userId types.UserId, link string) (*Bookmark, error) {
	rows, err := model.Pool.Query(context.Background(),
		`SELECT *
		FROM library_items
		WHERE user_id = $1 AND link = $2`, userId, link)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by link: %w", err)
	}
	bookmark, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by link: %w", err)
	}
	return &bookmark, nil
}

func allTagsRemoved(htmlContent string) string {
	// Remove everything except the text content
	// This will be executed after cleanHTMLForLLM, so we already know that the htmlContent is clean

	// Remove all HTML tags
	tagRe := regexp.MustCompile(`<[^>]*>`)
	textOnly := tagRe.ReplaceAllString(htmlContent, "")

	// Decode HTML entities
	textOnly = strings.ReplaceAll(textOnly, "&nbsp;", " ")
	textOnly = strings.ReplaceAll(textOnly, "&amp;", "&")
	textOnly = strings.ReplaceAll(textOnly, "&lt;", "<")
	textOnly = strings.ReplaceAll(textOnly, "&gt;", ">")
	textOnly = strings.ReplaceAll(textOnly, "&quot;", "\"")
	textOnly = strings.ReplaceAll(textOnly, "&#39;", "'")

	// Remove extra whitespace and normalize
	whitespaceRe := regexp.MustCompile(`\s+`)
	textOnly = whitespaceRe.ReplaceAllString(textOnly, " ")

	return strings.TrimSpace(textOnly)
}

// cleanHTMLForLLM removes unnecessary HTML elements and attributes to reduce LLM costs
func cleanHTMLForLLM(htmlContent string) string {
	// Remove script and style tags completely
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	styleRe := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)

	// Remove comments
	commentRe := regexp.MustCompile(`<!--.*?-->`)

	// Remove common tracking and analytics elements
	trackingRe := regexp.MustCompile(`(?i)<[^>]*(?:data-track|data-analytics|onclick|onload|onerror)[^>]*>`)

	// Remove most attributes except essential ones for content structure
	attrRe := regexp.MustCompile(`(?i)\s+(?:class|id|style|data-[^=]*|onclick|onload|onerror|width|height|align|bgcolor|border|cellpadding|cellspacing|valign)="[^"]*"`)

	// Remove empty lines and extra whitespace
	whitespaceRe := regexp.MustCompile(`\s+`)

	cleaned := htmlContent
	cleaned = scriptRe.ReplaceAllString(cleaned, "")
	cleaned = styleRe.ReplaceAllString(cleaned, "")
	cleaned = commentRe.ReplaceAllString(cleaned, "")
	cleaned = trackingRe.ReplaceAllString(cleaned, "")
	cleaned = attrRe.ReplaceAllString(cleaned, "")
	cleaned = whitespaceRe.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

func (model *BookmarkModel) generateAIData(content string, link string, bookmarkId string) {
	// Clean up HTML content to reduce LLM costs
	htmlContent := cleanHTMLForLLM(content)
	// Log the duration of the function and size of the content
	start := time.Now()
	slog.Info("starting to convert HTML to markdown", "link", link, "size", len(htmlContent))
	aiDataResponse, err := model.promptToGetAIData(htmlContent)
	if err != nil {
		slog.Warn("Failed to convert HTML to markdown, using text content", "error", err)
		return
	}
	slog.Info("converted HTML to markdown", "link", link, "duration", time.Since(start), "markdown_size", len(aiDataResponse.Markdown))
	// Update the bookmark with all AI-generated content in library_items table
	_, err = model.Pool.Exec(context.Background(), `
		UPDATE library_items SET ai_summary = $1, ai_excerpt = $2, ai_tags = $3 WHERE bookmark_id = $4`,
		aiDataResponse.Summary, aiDataResponse.Excerpt, aiDataResponse.Tags, bookmarkId)
	if err != nil {
		slog.Warn("Failed to update bookmark AI content", "error", err)
	}

	// Also update the markdown content in library_contents table
	_, err = model.Pool.Exec(context.Background(), `
		UPDATE library_contents SET ai_markdown = $1 WHERE bookmark_id = $2`,
		aiDataResponse.Markdown, bookmarkId)
	if err != nil {
		slog.Warn("Failed to update bookmark markdown content", "error", err)
	}
}

type aiDataResponseType struct {
	Markdown string `json:"markdown"`
	Summary  string `json:"summary"`
	Excerpt  string `json:"excerpt"`
	Tags     string `json:"tags"`
}

// promptToGetAIData uses Gemini to convert HTML content to markdown format and generate additional AI content
func (model *BookmarkModel) promptToGetAIData(htmlContent string) (*aiDataResponseType, error) {
	if model.GenAIClient == nil {
		return nil, fmt.Errorf("GenAI client not initialized")
	}

	// Limit content length to avoid excessive costs (roughly 8000 characters = ~2000 tokens)
	if len(htmlContent) > 8000 {
		htmlContent = htmlContent[:8000] + "..."
	}

	prompt := `You are an expert at analyzing HTML content and converting it to clean, well-formatted Markdown. Your task is to process the provided HTML content and return FOUR separate outputs using the following structured format:

===MARKDOWN===
[Convert the HTML to clean markdown here]
===END MARKDOWN===

===SUMMARY===
[Write a concise one-paragraph summary here]
===END SUMMARY===

===EXCERPT===
[Extract one representative paragraph from the article here]
===END EXCERPT===

===TAGS===
[comma,separated,list,of,relevant,tags]
===END TAGS===

Instructions for each output:

MARKDOWN:
1. Convert HTML headings (h1-h6) to appropriate Markdown headings (# to ######)
2. Convert HTML lists (ul, ol, li) to Markdown lists (-, *, 1.)
3. Convert HTML links (<a>) to Markdown links [text](url)
4. Convert HTML emphasis (<em>, <i>) to *italic*
5. Convert HTML strong (<strong>, <b>) to **bold**
6. Convert HTML code blocks (<pre>, <code>) to Markdown code blocks
7. Convert HTML blockquotes to Markdown blockquotes (>)
8. Convert HTML tables to Markdown tables when possible
9. Convert HTML images to Markdown images ![alt](src)
10. Remove all HTML tags that don't contribute to content structure
11. Preserve paragraph breaks and line spacing for readability
12. Remove navigation menus, sidebars, footers, and other non-content elements
13. Focus on the main article/content body
14. Ensure the output is clean and properly formatted Markdown
15. Only keep the main content of the page and throw away all the meta content

SUMMARY:
- Write a concise one-paragraph summary (2-3 sentences) of the main content
- Focus on the key points and main message of the article
- Use clear, professional language
- Keep it under 200 words

EXCERPT:
- Look for what can be considered as the main content of the article
- The main content is the content that is most relevant to the user
- Pick the first paragraph of the main content
- Use the exact text as it appears in the article (verbatim)
- Keep it under 200 words

TAGS:
- Generate 5-8 relevant tags that describe the content
- Use lowercase, single words or short phrases
- Separate with commas, no spaces after commas
- Focus on topics, themes, and key concepts
- Examples: technology,programming,web development,ai,machine learning

HTML content to process:
` + htmlContent

	ctx := context.Background()
	result, err := model.GenAIClient.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-lite-preview-06-17",
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("generate content with Gemini: %w", err)
	}

	responseText := result.Text()

	// Parse the structured response
	aiDataResponse := &aiDataResponseType{}

	// Extract markdown
	if markdownMatch := regexp.MustCompile(`(?s)===MARKDOWN===\n(.*?)\n===END MARKDOWN===`).FindStringSubmatch(responseText); len(markdownMatch) > 1 {
		aiDataResponse.Markdown = strings.TrimSpace(markdownMatch[1])
	}

	// Extract summary
	if summaryMatch := regexp.MustCompile(`(?s)===SUMMARY===\n(.*?)\n===END SUMMARY===`).FindStringSubmatch(responseText); len(summaryMatch) > 1 {
		aiDataResponse.Summary = strings.TrimSpace(summaryMatch[1])
	}

	// Extract excerpt
	if excerptMatch := regexp.MustCompile(`===EXCERPT===\n(.*?)\n===END EXCERPT===`).FindStringSubmatch(responseText); len(excerptMatch) > 1 {
		aiDataResponse.Excerpt = strings.TrimSpace(excerptMatch[1])
	}

	// Extract tags
	if tagsMatch := regexp.MustCompile(`===TAGS===\n(.*?)\n===END TAGS===`).FindStringSubmatch(responseText); len(tagsMatch) > 1 {
		aiDataResponse.Tags = strings.TrimSpace(tagsMatch[1])
	}

	return aiDataResponse, nil
}

func (model *BookmarkModel) Update(bookmark *Bookmark) error {
	_, err := model.Pool.Exec(context.Background(),
		`UPDATE library_items SET link = $1, title = $2 WHERE bookmark_id = $3`,
		bookmark.Link, bookmark.Title, bookmark.BookmarkId,
	)
	if err != nil {
		return fmt.Errorf("update bookmark: %w", err)
	}
	return nil
}

func (model *BookmarkModel) Delete(id types.BookmarkId) error {
	_, err := model.Pool.Exec(context.Background(),
		`DELETE FROM library_items WHERE bookmark_id = $1;`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}

type SearchResult struct {
	Headline   string
	BookmarkId types.BookmarkId
	Title      string
	Link       string
	Excerpt    string
	ImageUrl   string
	CreatedAt  time.Time
	Rank       float32
	// AI-generated fields for premium users
	AISummary *string
	AIExcerpt *string
	AITags    *string
}

func (model *BookmarkModel) Search(userId types.UserId, query string, subscriptionStatus SubscriptionStatus) ([]SearchResult, error) {
	rows, err := model.Pool.Query(context.Background(), `
		WITH search_query AS (
			SELECT plainto_tsquery(CASE WHEN $1 = '' THEN '' ELSE $1 END) AS query
		)
		SELECT
			ts_headline(bc.content, sq.query, 'MaxFragments=2, StartSel=<strong>, StopSel=</strong>') AS headline,
			ub.bookmark_id AS bookmark_id,
			ub.title AS title,
			ub.link AS link,
			ub.excerpt AS excerpt,
			ub.image_url AS image_url,
			ub.created_at AS created_at,
			ts_rank(bc.search_vector, sq.query) AS rank,
			ub.ai_summary AS ai_summary,
			ub.ai_excerpt AS ai_excerpt,
			ub.ai_tags AS ai_tags
		FROM library_items ub
		JOIN library_contents bc ON ub.bookmark_id = bc.bookmark_id
		CROSS JOIN search_query sq
		WHERE ub.user_id = $2
    		AND bc.search_vector @@ sq.query
		ORDER BY rank DESC
		LIMIT 10`, query, userId)

	if err != nil {
		return nil, fmt.Errorf("search bookmarks: %w", err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[SearchResult])
	if err != nil {
		return nil, fmt.Errorf("collect bookmark search rows: %w", err)
	}
	if subscriptionStatus != SubscriptionStatusPremium {
		for i := range results {
			results[i].AISummary = nil
			results[i].AIExcerpt = nil
			results[i].AITags = nil
		}
	}
	return results, nil
}

func (model *BookmarkModel) GetBookmarkContent(id types.BookmarkId) (string, error) {
	rows, err := model.Pool.Query(context.Background(), `
		SELECT content
		FROM library_contents
		WHERE bookmark_id = $1
		LIMIT 1`, id)
	if err != nil {
		return "", fmt.Errorf("query bookmark content by id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return "", errors.ErrNotFound
	}
	var content string
	err = rows.Scan(&content)
	if err != nil {
		return "", fmt.Errorf("scan bookmark content: %w", err)
	}
	if rows.Err() != nil {
		return "", fmt.Errorf("iterating rows: %w", rows.Err())
	}
	return content, nil
}

// GetBookmarkMarkdown retrieves the AI-generated markdown content for a bookmark
func (model *BookmarkModel) GetBookmarkMarkdown(id types.BookmarkId) (string, error) {
	rows, err := model.Pool.Query(context.Background(), `
		SELECT ai_markdown
		FROM library_contents
		WHERE bookmark_id = $1
		LIMIT 1`, id)
	if err != nil {
		return "", fmt.Errorf("query bookmark markdown by id: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return "", errors.ErrNotFound
	}
	var markdown sql.NullString
	err = rows.Scan(&markdown)
	if err != nil {
		return "", fmt.Errorf("scan bookmark markdown: %w", err)
	}
	if rows.Err() != nil {
		return "", fmt.Errorf("iterating rows: %w", rows.Err())
	}

	if !markdown.Valid {
		return "", errors.ErrNotFound
	}

	return markdown.String, nil
}

func (model *BookmarkModel) GetFullBookmark(id types.BookmarkId) (*BookmarkWithContent, error) {
	rows, err := model.Pool.Query(context.Background(), `
	SELECT
		ub.bookmark_id,
		ub.user_id,
		ub.title,
		ub.link,
		ub.source,
		bc.excerpt,
		bc.content,
		ub.image_url,
		bc.article_lang,
		ub.site_name,
		ub.created_at,
		bc.published_time
	FROM library_items ub
	JOIN library_contents bc ON ub.bookmark_id = bc.bookmark_id
	WHERE ub.bookmark_id = $1`, id)

	if err != nil {
		return nil, fmt.Errorf("query full bookmark by id: %w", err)
	}

	bookmark, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[BookmarkWithContent])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("bookmark by id: %w", err)
	}

	return &bookmark, nil
}

var metaRefreshRe = regexp.MustCompile(`(?i)<meta[^>]+http-equiv=["']?refresh["']?[^>]+content=["']?\d+;\s*url=([^"'>]+)["']?`)

func getPage(link string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	// Accept any 2xx status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	body := string(bodyBytes)

	metaRefresh := metaRefreshRe.FindStringSubmatch(body)
	if len(metaRefresh) > 0 {
		redirectURL, err := url.Parse(metaRefresh[1])
		if err != nil {
			return nil, fmt.Errorf("parse redirect URL: %w", err)
		}
		redirectLink := redirectURL.String()
		// If the redirect link is a relative link, join it with the host of the original link
		if strings.Index(redirectLink, "/") == 0 {
			parsedURL, err := url.Parse(link)
			if err != nil {
				return nil, fmt.Errorf("parse original link: %w", err)
			}
			redirectLink = parsedURL.Scheme + "://" + parsedURL.Host + redirectLink
		}
		return getPage(redirectLink)
	}
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return resp, nil
}
