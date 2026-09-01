package researchsearch

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/searchprovidertest"
)

func TestBraveSearchProviderConformance(t *testing.T) {
	t.Parallel()
	searchprovidertest.Run(t, ProviderID, func(t *testing.T, fixtures []application.SearchResult) application.SearchProvider {
		client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if err := request.Context().Err(); err != nil {
				return nil, err
			}
			count, err := strconv.Atoi(request.URL.Query().Get("count"))
			if err != nil {
				return nil, err
			}
			bounded := fixtures
			if len(bounded) > count {
				bounded = bounded[:count]
			}
			payload := braveResponse{}
			for _, fixture := range bounded {
				item := braveResult{
					Title: fixture.Title, URL: fixture.Locator.String(), Description: fixture.Snippet,
				}
				if fixture.PublishedHint != nil {
					item.Age = fixture.PublishedHint.Time().Format("2006-01-02T15:04:05Z07:00")
				}
				payload.Web.Results = append(payload.Web.Results, item)
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			return jsonResponse(http.StatusOK, string(body), nil), nil
		})
		provider, err := NewBrave(client, "fixture-token")
		if err != nil {
			t.Fatal(err)
		}
		return provider
	})
}
