package identity

import (
	"net/http"
	"strings"
)

type Resolver interface {
	Subject(request *http.Request) (string, bool)
}

type HeaderResolver struct {
	Header string
}

func (resolver HeaderResolver) Subject(request *http.Request) (string, bool) {
	subject := strings.TrimSpace(request.Header.Get(resolver.Header))

	return subject, subject != ""
}
