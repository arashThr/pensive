package models

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/types"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/go-shiori/go-readability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"google.golang.org/genai"
)

type BookmarkSource = int

const PageSize = 10

const (
	FreeUserDailyLimit       = 10  // Free users: 10 bookmarks with AI per day
	PremiumUserDailyLimit    = 100 // Premium users: 100 bookmarks with AI per day
	UnverifiedUserTotalLimit = 10  // Unverified users: 10 total bookmarks until verified
)

const (
	FreeUserDailyAIQuestions    = 1  // Free users: 1 AI question per day
	PremiumUserDailyAIQuestions = 10 // Premium users: 10 AI questions per day
)

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
	Id               types.BookmarkId
	UserId           types.UserId
	Title            string
	Link             string
	Source           string
	Excerpt          string
	ImageUrl         string
	ArticleLang      string
	SiteName         string
	AISummary        *string
	AIExcerpt        *string
	AITags           *string
	ExtractionMethod types.ExtractionMethod
	CreatedAt        time.Time
	PublishedTime    *time.Time
}

type BookmarkWithContent struct {
	Bookmark
	Content string // Full content of the bookmark
}

type BookmarkRepo struct {
	Pool        *pgxpool.Pool
	GenAIClient *genai.Client
}

// TODO: Add validation of the db query inputs (Like Id)
// TODO: When LLMs are inlcuded, don't use them for imports, such as pocket
func (model *BookmarkRepo) Create(
	ctx context.Context,
	link string,
	user *User,
	source BookmarkSource) (*Bookmark, error) {
	return model.CreateWithContent(ctx, link, user, source, nil)
}

// CreateWithContent creates a bookmark with provided HTML and text content
// If htmlContent and textContent are provided, they will be used instead of fetching the page
// If title and excerpt are provided, they will be used instead of extracting from content
func (model *BookmarkRepo) CreateWithContent(
	ctx context.Context,
	link string,
	user *User,
	source BookmarkSource,
	bookmarkRequest *types.CreateBookmarkRequest,
) (*Bookmark, error) {
	logger := loggercontext.Logger(ctx)
	parsedURL, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse URL in create bookmark: %w", err)
	}

	canonicalizedLink := validations.CanonicalURL(parsedURL)

	// Check if the link already exists
	existingBookmark, err := model.GetByLink(user.ID, canonicalizedLink)
	if err != nil {
		if !errors.Is(err, errors.ErrNotFound) {
			return nil, fmt.Errorf("failed to collect row: %w", err)
		}
	} else {
		return existingBookmark, nil
	}

	// Check rate limit before creating new bookmark
	if err := model.checkRateLimit(user); err != nil && source != Pocket {
		return nil, err
	}

	extractionMethod := types.ExtractionMethodServer
	if bookmarkRequest != nil {
		if bookmarkRequest.TextContent != "" && bookmarkRequest.HtmlContent != "" {
			extractionMethod = types.ExtractionMethodReadabilityHTML
		} else if bookmarkRequest.TextContent != "" {
			extractionMethod = types.ExtractionMethodReadability
		} else if bookmarkRequest.HtmlContent != "" {
			extractionMethod = types.ExtractionMethodHTML
		}
	}

	// Fallback to the original method of fetching the page
	article, err := fetchLink(ctx, link)
	if err != nil {
		logger.Warnw("Failed to fetch page on server", "error", err, "link", link)
		// Neither Readability nor the extension provided content
		// So we can't create a bookmark
		if bookmarkRequest == nil {
			logger.Warnw("Page content inaccessible", "link", link, "userId", user.ID)
			return nil, fmt.Errorf("page content inaccessible")
		}
	}

	// Input coming from the extension get higher priority than the fetched page
	htmlContent := ""
	if bookmarkRequest != nil {
		switch {
		case bookmarkRequest.Title != "":
			article.Title = bookmarkRequest.Title
		case bookmarkRequest.Excerpt != "":
			article.Excerpt = bookmarkRequest.Excerpt
		case bookmarkRequest.Lang != "":
			article.Language = bookmarkRequest.Lang
		case bookmarkRequest.SiteName != "":
			article.SiteName = bookmarkRequest.SiteName
		case bookmarkRequest.PublishedTime != nil:
			article.PublishedTime = bookmarkRequest.PublishedTime
		}
		htmlContent = bookmarkRequest.HtmlContent
	}

	// If HTML content is provided, use it instead of what Readability fetched
	if htmlContent != "" {
		logger.Infow("Using provided content from extension", "link", link, "htmlSize", len(htmlContent), "readabilityHtmlSize", len(article.Content))
		textContent := allTagsRemoved(htmlContent)
		article.TextContent = textContent
		article.Content = htmlContent
	}
	if article.Excerpt == "" {
		article.Excerpt = article.TextContent[:min(200, len(article.TextContent))]
	}
	if article.Title == "" {
		article.Title = parsedURL.String()
	}

	bookmarkId := strings.ToLower(rand.Text())[:8]
	inputBookmark := Bookmark{
		Id:               types.BookmarkId(bookmarkId),
		UserId:           user.ID,
		Title:            validations.CleanUpText(article.Title),
		Link:             canonicalizedLink,
		Excerpt:          validations.CleanUpText(article.Excerpt),
		ImageUrl:         article.Image,
		PublishedTime:    article.PublishedTime,
		ArticleLang:      article.Language,
		SiteName:         article.SiteName,
		Source:           sourceMapping[source],
		ExtractionMethod: extractionMethod,
	}

	if inputBookmark.ImageUrl != "" {
		if _, err := url.ParseRequestURI(inputBookmark.ImageUrl); err != nil {
			logger.Warnw("Failed to parse image URL", "error", err)
			inputBookmark.ImageUrl = ""
		}
	}

	_, err = model.Pool.Exec(ctx, `
		WITH inserted_bookmark AS (
			INSERT INTO library_items (
				id,
				user_id,
				link,
				title,
				source,
				excerpt,
				image_url,
				article_lang,
				site_name,
				published_time,
				extraction_method
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		)
		INSERT INTO library_contents (id, title, excerpt, content)
		VALUES ($1, $4, $6, $12);`,
		inputBookmark.Id, user.ID, inputBookmark.Link, inputBookmark.Title, inputBookmark.Source, inputBookmark.Excerpt,
		inputBookmark.ImageUrl, inputBookmark.ArticleLang, inputBookmark.SiteName, inputBookmark.PublishedTime,
		inputBookmark.ExtractionMethod, article.TextContent)
	if err != nil {
		return nil, fmt.Errorf("bookmark create: %w", err)
	}

	// Generate AI content for all users except for imports (like Pocket)
	if source != Pocket {
		// TODO: Should I also put content in db?
		contentForMarkdown := article.TextContent
		if article.Content != "" {
			contentForMarkdown = article.Content
		}
		// Generate the markdown content using Gemini
		go model.generateAIData(ctx, article.Title, contentForMarkdown, link, bookmarkId)
	}

	return &inputBookmark, nil
}

