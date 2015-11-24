package main

import (
	"fmt";
    "time";

	"github.com/laktek/Stack-on-Go/stackongo"
//	"appengine/urlfetch"
)

var delay = 500 * time.Millisecond

func main() {
/*	c := appengine.NewContext(r)
	ut := &urlfetch.Transport{Context: c}

	stackongo.SetTransport(ut)
*/
	tags := []string{"google-places-api"}
	session := stackongo.NewSession("stackoverflow")
	params := make(stackongo.Params)
	params.AddVectorized("tagged", tags)
	params.Add("accepted", false)

    go func(params *stackongo.Params) {
        questions, err := session.UnansweredQuestions(params)
	    if err != nil {
		    fmt.Println("*****************\n" + err.Error()+ "\n*****************")
	    }
        time.Sleep(delay)
    }

	for _, question := range questions.Items {
		fmt.Println(question.Title)
		fmt.Printf("Link: %v\n\n", question.Link)
	}
}
