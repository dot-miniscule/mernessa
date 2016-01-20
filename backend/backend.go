package backend

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

	questions, err := dataCollect.Collect(session, appInfo, params)
	if err != nil {
		return nil, errors.New("Error collecting questions\t" + err.Error())
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
