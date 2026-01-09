package media

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"MrRSS/internal/cache"
	"MrRSS/internal/handlers/core"
	"MrRSS/internal/utils"
)

// validateMediaURL validates that the URL is HTTP/HTTPS and properly formatted
func validateMediaURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return errors.New("invalid URL format")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL must use HTTP or HTTPS")
	}

	return nil
}

// proxyImagesInHTML replaces image URLs in HTML with proxied versions
func proxyImagesInHTML(htmlContent, referer string) string {
	if htmlContent == "" || referer == "" {
		return htmlContent
	}

	// Parse the referer URL once for resolving relative URLs
	baseURL, err := url.Parse(referer)
	if err != nil {
		log.Printf("Failed to parse referer URL: %v", err)
		return htmlContent
	}

	// Use regex to find and replace img src attributes
	// This handles various formats: src="url", src='url', src=url (unquoted)
	re := regexp.MustCompile(`<img[^>]*src\s*=\s*(?:['"]\s*)?([^'"\s>]+)(?:\s*['"])?[^>]*>`)
	htmlContent = re.ReplaceAllStringFunc(htmlContent, func(match string) string {
		// Extract the src URL from the match
		re := regexp.MustCompile(`src\s*=\s*(?:['"]\s*)?([^'"\s>]+)(?:\s*['"])?`)
		srcMatch := re.FindStringSubmatch(match)
		if len(srcMatch) < 2 {
			return match // No valid src found, return unchanged
		}

		srcURL := srcMatch[1]

		// Skip data URLs, blob URLs, and already proxied URLs
		if strings.HasPrefix(srcURL, "data:") ||
			strings.HasPrefix(srcURL, "blob:") ||
			strings.Contains(srcURL, "/api/media/proxy") {
			return match
		}

		// CRITICAL FIX: Decode HTML entities before processing the URL
		// HTML attributes contain &amp; which should be decoded to & before URL encoding
		// For example: ?key=val&amp;other=val becomes ?key=val&other=val
		srcURL = html.UnescapeString(srcURL)

		// Resolve relative URLs against the referer
		// Handles: images/photo.jpg, ./img.png, ../assets/image.gif, /static/img.png
		if !strings.HasPrefix(srcURL, "http://") && !strings.HasPrefix(srcURL, "https://") {
			parsedURL, err := url.Parse(srcURL)
			if err != nil {
				log.Printf("Failed to parse image URL %s: %v", srcURL, err)
				return match
			}
			srcURL = baseURL.ResolveReference(parsedURL).String()
		}

		// Build proxied URL
		proxyURL := fmt.Sprintf("/api/media/proxy?url=%s&referer=%s",
			url.QueryEscape(srcURL),
			url.QueryEscape(referer))

		// Replace the src attribute
		return strings.Replace(match, srcMatch[0], fmt.Sprintf(`src="%s"`, proxyURL), 1)
	})

	return htmlContent
}

