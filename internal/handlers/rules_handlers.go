package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"MrRSS/internal/models"
)

// RuleCondition represents a condition in a rule
type RuleCondition struct {
	ID       int64    `json:"id"`
	Logic    string   `json:"logic"`    // "and", "or" (null for first condition)
	Negate   bool     `json:"negate"`   // NOT modifier for this condition
	Field    string   `json:"field"`    // "feed_name", "feed_category", "article_title", etc.
	Operator string   `json:"operator"` // "contains", "exact"
	Value    string   `json:"value"`    // Single value for text/date fields
	Values   []string `json:"values"`   // Multiple values for feed_name and feed_category
}

// Rule represents an automation rule
type Rule struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	Enabled    bool            `json:"enabled"`
	Conditions []RuleCondition `json:"conditions"`
	Actions    []string        `json:"actions"` // "favorite", "unfavorite", "hide", "unhide", "mark_read", "mark_unread"
}

// ApplyRuleRequest represents the request body for applying a rule
type ApplyRuleRequest struct {
	Rule
}

// ApplyRuleResponse represents the response for rule application
type ApplyRuleResponse struct {
	Success  bool `json:"success"`
	Affected int  `json:"affected"`
}

// HandleApplyRule applies a rule to matching articles
func (h *Handler) HandleApplyRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(rule.Actions) == 0 {
		http.Error(w, "No actions specified", http.StatusBadRequest)
		return
	}

	// Get all articles from database
	articles, err := h.DB.GetArticles("", 0, "", true, 50000, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get feeds for category lookup
	feeds, err := h.DB.GetFeeds()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a map of feed ID to category and title
	feedCategories := make(map[int64]string)
	feedTitles := make(map[int64]string)
	for _, feed := range feeds {
		feedCategories[feed.ID] = feed.Category
		feedTitles[feed.ID] = feed.Title
	}

	// Apply rule to matching articles
	affected := 0
	for _, article := range articles {
		// Check if article matches conditions
		if matchesRuleConditions(article, rule.Conditions, feedCategories) {
			// Apply actions
			for _, action := range rule.Actions {
				if err := h.applyAction(article.ID, action); err != nil {
					log.Printf("Error applying action %s to article %d: %v", action, article.ID, err)
					continue
				}
			}
			affected++
		}
	}

	response := ApplyRuleResponse{
		Success:  true,
		Affected: affected,
	}
	json.NewEncoder(w).Encode(response)
}

// matchesRuleConditions checks if an article matches the rule conditions
func matchesRuleConditions(article models.Article, conditions []RuleCondition, feedCategories map[int64]string) bool {
	// If no conditions, apply to all articles
	if len(conditions) == 0 {
		return true
	}

	result := evaluateRuleCondition(article, conditions[0], feedCategories)

	for i := 1; i < len(conditions); i++ {
		condition := conditions[i]
		conditionResult := evaluateRuleCondition(article, condition, feedCategories)

		switch condition.Logic {
		case "and":
			result = result && conditionResult
		case "or":
			result = result || conditionResult
		}
	}

	return result
}

// evaluateRuleCondition evaluates a single rule condition
func evaluateRuleCondition(article models.Article, condition RuleCondition, feedCategories map[int64]string) bool {
	var result bool

	switch condition.Field {
	case "feed_name":
		result = matchRuleMultiSelect(article.FeedTitle, condition.Values, condition.Value)

	case "feed_category":
		feedCategory := feedCategories[article.FeedID]
		result = matchRuleMultiSelect(feedCategory, condition.Values, condition.Value)

	case "article_title":
		if condition.Value == "" {
			result = true
		} else {
			lowerValue := strings.ToLower(condition.Value)
			lowerTitle := strings.ToLower(article.Title)
			if condition.Operator == "exact" {
				result = lowerTitle == lowerValue
			} else {
				result = strings.Contains(lowerTitle, lowerValue)
			}
		}

	case "published_after":
		if condition.Value == "" {
			result = true
		} else {
			afterDate, err := time.Parse("2006-01-02", condition.Value)
			if err != nil {
				result = true
			} else {
				result = article.PublishedAt.After(afterDate) || article.PublishedAt.Equal(afterDate)
			}
		}

	case "published_before":
		if condition.Value == "" {
			result = true
		} else {
			beforeDate, err := time.Parse("2006-01-02", condition.Value)
			if err != nil {
				result = true
			} else {
				articleDateOnly := article.PublishedAt.UTC().Truncate(24 * time.Hour)
				beforeDateOnly := beforeDate.Truncate(24 * time.Hour)
				result = !articleDateOnly.After(beforeDateOnly)
			}
		}

	case "is_read":
		if condition.Value == "" {
			result = true
		} else {
			wantRead := condition.Value == "true"
			result = article.IsRead == wantRead
		}

	case "is_favorite":
		if condition.Value == "" {
			result = true
		} else {
			wantFavorite := condition.Value == "true"
			result = article.IsFavorite == wantFavorite
		}

	default:
		result = true
	}

	// Apply NOT modifier
	if condition.Negate {
		return !result
	}
	return result
}

// matchRuleMultiSelect checks if fieldValue matches any of the selected values
func matchRuleMultiSelect(fieldValue string, values []string, singleValue string) bool {
	if len(values) > 0 {
		lowerField := strings.ToLower(fieldValue)
		for _, val := range values {
			if strings.Contains(lowerField, strings.ToLower(val)) {
				return true
			}
		}
		return false
	} else if singleValue != "" {
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(singleValue))
	}
	return true
}

// applyAction applies an action to an article
func (h *Handler) applyAction(articleID int64, action string) error {
	switch action {
	case "favorite":
		return h.DB.SetArticleFavorite(articleID, true)
	case "unfavorite":
		return h.DB.SetArticleFavorite(articleID, false)
	case "hide":
		return h.DB.SetArticleHidden(articleID, true)
	case "unhide":
		return h.DB.SetArticleHidden(articleID, false)
	case "mark_read":
		return h.DB.MarkArticleRead(articleID, true)
	case "mark_unread":
		return h.DB.MarkArticleRead(articleID, false)
	default:
		log.Printf("Unknown action: %s", action)
		return nil
	}
}
