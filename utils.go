package main

import (
	"strings"
)

func parseUser(user string) string {
	parts := strings.Split(user, "@")
	ids := strings.Split(parts[0], "_")

	return ids[1]

}