// HandleMediaProxy serves cached media or downloads and caches it
// HandleMediaProxy proxies media files with optional caching
// @Summary      Proxy media file
// @Description  Proxy and cache media files (images, videos, audio) from external URLs
// @Tags         media
// @Accept       json
// @Produce      application/octet-stream
// @Param        url      query     string  true  "Media URL to proxy"
// @Param        referer  query     string  false  "Referer URL for hotlink protection"
// @Success      200  {file}  file  "Media file"
// @Failure      400  {object}  map[string]string  "Bad request (missing or invalid URL)"
// @Failure      403  {object}  map[string]string  "Media proxy is disabled"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /media/proxy [get]
func HandleMediaProxy(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get URL from query parameter
	mediaURL := r.URL.Query().Get("url")
	if mediaURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	// Validate mediaURL (must be HTTP/HTTPS and valid format)
	if err := validateMediaURL(mediaURL); err != nil {
		http.Error(w, "Invalid url parameter: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check if media cache is enabled
	mediaCacheEnabled, _ := h.DB.GetSetting("media_cache_enabled")
	mediaProxyFallback, _ := h.DB.GetSetting("media_proxy_fallback")

	// If neither cache nor fallback is enabled, return error
	if mediaCacheEnabled != "true" && mediaProxyFallback != "true" {
		http.Error(w, "Media proxy is disabled", http.StatusForbidden)
		return
	}

	// Get optional referer from query parameter
	referer := r.URL.Query().Get("referer")

	// Try cache first if enabled
	if mediaCacheEnabled == "true" {
		// Get media cache directory
		cacheDir, err := utils.GetMediaCacheDir()
		if err != nil {
			log.Printf("Failed to get media cache directory: %v", err)
			// Continue to fallback if enabled
		} else {
			// Initialize media cache
			mediaCache, err := cache.NewMediaCache(cacheDir)
			if err != nil {
				log.Printf("Failed to initialize media cache: %v", err)
				// Continue to fallback if enabled
			} else {
				// Get media (from cache or download)
				data, contentType, err := mediaCache.Get(mediaURL, referer)
				if err == nil {
					// Success! Serve from cache
					w.Header().Set("Content-Type", contentType)
					w.Header().Set("Content-Length", strconv.Itoa(len(data)))
					w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
					w.Header().Set("X-Media-Source", "cache")
					w.Write(data)
					return
				}
				log.Printf("Cache failed for %s: %v, trying fallback", mediaURL, err)
			}
		}
	}

	// Fallback: Direct proxy if enabled
	if mediaProxyFallback == "true" {
		err := proxyMediaDirectly(mediaURL, referer, w)
		if err == nil {
			return // Success
		}
		log.Printf("Direct proxy failed for %s: %v", mediaURL, err)
	}

	// All methods failed
	http.Error(w, "Failed to fetch media", http.StatusInternalServerError)
}

// HandleMediaCacheCleanup performs manual cleanup of media cache
// @Summary      Cleanup media cache
// @Description  Clean up the media cache by age and size
// @Tags         media
// @Accept       json
// @Produce      json
// @Param        all  query     bool  false  "Clean all files (ignores age/size settings)"  default(false)
// @Success      200  {object}  map[string]interface{}  "Cleanup result (success, files_cleaned)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /media/cache/cleanup [post]
func HandleMediaCacheCleanup(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get media cache directory
	cacheDir, err := utils.GetMediaCacheDir()
	if err != nil {
		log.Printf("Failed to get media cache directory: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Initialize media cache
	mediaCache, err := cache.NewMediaCache(cacheDir)
	if err != nil {
		log.Printf("Failed to initialize media cache: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if this is a manual cleanup (clean all) or automatic cleanup (respect settings)
	cleanAll := r.URL.Query().Get("all") == "true"

	var maxAgeDays int
	var maxSizeMB int

	if cleanAll {
		// Manual cleanup: remove all files
		maxAgeDays = 0
		maxSizeMB = 0 // Will skip size-based cleanup
	} else {
		// Automatic cleanup: use settings
		maxAgeDaysStr, _ := h.DB.GetSetting("media_cache_max_age_days")
		maxSizeMBStr, _ := h.DB.GetSetting("media_cache_max_size_mb")

		maxAgeDays, err = strconv.Atoi(maxAgeDaysStr)
		if err != nil || maxAgeDays < 0 {
			maxAgeDays = 7 // Default
		}

		maxSizeMB, err = strconv.Atoi(maxSizeMBStr)
		if err != nil || maxSizeMB <= 0 {
			maxSizeMB = 100 // Default
		}
	}

	// Cleanup by age
	ageCount, err := mediaCache.CleanupOldFiles(maxAgeDays)
	if err != nil {
		log.Printf("Failed to cleanup old media files: %v", err)
	}

	// Cleanup by size (only for automatic cleanup)
	sizeCount := 0
	if !cleanAll {
		sizeCount, err = mediaCache.CleanupBySize(maxSizeMB)
		if err != nil {
			log.Printf("Failed to cleanup media files by size: %v", err)
		}
	}

	totalCleaned := ageCount + sizeCount
	log.Printf("Media cache cleanup: removed %d files (clean_all: %v)", totalCleaned, cleanAll)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"success":       true,
		"files_cleaned": totalCleaned,
	}
	json.NewEncoder(w).Encode(response)
}

// HandleWebpageProxy proxies webpage content to bypass CORS restrictions in iframes
// @Summary      Proxy webpage content
// @Description  Proxy webpage HTML content and rewrite resource URLs to bypass CORS restrictions
// @Tags         media
// @Accept       json
// @Produce      html
// @Param        url  query     string  true  "Webpage URL to proxy"
// @Success      200  {string}  string  "Webpage HTML content"
// @Failure      400  {object}  map[string]string  "Bad request (missing or invalid URL)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /media/proxy-webpage [get]
func HandleWebpageProxy(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get URL from query parameter
	webpageURL := r.URL.Query().Get("url")
	if webpageURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	// Validate webpageURL (must be HTTP/HTTPS and valid format)
	if err := validateMediaURL(webpageURL); err != nil {
		http.Error(w, "Invalid url parameter: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Create HTTP client with proxy settings if enabled
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Check if proxy is enabled and configure client
	proxyEnabled, _ := h.DB.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSetting("proxy_type")
		proxyHost, _ := h.DB.GetSetting("proxy_host")
		proxyPort, _ := h.DB.GetSetting("proxy_port")
		proxyUsername, _ := h.DB.GetSetting("proxy_username")
		proxyPassword, _ := h.DB.GetSetting("proxy_password")

		proxyURLStr := utils.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		if proxyURLStr != "" {
			proxyURL, err := url.Parse(proxyURLStr)
			if err != nil {
				log.Printf("Failed to parse proxy URL: %v", err)
			} else {
				transport := &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				}
				client.Transport = transport
			}
		}
	}

	// Create request to the target URL
	req, err := http.NewRequest("GET", webpageURL, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Set User-Agent to mimic a regular browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Forward some headers from the original request
	if referer := r.Header.Get("Referer"); referer != "" {
		req.Header.Set("Referer", referer)
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch webpage %s: %v", webpageURL, err)
		http.Error(w, "Failed to fetch webpage", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Webpage returned status %d: %s", resp.StatusCode, webpageURL)
		http.Error(w, "Webpage returned error", resp.StatusCode)
		return
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/html; charset=utf-8"
	}

	// Read the entire response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		http.Error(w, "Failed to read webpage content", http.StatusInternalServerError)
		return
	}

	// If this is HTML content, rewrite all resource URLs
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		bodyBytes = rewriteHTMLContent(bodyBytes, webpageURL)
	}

	// Set response headers to allow framing and remove CORS restrictions
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Frame-Options", "SAMEORIGIN") // Allow framing from same origin
	w.Header().Set("Content-Security-Policy", "")   // Remove CSP to allow resources
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Length", strconv.Itoa(len(bodyBytes)))

	// Write modified response body
	_, err = w.Write(bodyBytes)
	if err != nil {
		log.Printf("Failed to write response body: %v", err)
	}
}

// rewriteHTMLContent rewrites HTML to proxy all external resources
func rewriteHTMLContent(bodyBytes []byte, baseURL string) []byte {
	// Parse the base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("Failed to parse base URL: %v", err)
		return bodyBytes
	}

	baseOrigin := parsedURL.Scheme + "://" + parsedURL.Host

	// Convert to string for manipulation
	content := string(bodyBytes)

	// Inject our script FIRST - before anything else
	// This script must run before any other scripts to intercept API calls early
	interceptionScript := `<script>
	// Use immediately-invoked function with strict error suppression
	(function() {
		'use strict';
		const ORIGINAL_BASE_URL = ` + fmt.Sprintf("'%s'", baseURL) + `;
		const PROXY_ORIGIN = window.location.origin;

		// DEBUG: Log that interceptor is loaded
		console.log('[Proxy] Interceptor loaded for:', ORIGINAL_BASE_URL);

		// Override History API BEFORE anything else with try-catch to suppress ALL errors
		try {
			const originalPushState = History.prototype.pushState;
			History.prototype.pushState = function(state, title, url) {
				try {
					if (url && typeof url === 'string' && (url.indexOf('http://') === 0 || url.indexOf('https://') === 0)) {
						// Silently block - don't even log to avoid console spam
						return undefined;
					}
				} catch(e) { /* Suppress all errors */ }
				try {
					return originalPushState.call(this, state, title, url);
				} catch(e) { /* Suppress errors from original call */ }
			};
		} catch(e) { /* Suppress errors during override */ }

		try {
			const originalReplaceState = History.prototype.replaceState;
			History.prototype.replaceState = function(state, title, url) {
				try {
					if (url && typeof url === 'string' && (url.indexOf('http://') === 0 || url.indexOf('https://') === 0)) {
						// Silently block
						return undefined;
					}
				} catch(e) { /* Suppress all errors */ }
				try {
					return originalReplaceState.call(this, state, title, url);
				} catch(e) { /* Suppress errors from original call */ }
			};
		} catch(e) { /* Suppress errors during override */ }

		// Also override on window.history for direct access
		try {
			if (window.history && window.history.pushState) {
				const originalPushState = window.history.pushState;
				window.history.pushState = function(state, title, url) {
					try {
						if (url && typeof url === 'string' && (url.indexOf('http://') === 0 || url.indexOf('https://') === 0)) {
							return undefined;
						}
					} catch(e) { }
					try {
						return originalPushState.call(this, state, title, url);
					} catch(e) { }
				};
			}
		} catch(e) { }

		try {
			if (window.history && window.history.replaceState) {
				const originalReplaceState = window.history.replaceState;
				window.history.replaceState = function(state, title, url) {
					try {
						if (url && typeof url === 'string' && (url.indexOf('http://') === 0 || url.indexOf('https://') === 0)) {
							return undefined;
						}
					} catch(e) { }
					try {
						return originalReplaceState.call(this, state, title, url);
					} catch(e) { }
				};
			}
		} catch(e) { }

		// Helper function to resolve relative URLs
		function resolveRelativeURL(url) {
			try {
				// If already absolute, return as-is
				if (url.indexOf('http://') === 0 || url.indexOf('https://') === 0) {
					return url;
				}
				// Protocol-relative URL
				if (url.indexOf('//') === 0) {
					return 'https:' + url;
				}
				// Relative URL - resolve against base URL
				const base = new URL(ORIGINAL_BASE_URL);
				return new URL(url, base).href;
			} catch(e) {
				return url;
			}
		}

		// List of domains to skip proxying (analytics, ads, tracking)
		const SKIP_PROXY_DOMAINS = [
			'google-analytics.com',
			'googletagmanager.com',
			'googlesyndication.com',
			'googleadservices.com',
			'doubleclick.net',
			'facebook.com/tr',
			'connect.facebook.net',
			'analytics.twitter.com',
			't.co',
			'adform.net',
			'adnxs.com',
			'rubiconproject.com',
			'pubmatic.com',
			'criteo.com',
			'crwdcntrl.net',
			'cookielaw.org',
			'onetrust.com',
			'clarity.ms',
			'bing.com'
		];

		// Helper function to check if URL should be skipped
		function shouldSkipProxy(url) {
			try {
				const urlObj = new URL(url);
				const hostname = urlObj.hostname.toLowerCase();
				return SKIP_PROXY_DOMAINS.some(domain =>
					hostname === domain || hostname.endsWith('.' + domain)
				);
			} catch(e) {
				return false;
			}
		}

		// Intercept fetch() with error suppression
		try {
			const originalFetch = window.fetch;
			window.fetch = function(input, ...args) {
				let modifiedInput = input;
				try {
					let url = input;
					// Handle Request objects
					if (input && typeof input === 'object' && input.url) {
						url = input.url;
					}
					if (typeof url === 'string') {
						// Resolve relative URLs to absolute
						const absoluteUrl = resolveRelativeURL(url);

						// Only intercept external URLs (not our own proxy)
						if (absoluteUrl.indexOf(PROXY_ORIGIN) !== 0) {
							// Skip known analytics/ad/tracking domains
							if (shouldSkipProxy(absoluteUrl)) {
								// Don't intercept - let it fail naturally
								return originalFetch.call(this, input, ...args);
							}

							// Reduce noise - only log important requests
							if (!absoluteUrl.includes('/analytics') && !absoluteUrl.includes('/collect') && !absoluteUrl.includes('/rum')) {
								console.log('[Proxy] Intercepting fetch:', url, '->', absoluteUrl);
							}
							try {
								const proxyUrl = PROXY_ORIGIN + '/api/webpage/resource?url=' + encodeURIComponent(absoluteUrl) + '&referer=' + encodeURIComponent(ORIGINAL_BASE_URL);
								if (input && typeof input === 'object' && input.url) {
									modifiedInput = new Request(proxyUrl, input);
								} else {
									modifiedInput = proxyUrl;
								}
							} catch(e) { }
						}
					}
				} catch(e) { }
				try {
					return originalFetch.call(this, modifiedInput, ...args);
				} catch(e) {
					return Promise.reject(e);
				}
			};
			console.log('[Proxy] Fetch interceptor installed');
		} catch(e) { }

		// Intercept XMLHttpRequest with error suppression
		try {
			const originalXHROpen = XMLHttpRequest.prototype.open;
			XMLHttpRequest.prototype.open = function(method, url, ...args) {
				let modifiedUrl = url;
				try {
					if (typeof url === 'string') {
						// Resolve relative URLs to absolute
						const absoluteUrl = resolveRelativeURL(url);

						// Only intercept external URLs (not our own proxy)
						if (absoluteUrl.indexOf(PROXY_ORIGIN) !== 0) {
							// Skip known analytics/ad/tracking domains
							if (shouldSkipProxy(absoluteUrl)) {
								// Don't intercept - let it fail naturally
								return originalXHROpen.call(this, method, url, ...args);
							}

							// Reduce noise - only log important requests
							if (!absoluteUrl.includes('/analytics') && !absoluteUrl.includes('/collect') && !absoluteUrl.includes('/rum')) {
								console.log('[Proxy] Intercepting XHR:', method, url, '->', absoluteUrl);
							}
							try {
								modifiedUrl = PROXY_ORIGIN + '/api/webpage/resource?url=' + encodeURIComponent(absoluteUrl) + '&referer=' + encodeURIComponent(ORIGINAL_BASE_URL);
							} catch(e) { }
						}
					}
				} catch(e) { }
				try {
					return originalXHROpen.call(this, method, modifiedUrl, ...args);
				} catch(e) {
					throw e;
				}
			};
			console.log('[Proxy] XHR interceptor installed');
		} catch(e) { }
	})();
	</script>`

	// Add meta tags to block manifest and other external resource requests
	metaTags := `<meta name="manifest" content=""><link rel="manifest" href="about:blank">`

	// Insert a base tag to handle relative URLs
	baseTag := fmt.Sprintf("<base href=\"%s\">", baseOrigin)

	// Find <head> tag and insert our interception script FIRST, then base tag, then meta tags
	headIndex := strings.Index(strings.ToLower(content), "<head>")
	if headIndex == -1 {
		// If no <head>, look for <html>
		htmlIndex := strings.Index(strings.ToLower(content), "<html>")
		if htmlIndex != -1 {
			htmlEndIndex := htmlIndex + strings.Index(content[htmlIndex:], ">") + 1
			content = content[:htmlEndIndex] + "<head>" + interceptionScript + baseTag + metaTags + "</head>" + content[htmlEndIndex:]
		}
	} else {
		headEndIndex := headIndex + strings.Index(content[headIndex:], ">") + 1
		// Insert interception script FIRST, then base tag, then meta tags
		content = content[:headEndIndex] + interceptionScript + baseTag + metaTags + content[headEndIndex:]
	}

	// Rewrite script src attributes
	content = rewriteAttribute(content, "script", "src", baseURL)

	// Rewrite link href attributes (for stylesheets)
	content = rewriteLinkHref(content, baseURL)

	// Rewrite img src attributes
	content = rewriteAttribute(content, "img", "src", baseURL)

	// Rewrite iframe src attributes
	content = rewriteAttribute(content, "iframe", "src", baseURL)

	// Rewrite video src and poster attributes
	content = rewriteAttribute(content, "video", "src", baseURL)
	content = rewriteAttribute(content, "video", "poster", baseURL)

	// Rewrite audio src attributes
	content = rewriteAttribute(content, "audio", "src", baseURL)

	// Rewrite source src attributes (for video/audio)
	content = rewriteAttribute(content, "source", "src", baseURL)

	// Rewrite track src attributes
	content = rewriteAttribute(content, "track", "src", baseURL)

	// Rewrite embed src attributes
	content = rewriteAttribute(content, "embed", "src", baseURL)

	// Rewrite object data attributes
	content = rewriteAttribute(content, "object", "data", baseURL)

	// Rewrite action attributes in forms
	content = rewriteAttribute(content, "form", "action", baseURL)

	// Rewrite href attributes in anchor tags (for absolute URLs only)
	content = rewriteAnchorHref(content, baseURL)

	// Rewrite CSS in style tags
	content = rewriteStyleTags(content, baseURL)

	// Rewrite inline style attributes
	content = rewriteInlineStyles(content, baseURL)

	return []byte(content)
}

// rewriteAttribute rewrites a specific attribute in HTML tags
func rewriteAttribute(content, tag, attr, baseURL string) string {
	// Pattern matches: <tag attr="value"> or <tag attr='value'> or <tag attr=value>
	// This is a simplified pattern - may need refinement for edge cases
	pattern := fmt.Sprintf(`<%s[^>]*\s%s\s*=\s*(["']?)([^"'\s>]+)\1`, tag, attr)
	pattern = strings.ReplaceAll(pattern, `\1`, `\$1`) // Fix backreference
	re := regexp.MustCompile(pattern)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the attribute value
		subPattern := fmt.Sprintf(`\s%s\s*=\s*(["']?)([^"'\s>]+)\1`, attr)
		subPattern = strings.ReplaceAll(subPattern, `\1`, `\$1`) // Fix backreference
		subRe := regexp.MustCompile(subPattern)
		matches := subRe.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		urlValue := matches[2]

		// Skip data: URLs, blob: URLs, and already proxied URLs
		if strings.HasPrefix(urlValue, "data:") ||
			strings.HasPrefix(urlValue, "blob:") ||
			strings.HasPrefix(urlValue, "/api/") ||
			strings.HasPrefix(urlValue, "#") {
			return match
		}

		// Resolve relative URLs
		resolvedURL := resolveURL(urlValue, baseURL)

		// Create proxied URL
		proxiedURL := fmt.Sprintf("/api/webpage/resource?url=%s&referer=%s",
			url.QueryEscape(resolvedURL),
			url.QueryEscape(baseURL))

		// Replace the URL in the match
		return strings.Replace(match, urlValue, proxiedURL, 1)
	})
}

// rewriteLinkHref rewrites href attributes in link tags
func rewriteLinkHref(content, baseURL string) string {
	// Match link tags with rel="stylesheet" or rel="icon" etc.
	pattern := `<link[^>]*\shref\s*=\s*(["']?)([^"'\s>]+)\1[^>]*>`
	pattern = strings.ReplaceAll(pattern, `\1`, `\$1`) // Fix backreference
	re := regexp.MustCompile(pattern)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the href value
		subPattern := `\shref\s*=\s*(["']?)([^"'\s>]+)\1`
		subPattern = strings.ReplaceAll(subPattern, `\1`, `\$1`) // Fix backreference
		subRe := regexp.MustCompile(subPattern)
		matches := subRe.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		urlValue := matches[2]

		// Skip data: URLs, blob: URLs, and already proxied URLs
		if strings.HasPrefix(urlValue, "data:") ||
			strings.HasPrefix(urlValue, "blob:") ||
			strings.HasPrefix(urlValue, "/api/") ||
			strings.HasPrefix(urlValue, "#") {
			return match
		}

		// Resolve relative URLs
		resolvedURL := resolveURL(urlValue, baseURL)

		// Create proxied URL
		proxiedURL := fmt.Sprintf("/api/webpage/resource?url=%s&referer=%s",
			url.QueryEscape(resolvedURL),
			url.QueryEscape(baseURL))

		// Replace the URL in the match
		return strings.Replace(match, urlValue, proxiedURL, 1)
	})
}