func fetchLink(ctx context.Context, link string) (readability.Article, error) {
	logger := loggercontext.Logger(ctx)
	logger.Infow("Fetching page content", "link", link)
	resp, err := getPage(link)
	if err != nil {
		logger.Warnw("Failed to fetch page", "error", err, "link", link)
		return readability.Article{}, fmt.Errorf("fetch page on server: %w", err)
	}
	defer resp.Body.Close()
	finalURL := resp.Request.URL

	// ******
	// TODO: readability.Check
	// ******

	article, err := readability.FromReader(resp.Body, finalURL)
	// TODO: Check for the language
	if err != nil {
		logger.Warnw("Failed to parse page", "error", err, "link", link)
		return readability.Article{}, fmt.Errorf("parse page on server: %w", err)
	}
	return article, nil
}

func (model *BookmarkRepo) GetById(id types.BookmarkId) (*Bookmark, error) {
	bookmark := Bookmark{
		Id: id,
	}
	rows, err := model.Pool.Query(context.Background(),
		`SELECT * FROM library_items WHERE id = $1;`, id)
	if err != nil {
		return nil, fmt.Errorf("query bookmark by id: %w", err)
	}
	bookmark, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[Bookmark])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("collect exactly one row: %w", err)
	}
	return &bookmark, nil
}

