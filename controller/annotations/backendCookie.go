package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type BackendCookie struct {
	name    string
	backend *models.Backend
}

func NewBackendCookie(n string, b *models.Backend) *BackendCookie {
	return &BackendCookie{name: n, backend: b}
}

func (a *BackendCookie) GetName() string {
	return a.name
}

func (a *BackendCookie) Process(input string) error {
	if input == "" {
		a.backend.Cookie = nil
		return nil
	}
	cookieName := input
	if len(strings.Fields(cookieName)) != 1 {
		return fmt.Errorf("cookie-persistence: Incorrect input %s", input)
	}
	cookie := models.Cookie{
		Name:     &cookieName,
		Type:     "insert",
		Nocache:  true,
		Indirect: true,
	}
	a.backend.Cookie = &cookie
	return nil
}