// rewriteAnchorHref rewrites href attributes in anchor tags
func rewriteAnchorHref(content, baseURL string) string {
	pattern := `<a[^>]*\shref\s*=\s*(["']?)([^"'\s>]+)\1[^>]*>`
	pattern = strings.ReplaceAll(pattern, `\1`, `\$1`) // Fix backreference
	re := regexp.MustCompile(pattern)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		subPattern := `\shref\s*=\s*(["']?)([^"'\s>]+)\1`
		subPattern = strings.ReplaceAll(subPattern, `\1`, `\$1`) // Fix backreference
		subRe := regexp.MustCompile(subPattern)
		matches := subRe.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		urlValue := matches[2]

		// Skip relative URLs, anchors, and already proxied URLs
		if !strings.HasPrefix(urlValue, "http://") && !strings.HasPrefix(urlValue, "https://") {
			return match
		}

		if strings.HasPrefix(urlValue, "/api/") {
			return match
		}

		// Keep absolute URLs as-is but add target="_blank" for external links
		if !strings.Contains(match, "target=") {
			match = strings.Replace(match, ">", ` target="_blank" rel="noopener noreferrer">`, 1)
		}

		return match
	})
}

// rewriteStyleTags rewrites CSS URLs in <style> tags
func rewriteStyleTags(content, baseURL string) string {
	pattern := `<style[^>]*>(.*?)</style>`
	re := regexp.MustCompile(`(?is)` + pattern)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the CSS content
		subRe := regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)
		matches := subRe.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		cssContent := matches[1]
		// Rewrite @font-face rules first
		cssContent = rewriteFontFaceRules(cssContent, baseURL)
		// Then rewrite all other url() references
		rewrittenCSS := rewriteCSSURLs(cssContent, baseURL)

		return strings.Replace(match, cssContent, rewrittenCSS, 1)
	})
}