func (model *BookmarkRepo) GetByUserId(userId types.UserId, page int) ([]Bookmark, int, bool, error) {
	row := model.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM library_items WHERE user_id = $1`, userId)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return nil, 0, false, fmt.Errorf("count bookmarks to get all by user ID: %w", err)
	}
	if count == 0 {
		return []Bookmark{}, 0, false, nil
	}

	if page <= 0 || page >= 100 {
		return nil, 0, false, fmt.Errorf("page number out of range")
	}
	page -= 1
	rows, err := model.Pool.Query(context.Background(),
		`SELECT id, title, link, excerpt, created_at
		FROM library_items
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
		OFFSET $3
		`, userId, PageSize, page*PageSize)
	if err != nil {
		return nil, 0, false, fmt.Errorf("query bookmark by user id: %w", err)
	}
	defer rows.Close()
	// TODO: Get all the row elements
	var bookmarks []Bookmark
	// Iterate through the result set
	for rows.Next() {
		var bookmark Bookmark
		err := rows.Scan(&bookmark.Id, &bookmark.Title, &bookmark.Link, &bookmark.Excerpt, &bookmark.CreatedAt)
		if err != nil {
			return nil, 0, false, fmt.Errorf("scan bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmark)
	}
	if rows.Err() != nil {
		return nil, 0, false, fmt.Errorf("iterating rows: %w", rows.Err())
	}
	morePages := PageSize+page*PageSize < count

	return bookmarks, count, morePages, nil
}

// GetRecentBookmarks returns the most recent bookmarks for the home page
func (model *BookmarkRepo) GetRecentBookmarks(user *User, limit int) ([]Bookmark, error) {
	rows, err := model.Pool.Query(context.Background(), `
		SELECT * FROM library_items
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, user.ID, limit)
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

func (model *BookmarkRepo) GetByLink(userId types.UserId, link string) (*Bookmark, error) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse URL in get bookmark by link: %w", err)
	}
	cannonilizedLink := validations.CanonicalURL(parsedURL)
	rows, err := model.Pool.Query(context.Background(),
		`SELECT *
		FROM library_items
		WHERE user_id = $1 AND link = $2`, userId, cannonilizedLink)
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
	textOnly := tagRe.ReplaceAllString(htmlContent, " ")

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
	cleaned = scriptRe.ReplaceAllString(cleaned, " ")
	cleaned = styleRe.ReplaceAllString(cleaned, " ")
	cleaned = commentRe.ReplaceAllString(cleaned, " ")
	cleaned = trackingRe.ReplaceAllString(cleaned, " ")
	cleaned = attrRe.ReplaceAllString(cleaned, " ")
	cleaned = whitespaceRe.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

func (model *BookmarkRepo) generateAIData(ctx context.Context, title string, content string, link string, bookmarkId string) {
	logger := loggercontext.Logger(ctx)
	// Use context.Background() since this runs in a goroutine and the request context may be canceled
	genCtx := context.Background()
	// Clean up HTML content to reduce LLM costs
	htmlContent := cleanHTMLForLLM(content)
	// Log the duration of the function and size of the content
	start := time.Now()
	logger.Infow("starting to convert HTML to markdown", "link", link, "size", len(htmlContent))
	aiDataResponse, err := model.promptToGetAIData(htmlContent)
	if err != nil {
		logger.Warnw("Failed to convert HTML to markdown, using text content", "error", err)
		return
	}
	logger.Infow("converted HTML to markdown", "link", link, "duration", time.Since(start), "markdown_size", len(aiDataResponse.Markdown))
	// Update the bookmark with all AI-generated content in library_items table
	_, err = model.Pool.Exec(genCtx, `
		UPDATE library_items SET ai_summary = $1, ai_excerpt = $2, ai_tags = $3 WHERE id = $4`,
		aiDataResponse.Summary, aiDataResponse.Excerpt, aiDataResponse.Tags, bookmarkId)
	if err != nil {
		logger.Warnw("Failed to update bookmark AI content", "error", err)
	}

	// Also update the markdown content in library_contents table
	_, err = model.Pool.Exec(genCtx, `
		UPDATE library_contents SET ai_markdown = $1 WHERE id = $2`,
		aiDataResponse.Markdown, bookmarkId)
	if err != nil {
		logger.Warnw("Failed to update bookmark markdown content", "error", err)
	}

	// Generate and store embedding for the content
	logger.Infow("generating embedding for bookmark", "link", link)
	fullAIText := aiDataResponse.Markdown + "\n\n" + aiDataResponse.Excerpt + "\n\n" + aiDataResponse.Summary
	embedding, err := model.generateEmbedding(genCtx, title, fullAIText)
	if err != nil {
		logger.Warnw("Failed to generate embedding", "error", err, "link", link)
		return
	}

	// Convert to pgvector type and store
	_, err = model.Pool.Exec(genCtx, `
		UPDATE library_contents SET content_embedding = $1 WHERE id = $2`,
		pgvector.NewVector(embedding), bookmarkId)
	if err != nil {
		logger.Warnw("Failed to store embedding", "error", err, "link", link)
	} else {
		logger.Infow("embedding stored successfully", "link", link, "dimensions", len(embedding))
	}
}

