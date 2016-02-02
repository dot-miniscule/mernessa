package dataCollect

import (
	"fmt"
	"net/http"

	"time"

	"github.com/laktek/Stack-on-Go/stackongo"

	"google.golang.org/appengine/log"
	"google.golang.org/appengine"
)

type AppDetails struct {
	Client_id       string
	Redirect_uri    string
	Client_secret   string
	Key             string
	Options         map[string]string
	Filters         string
	Quota_remaining int
}

var delay = 1 * time.Second
var week = (24 * 7) * time.Hour

func Collect(session *stackongo.Session, appInfo AppDetails, params stackongo.Params) (*stackongo.Questions, error) {
	params = addParams(appInfo, params)
	questions, err := session.AllQuestions(params)
	if err != nil {
		return nil, err
	}
	if questions.Error_id != 0 {
		return nil, fmt.Errorf("%v: %v", questions.Error_name, questions.Error_message)
	}
	for questions.Has_more && questions.Quota_remaining > 0 {
		params.Page(questions.Page + 1)
		nextPage, err := session.AllQuestions(params)
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

func GetQuestionsByIDs(req *http.Request, session *stackongo.Session, ids []int, appInfo AppDetails, params stackongo.Params) (*stackongo.Questions, error) {
	params = addParams(appInfo, params)
	c := appengine.NewContext(req)
	questions, err := session.GetQuestions(ids, params)
	if err != nil {
		log.Errorf(c, "Failed at ln 58 of dataCollect:\t", err)
		return nil, err
	}
	if questions.Error_id != 0 {
		return nil, fmt.Errorf("%v: %v", questions.Error_name, questions.Error_message)
	}
	for questions.Has_more && questions.Quota_remaining > 0 {
		params.Page(questions.Page + 1)
		nextPage, err := session.GetQuestions(ids, params)
		if err != nil {
			log.Errorf(c, "Failed at ln 68 of dataCollect:\t", err)
			return nil, err
		}
		if questions.Error_id != 0 {
			return nil, fmt.Errorf("%v: %v", nextPage.Error_name, nextPage.Error_message)
		}
		nextPage.Items = append(questions.Items, nextPage.Items...)
		questions = nextPage
	}
	return questions, nil
}

func addParams(appInfo AppDetails, params stackongo.Params) stackongo.Params {
	params.Add("key", appInfo.Key)
	params.Add("filter", appInfo.Filters)
	params.Add("site", "stackoverflow")
	return params
}