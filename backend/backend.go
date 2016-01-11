package backend

//package main

import (
	"dataCollect"
	"errors"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/laktek/Stack-on-Go/stackongo"
)

var (
	tags    = []string{"google-places-api"}
	appInfo = dataCollect.AppDetails{
		Client_id:     "6029",
		Redirect_uri:  "http://stacktracker-1184.appspot.com/home",
		Client_secret: "ymefu0zw2TIULhSTM03qyg((",
		Key:           "nHI22oWrBEsUN8kHe4ARsQ((",

		Filters: "!846.hCHXJtBDPB1pe-0GnXRad1cyWBkz(ithJ4-ztkzynXtQgKxaGE4ry3jiLpLNWv5",
		// Filters include:
		//	- Wrapper: backoff, error_id, error_message, error_name,
		//             has_more, items, quota_remaining
		//	- Question: body, creation_date, link, question_id, title

		Options: map[string]string{
			"scope": "write_access, no_expiry",
		},
	}
	session = new(stackongo.Session)
)

func SetTransport(r *http.Request) {
	c := appengine.NewContext(r)
	ut := &urlfetch.Transport{Context: c}
	stackongo.SetTransport(ut)
}

func NewSession() {
	session = stackongo.NewSession("stackoverflow")
}

func GetNewQns(fromDate time.Time, toDate time.Time) (*stackongo.Questions, error) {
	// Set starting variable parameters
	// Adding parameters to request
	params := make(stackongo.Params)
	params.Pagesize(100)
	params.Fromdate(fromDate)
	params.Todate(toDate)
	params.Sort("creation")
	params.Add("accepted", false)
	params.AddVectorized("tagged", tags)

	questions := new(stackongo.Questions)
	questions.Has_more = true
	page := 0

	for questions.Has_more && appInfo.Quota_remaining > 0 {
		page++
		params.Page(page)

		nextPage, err := dataCollect.Collect(session, appInfo, params)
		if err != nil {
			return nil, errors.New("Error collecting questions\t" + err.Error())
		}

		appInfo.Quota_remaining = nextPage.Quota_remaining
		if nextPage.Error_id != 0 {
			return nil, errors.New("Request error:\t" + questions.Error_name + ": " + questions.Error_message)
		}
		nextPage.Items = append(questions.Items, nextPage.Items...)
		questions = nextPage
	}
	return questions, nil
}

func NewSearch(r *http.Request, params stackongo.Params) (*stackongo.Questions, error) {
	return dataCollect.Collect(session, appInfo, params)
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

// for collecting datasets
// TODO(gregoriou): remove before launch
/*func main() {
	input, err := Collect(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	ioutil.WriteFile("3-12_dataset.json", input, 640)
}*/
