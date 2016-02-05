package dataCollect

import (
	"fmt"
	"net/http"

	"github.com/laktek/Stack-on-Go/stackongo"
)

// Details on a StackExchange app
type AppDetails struct {
	Client_id       string
	Redirect_uri    string
	Client_secret   string
	Key             string
	Options         map[string]string
	Filters         string
	Quota_remaining int
}

// Returns questions based on search parameters params
func Collect(appInfo AppDetails, params stackongo.Params, transport http.RoundTripper) (*stackongo.Questions, error) {
	// Add standard parameters
	params = addParams(appInfo, params)

	// Search for questions
	questions, err := searchAdvanced(params, transport)
	if err != nil {
		return nil, err
	}
	if questions.Error_id != 0 {
		return nil, fmt.Errorf("%v: %v", questions.Error_name, questions.Error_message)
	}

	// If there are more pages of search results, repeat search with next page
	for questions.Has_more && questions.Quota_remaining > 0 {
		params.Page(questions.Page + 1)
		nextPage, err := searchAdvanced(params, transport)
		if err != nil {
			return nil, err
		}
		if nextPage.Error_id != 0 {
			return nil, fmt.Errorf("%v: %v", nextPage.Error_name, nextPage.Error_message)
		}
		nextPage.Items = append(questions.Items, nextPage.Items...)
		questions = nextPage
	}
	return questions, nil
}

// Return questions based on ids
func GetQuestionsByIDs(session *stackongo.Session, ids []int, appInfo AppDetails, params stackongo.Params) (*stackongo.Questions, error) {
	// Add standard parameters
	params = addParams(appInfo, params)

	// Search for questions
	questions, err := session.GetQuestions(ids, params)
	if err != nil {
		return nil, fmt.Errorf("Failed at ln 57 of dataCollect: %v", err.Error())
	}
	if questions.Error_id != 0 {
		return nil, fmt.Errorf("%v: %v", questions.Error_name, questions.Error_message)
	}

	// If there are more pages of search results, repeat search with next page
	for questions.Has_more && questions.Quota_remaining > 0 {
		params.Page(questions.Page + 1)
		nextPage, err := session.GetQuestions(ids, params)
		if err != nil {
			return nil, fmt.Errorf("Failed at ln 66 of dataCollect: %v", err)
		}
		if questions.Error_id != 0 {
			return nil, fmt.Errorf("%v: %v", nextPage.Error_name, nextPage.Error_message)
		}
		nextPage.Items = append(questions.Items, nextPage.Items...)
		questions = nextPage
	}
	return questions, nil
}

// Add standard parameters
func addParams(appInfo AppDetails, params stackongo.Params) stackongo.Params {
	params.Add("key", appInfo.Key)
	params.Add("filter", appInfo.Filters)
	params.Add("site", "stackoverflow")
	return params
}
