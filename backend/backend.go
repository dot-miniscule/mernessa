package backend

import (
	"dataCollect"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/appengine/urlfetch"

	"github.com/laktek/Stack-on-Go/stackongo"
)

var (
	transport http.RoundTripper
	keyWords  = "places api"
	tags      = []string{"google-places-api", "google-places"}
	appInfo   = dataCollect.AppDetails{
		Client_id:     "6029",
		Redirect_uri:  "http://stacktracker-1184.appspot.com/home",
		Client_secret: "ymefu0zw2TIULhSTM03qyg((",
		Key:           "nHI22oWrBEsUN8kHe4ARsQ((",

		Filters: "!*7Pmg80Pr9s5.fICa8ob-dRh8NP6",
		// Filters include:
		//	- Wrapper: backoff, error_id, error_message, error_name,
		//             has_more, items, page, quota_remaining
		//	- Question: body, creation_date, link, question_id, tags, title

		Options: map[string]string{
			"scope": "write_access, no_expiry",
		},
	}
	session = new(stackongo.Session)
)

func SetTransport(c context.Context) {
	transport = &urlfetch.Transport{Context: c}
	stackongo.SetTransport(transport)
}

func NewSession() {
	session = stackongo.NewSession("stackoverflow")
}

type byCreationDate []stackongo.Question

func (a byCreationDate) Len() int           { return len(a) }
func (a byCreationDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreationDate) Less(i, j int) bool { return a[i].Creation_date > a[j].Creation_date }

func GetNewQns(fromDate time.Time, toDate time.Time) (*stackongo.Questions, error) {
	// Adding parameters to request
	params := make(stackongo.Params)
	params.Pagesize(100)
	params.Fromdate(fromDate)
	params.Todate(toDate)
	params.Sort("creation")
	params.Add("accepted", false)
	params.Add("closed", false)

	// Add questions tagged with "google-places-api"
	params.Add("tagged", tags[0])
	questions, err := dataCollect.Collect(appInfo, params, transport)
	if err != nil {
		return nil, err
	}

	if questions.Quota_remaining <= 0 {
		return questions, fmt.Errorf("No StackExchange requests remaining")
	}

	// Add questions tagged with "google-places"
	params.Add("tagged", tags[1])
	params.Add("nottagged", tags[0])
	tagQuestions, err := dataCollect.Collect(appInfo, params, transport)
	if err != nil {
		return questions, err
	}
	tagQuestions.Items = append(questions.Items, tagQuestions.Items...)
	questions = tagQuestions
	sort.Sort(byCreationDate(questions.Items))

	if questions.Quota_remaining <= 0 {
		return questions, fmt.Errorf("No StackExchange requests remaining")
	}

	// Add questions with "Places API" in the body
	params.Del("tagged")
	params.Add("body", keyWords)
	params.AddVectorized("nottagged", tags)
	bodyQuestions, err := dataCollect.Collect(appInfo, params, transport)
	if err != nil {
		return questions, err
	}
	bodyQuestions.Items = append(questions.Items, bodyQuestions.Items...)
	questions = bodyQuestions
	sort.Sort(byCreationDate(questions.Items))

	if questions.Quota_remaining <= 0 {
		return questions, fmt.Errorf("No StackExchange requests remaining")
	}

	// Add questions with "Places API" in the body
	params.Del("body")
	params.Add("title", keyWords)
	titleQuestions, err := dataCollect.Collect(appInfo, params, transport)
	if err != nil {
		return questions, err
	}
	titleQuestions.Items = append(questions.Items, titleQuestions.Items...)
	questions = titleQuestions
	sort.Sort(byCreationDate(questions.Items))

	return questions, nil
}

func NewSearch(r *http.Request, params stackongo.Params) (*stackongo.Questions, error) {
	return dataCollect.Collect(appInfo, params, transport)
}

func AuthURL() string {
	return stackongo.AuthURL(appInfo.Client_id, appInfo.Redirect_uri, appInfo.Options)
}

func ObtainAccessToken(code string) (map[string]string, error) {
	return stackongo.ObtainAccessToken(appInfo.Client_id, appInfo.Client_secret, code, appInfo.Redirect_uri)
}

func AuthenticatedUser(params map[string]string, access_token string) (stackongo.User, error) {
	return session.AuthenticatedUser(params, map[string]string{"key": appInfo.Key, "access_token": access_token})
}

func GetUser(user_id int, params map[string]string) (stackongo.User, error) {
	params["key"] = appInfo.Key
	users, err := session.GetUsers([]int{user_id}, params)
	if err != nil {
		return stackongo.User{}, err
	}
	if len(users.Items) == 0 {
		return stackongo.User{}, errors.New("User " + strconv.Itoa(user_id) + " not found")
	}
	return users.Items[0], nil
}

// Function to make a fresh request to the Stack Exchange API to return questions relating to set of ID's
// Initiates parameters required to make the request.
// The returning data is then sent back to the webui handler to be parsed into the page
func GetQuestions(ctx context.Context, ids []int) (*stackongo.Questions, error) {
	params := make(stackongo.Params)
	params.Pagesize(100)
	params.Sort("creation")
	params.AddVectorized("tagged", tags)

	questions, err := dataCollect.GetQuestionsByIDs(session, ids, appInfo, params)
	if err != nil {
		return nil, errors.New("Error collection new question by id\t" + err.Error())
	}
	return questions, nil
}