// rewriteInlineStyles rewrites style attribute values
func rewriteInlineStyles(content, baseURL string) string {
	pattern := `style\s*=\s*(["'])(.*?)\1`
	pattern = strings.ReplaceAll(pattern, `\1`, `\$1`) // Fix backreference
	re := regexp.MustCompile(pattern)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		subPattern := `style\s*=\s*(["'])(.*?)\1`
		subPattern = strings.ReplaceAll(subPattern, `\1`, `\$1`) // Fix backreference
		subRe := regexp.MustCompile(subPattern)
		matches := subRe.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		quote := matches[1]
		styleContent := matches[2]
		// Rewrite @font-face rules first
		styleContent = rewriteFontFaceRules(styleContent, baseURL)
		// Then rewrite all other url() references
		rewrittenStyle := rewriteCSSURLs(styleContent, baseURL)

		return fmt.Sprintf(`style=%s%s%s`, quote, rewrittenStyle, quote)
	})
}

// rewriteCSSURLs rewrites url() references in CSS
func rewriteCSSURLs(css, baseURL string) string {
	// Match url(...) patterns in CSS
	pattern := `url\((['"]?)([^'")]+)\1\)`
	pattern = strings.ReplaceAll(pattern, `\1`, `\$1`) // Fix backreference
	re := regexp.MustCompile(pattern)

	return re.ReplaceAllStringFunc(css, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			return match
		}

		urlValue := subMatches[2]

		// Skip data: URLs and already proxied URLs
		if strings.HasPrefix(urlValue, "data:") ||
			strings.HasPrefix(urlValue, "/api/") {
			return match
		}

		// Resolve relative URLs
		resolvedURL := resolveURL(urlValue, baseURL)

		// Create proxied URL
		proxiedURL := fmt.Sprintf("/api/webpage/resource?url=%s&referer=%s",
			url.QueryEscape(resolvedURL),
			url.QueryEscape(baseURL))

		return fmt.Sprintf(`url(%s)`, proxiedURL)
	})
}