type aiDataResponseType struct {
	Markdown string `json:"markdown"`
	Summary  string `json:"summary"`
	Excerpt  string `json:"excerpt"`
	Tags     string `json:"tags"`
}

// promptToGetAIData uses Gemini to convert HTML content to markdown format and generate additional AI content
func (model *BookmarkRepo) promptToGetAIData(htmlContent string) (*aiDataResponseType, error) {
	if model.GenAIClient == nil {
		return nil, fmt.Errorf("GenAI client not initialized")
	}

	// Limit content length to avoid excessive costs (roughly 8000 characters = ~2000 tokens)
	if len(htmlContent) > 8000 {
		htmlContent = htmlContent[:8000] + "... [SKIPPED CONTENT] ..." + htmlContent[len(htmlContent)-2000:]
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
- Use the exact text (in HTML format) as it appears in the article (verbatim)
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
		logging.Telegram.SendMessage(fmt.Sprintf("Failed to generate content with Gemini: %v", err))
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
	if excerptMatch := regexp.MustCompile(`(?s)===EXCERPT===\n(.*?)\n===END EXCERPT===`).FindStringSubmatch(responseText); len(excerptMatch) > 1 {
		aiDataResponse.Excerpt = strings.TrimSpace(excerptMatch[1])
	}

	// Extract tags
	if tagsMatch := regexp.MustCompile(`(?s)===TAGS===\n(.*?)\n===END TAGS===`).FindStringSubmatch(responseText); len(tagsMatch) > 1 {
		aiDataResponse.Tags = strings.TrimSpace(tagsMatch[1])
	}

	return aiDataResponse, nil
}

// generateEmbedding generates a vector embedding for the given text using Google's embedding model
func (model *BookmarkRepo) generateEmbedding(ctx context.Context, title, text string) ([]float32, error) {
	logger := loggercontext.Logger(ctx)

	if model.GenAIClient == nil {
		return nil, fmt.Errorf("GenAI client not initialized")
	}

	// Limit content length to avoid API limits
	// Google's gemini-embedding-001 supports up to 2,048 tokens, but we'll use ~2000 chars (~500 tokens) for efficiency
	maxLength := 2000
	if len(text) > maxLength {
		// Take first 1000 and last 1000 chars to capture both intro and conclusion
		text = text[:1000] + "..." + text[len(text)-1000:]
	}

	// Use Google's gemini-embedding-001 model (768 dimensions)
	var dimension int32 = 768
	geminiConfigs := genai.EmbedContentConfig{
		OutputDimensionality: &dimension,
		TaskType:             "RETRIEVAL_DOCUMENT",
		Title:                title,
	}
	result, err := model.GenAIClient.Models.EmbedContent(
		ctx,
		"gemini-embedding-001",
		genai.Text(text),
		&geminiConfigs,
	)
	if err != nil {
		logging.Telegram.SendMessage(fmt.Sprintf("Failed to generate embedding with Gemini: %v", err))
		logger.Warnw("Failed to generate embedding", "error", err)
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	if len(result.Embeddings) == 0 {
		logging.Telegram.SendMessage("Empty embedding returned from Gemini")
		return nil, fmt.Errorf("empty embedding returned")
	}

	// Extract the float32 values from the first embedding
	if len(result.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("embedding has no values")
	}

	return result.Embeddings[0].Values, nil
}

func (model *BookmarkRepo) generateQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	logger := loggercontext.Logger(ctx)

	if model.GenAIClient == nil {
		return nil, fmt.Errorf("GenAI client not initialized")
	}

	// Limit query length to avoid API limits
	maxLength := 100
	if len(query) > maxLength {
		query = query[:maxLength] + "..."
	}

	var dimension int32 = 768
	geminiConfigs := genai.EmbedContentConfig{
		OutputDimensionality: &dimension,
		TaskType:             "RETRIEVAL_QUERY",
	}
	result, err := model.GenAIClient.Models.EmbedContent(
		ctx,
		"gemini-embedding-001",
		genai.Text(query),
		&geminiConfigs,
	)
	if err != nil {
		logger.Warnw("Failed to generate embedding for query", "error", err)
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("empty query embedding returned")
	}

	// Extract the float32 values from the first embedding
	if len(result.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("query embedding has no values")
	}

	return result.Embeddings[0].Values, nil

}

func (model *BookmarkRepo) Update(bookmark *Bookmark) error {
	_, err := model.Pool.Exec(context.Background(),
		`UPDATE library_items SET link = $1, title = $2 WHERE id = $3`,
		bookmark.Link, bookmark.Title, bookmark.Id,
	)
	if err != nil {
		return fmt.Errorf("update bookmark: %w", err)
	}
	return nil
}

func (model *BookmarkRepo) Delete(id types.BookmarkId) error {
	_, err := model.Pool.Exec(context.Background(),
		`DELETE FROM library_items WHERE id = $1;`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}
	return nil
}

type SearchResult struct {
	Headline  string
	Id        types.BookmarkId
	Title     string
	Link      string
	Excerpt   string
	ImageUrl  string
	CreatedAt time.Time
	Rank      float32
	// AI-generated fields for premium users
	AISummary *string
	AIExcerpt *string
	AITags    *string
}

func (model *BookmarkRepo) Search(user *User, query string) ([]SearchResult, error) {
	// Sanitize and validate the search query
	sanitizedQuery := sanitizeSearchQuery(query)
	if sanitizedQuery == "" {
		// Return empty results for empty queries instead of erroring
		return []SearchResult{}, nil
	}

	// Try full-text search first (instant, keyword-based)
	results, err := model.performFullTextSearch(user, sanitizedQuery)
	if err != nil {
		// If full-text search fails, fall back to simple pattern matching
		return model.performFallbackSearch(user, sanitizedQuery)
	}

	// If full-text search returns no results, try fallback search
	if len(results) == 0 {
		return model.performFallbackSearch(user, sanitizedQuery)
	}

	return results, nil
}

// sanitizeSearchQuery cleans and validates search input
func sanitizeSearchQuery(query string) string {
	if query == "" {
		return ""
	}

	// Trim whitespace and normalize
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Remove potentially problematic tsquery operators and characters
	// Keep alphanumeric, spaces, hyphens, and quotes for phrase searching
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s\-"']`)
	query = reg.ReplaceAllString(query, " ")

	// Normalize multiple spaces to single spaces
	spaceReg := regexp.MustCompile(`\s+`)
	query = spaceReg.ReplaceAllString(query, " ")

	// Trim again after cleanup
	query = strings.TrimSpace(query)

	// Ensure we don't return empty string after sanitization
	if query == "" {
		return ""
	}

	return query
}

// performFullTextSearch executes the full-text search using PostgreSQL's search capabilities
func (model *BookmarkRepo) performFullTextSearch(user *User, query string) ([]SearchResult, error) {
	rows, err := model.Pool.Query(context.Background(), `
		WITH search_terms AS (
			SELECT string_agg(
				CASE 
					WHEN trim(lexeme) = '' THEN NULL
					ELSE trim(lexeme) || ':*'
				END, 
				' & '
			) AS tsquery_string
			FROM unnest(string_to_array($1, ' ')) AS lexeme
			WHERE trim(lexeme) != ''
		),
		search_query AS (
			SELECT 
				CASE 
					WHEN st.tsquery_string IS NULL OR st.tsquery_string = '' THEN NULL
					ELSE to_tsquery('english', st.tsquery_string)
				END AS query
			FROM search_terms st
		)
		SELECT
			CASE 
				WHEN sq.query IS NOT NULL THEN 
					ts_headline('english', lc.content, sq.query, 'MaxFragments=2, StartSel=<strong>, StopSel=</strong>')
				ELSE lc.excerpt
			END AS headline,
			li.id AS id,
			li.title AS title,
			li.link AS link,
			li.excerpt AS excerpt,
			li.image_url AS image_url,
			li.created_at AS created_at,
			CASE 
				WHEN sq.query IS NOT NULL THEN ts_rank(lc.search_vector, sq.query)
				ELSE 0.0
			END AS rank,
			li.ai_summary AS ai_summary,
			li.ai_excerpt AS ai_excerpt,
			li.ai_tags AS ai_tags
		FROM library_items li
		JOIN library_contents lc ON li.id = lc.id
		CROSS JOIN search_query sq
		WHERE li.user_id = $2
			AND sq.query IS NOT NULL
			AND lc.search_vector IS NOT NULL
			AND lc.search_vector @@ sq.query
		ORDER BY rank DESC, li.created_at DESC
		LIMIT 10`, query, user.ID)

	if err != nil {
		return nil, fmt.Errorf("full-text search failed: %w", err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[SearchResult])
	if err != nil {
		return nil, fmt.Errorf("collect full-text search rows: %w", err)
	}

	return results, nil
}

// performFallbackSearch performs a simpler pattern-based search when full-text search fails
func (model *BookmarkRepo) performFallbackSearch(user *User, query string) ([]SearchResult, error) {
	// Create a pattern for ILIKE search
	pattern := "%" + strings.ReplaceAll(query, " ", "%") + "%"

	rows, err := model.Pool.Query(context.Background(), `
		SELECT
			COALESCE(li.excerpt, '(No excerpt available)') AS headline,
			li.id AS id,
			li.title AS title,
			li.link AS link,
			li.excerpt AS excerpt,
			li.image_url AS image_url,
			li.created_at AS created_at,
			CASE 
				WHEN LOWER(li.title) ILIKE LOWER($1) THEN 1.0
				WHEN LOWER(li.excerpt) ILIKE LOWER($1) THEN 0.8
				WHEN LOWER(lc.content) ILIKE LOWER($1) THEN 0.6
				WHEN LOWER(li.ai_summary) ILIKE LOWER($1) THEN 0.5
				WHEN LOWER(li.ai_tags) ILIKE LOWER($1) THEN 0.3
				ELSE 0.1
			END AS rank,
			li.ai_summary AS ai_summary,
			li.ai_excerpt AS ai_excerpt,
			li.ai_tags AS ai_tags
		FROM library_items li
		JOIN library_contents lc ON li.id = lc.id
		WHERE li.user_id = $2
			AND (
				LOWER(li.title) ILIKE LOWER($1) OR
				LOWER(li.excerpt) ILIKE LOWER($1) OR
				LOWER(lc.content) ILIKE LOWER($1) OR
				LOWER(COALESCE(li.ai_summary, '')) ILIKE LOWER($1) OR
				LOWER(COALESCE(li.ai_tags, '')) ILIKE LOWER($1)
			)
		ORDER BY rank DESC, li.created_at DESC
		LIMIT 10`, pattern, user.ID)

	if err != nil {
		return nil, fmt.Errorf("fallback search failed: %w", err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[SearchResult])
	if err != nil {
		return nil, fmt.Errorf("collect fallback search rows: %w", err)
	}

	return results, nil
}

// performVectorSearch performs semantic similarity search using embeddings
func (model *BookmarkRepo) performVectorSearch(ctx context.Context, user *User, query string) ([]SearchResult, error) {
	logger := loggercontext.Logger(ctx)

	// Generate embedding for the search query
	queryEmbedding, err := model.generateQueryEmbedding(ctx, query)
	if err != nil {
		logger.Warnw("Failed to generate query embedding", "error", err)
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	// Search for similar bookmarks using cosine similarity
	// The <=> operator in pgvector computes cosine distance (lower is more similar)
	// We convert distance to similarity score for consistency with full-text search
	rows, err := model.Pool.Query(ctx, `
		SELECT
			COALESCE(li.excerpt, lc.excerpt, '(No excerpt available)') AS headline,
			li.id AS id,
			li.title AS title,
			li.link AS link,
			li.excerpt AS excerpt,
			li.image_url AS image_url,
			li.created_at AS created_at,
			(1 - (lc.content_embedding <=> $1)) AS rank,
			li.ai_summary AS ai_summary,
			li.ai_excerpt AS ai_excerpt,
			li.ai_tags AS ai_tags
		FROM library_items li
		JOIN library_contents lc ON li.id = lc.id
		WHERE li.user_id = $2
			AND lc.content_embedding IS NOT NULL
		ORDER BY lc.content_embedding <=> $1
		LIMIT 10`, pgvector.NewVector(queryEmbedding), user.ID)

	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[SearchResult])
	if err != nil {
		return nil, fmt.Errorf("collect vector search rows: %w", err)
	}

	return results, nil
}

// RAGResponse represents the response from asking a question about bookmarks
type RAGResponse struct {
	Answer          string
	SourceBookmarks []SearchResult
}

// AskQuestion uses RAG (Retrieval-Augmented Generation) to answer questions about bookmarks
// It retrieves relevant bookmarks using semantic search, then uses Gemini to answer the question
func (model *BookmarkRepo) AskQuestion(ctx context.Context, user *User, question string) (*RAGResponse, error) {
	logger := loggercontext.Logger(ctx)

	if model.GenAIClient == nil {
		return nil, fmt.Errorf("GenAI client not initialized")
	}

	// Use vector search to find relevant bookmarks
	relevantBookmarks, err := model.performVectorSearch(ctx, user, question)
	if err != nil {
		logger.Warnw("Failed to retrieve relevant bookmarks", "error", err)
		return nil, fmt.Errorf("retrieve relevant bookmarks: %w", err)
	}

	if len(relevantBookmarks) == 0 {
		return &RAGResponse{
			Answer:          "I couldn't find any relevant bookmarks to answer your question.",
			SourceBookmarks: []SearchResult{},
		}, nil
	}

	// Limit to top 5 most relevant bookmarks to avoid token limits
	if len(relevantBookmarks) > 3 {
		relevantBookmarks = relevantBookmarks[:3]
	}

	// Fetch full content for the relevant bookmarks
	var contexts []string
	for i, bookmark := range relevantBookmarks {
		content, err := model.GetBookmarkMarkdown(bookmark.Id)
		if err != nil {
			logger.Warnw("Failed to get bookmark content", "error", err, "bookmarkId", bookmark.Id)
			continue
		}

		// Limit each content to 1000 chars to avoid excessive tokens
		if len(content) > 1000 {
			content = content[:1000] + "..."
		}

		contexts = append(contexts, fmt.Sprintf(
			"[Source %d: %s]\nURL: %s\nContent: %s\n",
			i+1, bookmark.Title, bookmark.Link, content,
		))
	}

	if len(contexts) == 0 {
		return &RAGResponse{
			Answer:          "I found relevant bookmarks but couldn't access their content.",
			SourceBookmarks: relevantBookmarks,
		}, nil
	}

	// Build the prompt for Gemini
	prompt := fmt.Sprintf(`You are a helpful assistant that answers questions based on the user's bookmarked content.

Question: %s

Here are the relevant bookmarks from the user's library:

%s

Instructions:
- Answer the question using ONLY the information provided in the bookmarks above
- Be concise and direct
- If the bookmarks don't contain enough information to answer the question, say so
- Cite your sources by mentioning the bookmark titles
- If multiple bookmarks provide relevant information, synthesize them into a coherent answer
- Keep your answer under 300 words

Answer:`, question, strings.Join(contexts, "\n---\n"))

	logger.Infow("generating RAG answer", "question", question, "numSources", len(contexts))

	// Generate answer using Gemini
	result, err := model.GenAIClient.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash-lite-preview-06-17",
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		logger.Warnw("Failed to generate RAG answer", "error", err)
		return nil, fmt.Errorf("generate RAG answer: %w", err)
	}

	answer := result.Text()

	return &RAGResponse{
		Answer:          answer,
		SourceBookmarks: relevantBookmarks,
	}, nil
}

func (model *BookmarkRepo) GetBookmarkContent(id types.BookmarkId) (string, error) {
	rows, err := model.Pool.Query(context.Background(), `
		SELECT content
		FROM library_contents
		WHERE id = $1
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
func (model *BookmarkRepo) GetBookmarkMarkdown(id types.BookmarkId) (string, error) {
	rows, err := model.Pool.Query(context.Background(), `
		SELECT ai_markdown
		FROM library_contents
		WHERE id = $1
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

func (model *BookmarkRepo) GetFullBookmark(id types.BookmarkId) (*BookmarkWithContent, error) {
	rows, err := model.Pool.Query(context.Background(), `
	SELECT
	li.id,
		li.user_id,
		li.title,
		li.link,
		li.source,
		lc.excerpt,
		lc.content,
		li.image_url,
		lc.article_lang,
		li.site_name,
		li.created_at,
		lc.published_time
	FROM library_items li
	JOIN library_contents lc ON li.id = lc.id
	WHERE li.id = $1`, id)

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

	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "*/*")
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

// checkRateLimit checks if a user has exceeded their bookmark limits
func (model *BookmarkRepo) checkRateLimit(user *User) error {
	// For unverified users, check total bookmark count (not daily)
	if !user.EmailVerified {
		row := model.Pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM library_items WHERE user_id = $1
		`, user.ID)

		var totalCount int
		if err := row.Scan(&totalCount); err != nil {
			return fmt.Errorf("check total bookmark count for unverified user: %w", err)
		}

		if totalCount >= UnverifiedUserTotalLimit {
			return errors.ErrUnverifiedUserLimitExceeded
		}

		return nil
	}

	// For verified users, check daily limit
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	row := model.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM library_items 
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`, user.ID, today, tomorrow)

	var count int
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("check daily rate limit: %w", err)
	}

	var limit int
	if user.IsSubscriptionPremium() {
		limit = PremiumUserDailyLimit
	} else {
		limit = FreeUserDailyLimit
	}

	if count >= limit {
		return errors.ErrDailyLimitExceeded
	}

	return nil
}

// GetRemainingBookmarks returns the number of bookmarks a user can still create
func (model *BookmarkRepo) GetRemainingBookmarks(user *User) (int, error) {
	// For unverified users, return total remaining bookmarks (lifetime limit)
	if !user.EmailVerified {
		row := model.Pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM library_items WHERE user_id = $1
		`, user.ID)

		var totalCount int
		if err := row.Scan(&totalCount); err != nil {
			return 0, fmt.Errorf("get total bookmarks for unverified user: %w", err)
		}

		remaining := max(UnverifiedUserTotalLimit-totalCount, 0)
		return remaining, nil
	}

	// For verified users, return daily remaining bookmarks
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	row := model.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM library_items 
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3
	`, user.ID, today, tomorrow)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("get remaining bookmarks: %w", err)
	}

	var limit int
	if user.IsSubscriptionPremium() {
		limit = PremiumUserDailyLimit
	} else {
		limit = FreeUserDailyLimit
	}

	remaining := max(limit-count, 0)

	return remaining, nil
}

// CheckAndIncrementAIQuestionLimit checks if the user can ask another AI question
// and increments the count if they can. Returns an error if limit is exceeded.
func (model *BookmarkRepo) CheckAndIncrementAIQuestionLimit(ctx context.Context, user *User) error {
	// Get today's date
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Determine the user's limit
	var limit int
	if user.IsSubscriptionPremium() {
		limit = PremiumUserDailyAIQuestions
	} else {
		limit = FreeUserDailyAIQuestions
	}

	// Use a transaction to ensure atomicity
	tx, err := model.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get or create today's record
	var currentCount int
	err = tx.QueryRow(ctx, `
		INSERT INTO daily_ai_limits (user_id, day, question_count)
		VALUES ($1, $2, 1)
		ON CONFLICT (user_id, day) DO UPDATE
		SET question_count = daily_ai_limits.question_count + 1,
		    updated_at = NOW()
		RETURNING question_count
	`, user.ID, today).Scan(&currentCount)

	if err != nil {
		return fmt.Errorf("increment AI question count: %w", err)
	}

	// Check if the limit has been exceeded
	if currentCount > limit {
		// Rollback the increment
		tx.Rollback(ctx)
		return fmt.Errorf("daily AI question limit exceeded (%d/%d)", currentCount-1, limit)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetRemainingAIQuestions returns how many AI questions the user has left today
func (model *BookmarkRepo) GetRemainingAIQuestions(user *User) (int, error) {
	// Get today's date
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Determine the user's limit
	var limit int
	if user.IsSubscriptionPremium() {
		limit = PremiumUserDailyAIQuestions
	} else {
		limit = FreeUserDailyAIQuestions
	}

	// Get current count for today
	var count int
	err := model.Pool.QueryRow(context.Background(), `
		SELECT question_count FROM daily_ai_limits
		WHERE user_id = $1 AND day = $2
	`, user.ID, today).Scan(&count)

	if err != nil {
		if err == pgx.ErrNoRows {
			// No record yet, user has full limit
			return limit, nil
		}
		return 0, fmt.Errorf("get AI question count: %w", err)
	}

	remaining := max(limit-count, 0)
	return remaining, nil
}
