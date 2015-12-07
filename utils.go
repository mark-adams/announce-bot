package announcebot

import (
	"net/http"
	"strings"

	"github.com/gorilla/context"
)

func parseUser(user string) string {
	parts := strings.Split(user, "@")
	ids := strings.Split(parts[0], "_")

	return ids[1]

}

func getConfiguration(r *http.Request) *Configuration {
	if rv := context.Get(r, "config"); rv != nil {
		return rv.(*Configuration)
	}
	return nil
}