// rewriteFontFaceRules rewrites @font-face rules in CSS
func rewriteFontFaceRules(css, baseURL string) string {
	// Match @font-face blocks
	pattern := `@font-face\s*\{[^}]*\}`
	re := regexp.MustCompile(`(?is)` + pattern)

	return re.ReplaceAllStringFunc(css, func(match string) string {
		// Rewrite url() within this @font-face block
		return rewriteCSSURLs(match, baseURL)
	})
}

// resolveURL resolves a URL relative to a base URL
func resolveURL(urlStr, baseURL string) string {
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}

	return parsedBase.ResolveReference(parsedURL).String()
}

// HandleWebpageResource proxies individual webpage resources (CSS, JS, images, etc.)
// @Summary      Proxy webpage resource
// @Description  Proxy individual resources (CSS, JS, images, fonts, etc.) from a webpage
// @Tags         media
// @Accept       json
// @Produce      application/octet-stream
// @Param        url      query     string  true  "Resource URL to proxy"
// @Param        referer  query     string  true  "Referer URL for the webpage"
// @Success      200  {file}  file  "Resource file"
// @Failure      400  {object}  map[string]string  "Bad request (missing or invalid URL)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /webpage/resource [get]
func HandleWebpageResource(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	// Handle CORS preflight requests
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get URL from query parameter
	resourceURL := r.URL.Query().Get("url")
	if resourceURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	// Get referer from query parameter
	referer := r.URL.Query().Get("referer")
	if referer == "" {
		http.Error(w, "Missing referer parameter", http.StatusBadRequest)
		return
	}

	// Validate URLs
	if err := validateMediaURL(resourceURL); err != nil {
		log.Printf("Invalid URL validation failed for %s: %v", resourceURL, err)
		http.Error(w, "Invalid url parameter: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateMediaURL(referer); err != nil {
		log.Printf("Invalid referer validation failed for %s: %v", referer, err)
		http.Error(w, "Invalid referer parameter: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Create HTTP client with proxy settings if enabled
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Check if proxy is enabled and configure client
	proxyEnabled, _ := h.DB.GetSetting("proxy_enabled")
	if proxyEnabled == "true" {
		proxyType, _ := h.DB.GetSetting("proxy_type")
		proxyHost, _ := h.DB.GetSetting("proxy_host")
		proxyPort, _ := h.DB.GetSetting("proxy_port")
		proxyUsername, _ := h.DB.GetSetting("proxy_username")
		proxyPassword, _ := h.DB.GetSetting("proxy_password")

		proxyURLStr := utils.BuildProxyURL(proxyType, proxyHost, proxyPort, proxyUsername, proxyPassword)
		if proxyURLStr != "" {
			proxyURL, err := url.Parse(proxyURLStr)
			if err != nil {
				log.Printf("Failed to parse proxy URL: %v", err)
			} else {
				transport := &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				}
				client.Transport = transport
			}
		}
	}

	// Create request to the resource URL
	var req *http.Request
	var err error

	// For POST requests, read the body
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Failed to read request body: %v", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		req, err = http.NewRequest("POST", resourceURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}
		// Forward content type
		if contentType := r.Header.Get("Content-Type"); contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
	} else {
		req, err = http.NewRequest("GET", resourceURL, nil)
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "*/*")

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch resource %s: %v", resourceURL, err)
		http.Error(w, "Failed to fetch resource", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status - allow 200, 201, 202, 203, 204, 206
	if resp.StatusCode < 200 || resp.StatusCode > 206 {
		log.Printf("Resource returned status %d for %s (method: %s)", resp.StatusCode, resourceURL, r.Method)
		http.Error(w, "Resource returned error", resp.StatusCode)
		return
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")

	// Copy headers from the response, excluding problematic ones
	for key, values := range resp.Header {
		// Skip headers that might cause issues
		if key == "Content-Security-Policy" ||
			key == "X-Frame-Options" ||
			key == "Set-Cookie" ||
			key == "Access-Control-Allow-Origin" ||
			key == "Content-Length" { // We'll recalculate this
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set CORS headers to allow loading from the same origin
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")

	// If this is a CSS file, rewrite URLs in it
	if strings.Contains(strings.ToLower(contentType), "text/css") {
		// Read the CSS content
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Failed to read CSS content: %v", err)
			http.Error(w, "Failed to read CSS content", http.StatusInternalServerError)
			return
		}

		// Rewrite @font-face rules
		cssContent := rewriteFontFaceRules(string(bodyBytes), referer)
		// Rewrite all url() references
		cssContent = rewriteCSSURLs(cssContent, referer)

		// Update content length
		bodyBytes = []byte(cssContent)
		w.Header().Set("Content-Length", strconv.Itoa(len(bodyBytes)))

		// Write the modified CSS
		_, err = w.Write(bodyBytes)
		if err != nil {
			log.Printf("Failed to write CSS content: %v", err)
		}
		return
	}

	// For non-CSS files, stream directly
	// Stream the response directly to avoid loading large files into memory
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Failed to stream resource: %v", err)
	}
}

// proxyMediaDirectly proxies media directly without caching
func proxyMediaDirectly(mediaURL, referer string, w http.ResponseWriter) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to bypass anti-hotlinking
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	// Add additional headers
	// Note: Don't set Accept-Encoding - let Go's http.Transport handle it automatically
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = getContentTypeFromPath(mediaURL)
	}

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Header().Set("X-Media-Source", "direct-proxy")

	// Stream the response directly to avoid loading large files into memory
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to stream response: %w", err)
	}

	return nil
}

// HandleMediaCacheInfo returns information about the media cache
// HandleMediaCacheInfo returns information about the media cache
// @Summary      Get media cache info
// @Description  Get media cache statistics (size in MB)
// @Tags         media
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Cache info (cache_size_mb)"
// @Failure      500  {object}  map[string]string  "Internal server error"
// @Router       /media/cache/info [get]
func HandleMediaCacheInfo(h *core.Handler, w http.ResponseWriter, r *http.Request) {

	// Get media cache directory
	cacheDir, err := utils.GetMediaCacheDir()
	if err != nil {
		log.Printf("Failed to get media cache directory: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Initialize media cache
	mediaCache, err := cache.NewMediaCache(cacheDir)
	if err != nil {
		log.Printf("Failed to initialize media cache: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get cache size
	cacheSize, err := mediaCache.GetCacheSize()
	if err != nil {
		log.Printf("Failed to get cache size: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert to MB
	cacheSizeMB := float64(cacheSize) / (1024 * 1024)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"cache_size_mb": cacheSizeMB,
	}
	json.NewEncoder(w).Encode(response)
}

// getContentTypeFromPath determines content type from file extension
func getContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogg":
		return "video/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	default:
		return "application/octet-stream"
	}
}
