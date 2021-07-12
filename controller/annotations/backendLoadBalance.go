package annotations

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
)

type BackendLoadBalance struct {
	name    string
	params  *models.Balance
	backend *models.Backend
}

func NewBackendLoadBalance(n string, b *models.Backend) *BackendLoadBalance {
	return &BackendLoadBalance{name: n, backend: b}
}

func (a *BackendLoadBalance) GetName() string {
	return a.name
}

func (a *BackendLoadBalance) Parse(input string) error {
	params, err := getParamsFromInput(input)
	if err != nil {
		return fmt.Errorf("load-balance: %w", err)
	}

	if err := params.Validate(nil); err != nil {
		return fmt.Errorf("load-balance: %w", err)
	}
	a.params = params
	return nil
}

func (a *BackendLoadBalance) Update() error {
	a.backend.Balance = a.params
	return nil
}

func getParamsFromInput(value string) (*models.Balance, error) {
	balance := &models.Balance{}
	tokens := strings.Split(value, " ")
	if len(tokens) == 0 {
		return nil, errors.New("missing algorithm name")
	}

	reg := regexp.MustCompile(`(\\(|\\))"`)
	algorithmTokens := reg.Split(tokens[0], -1)
	algorithm := algorithmTokens[0]
	balance.Algorithm = &algorithm
	if len(algorithmTokens) == 3 {
		switch algorithm {
		case "hdr":
			balance.HdrName = algorithmTokens[1]
		case "random":
			if randomDraws, err := strconv.Atoi(algorithmTokens[1]); err == nil {
				balance.RandomDraws = int64(randomDraws)
			}
		case "rdp-cookie":
			balance.RdpCookieName = algorithmTokens[1]
		}
	}
	i := 1
	if algorithm == "url_param" {
		balance.URLParam = tokens[i]
		i++
	}

	for ; i < len(tokens); i++ {
		token := tokens[i]
		switch token {
		case "len":
			if i+1 >= len(tokens) {
				logger.Errorf("Missing parameter for option '%s' in balance configuration", token)
				continue
			}
			if length, err := strconv.Atoi(tokens[i+1]); err == nil {
				balance.URILen = int64(length)
			}
			// We already got the next token
			i++
		case "depth":
			if i+1 >= len(tokens) {
				logger.Errorf("Missing parameter for option '%s' in balance configuration", token)
				continue
			}
			if depth, err := strconv.Atoi(tokens[i+1]); err == nil {
				balance.URIDepth = int64(depth)
			}
			// We already got the next token
			i++
		case "whole":
			balance.URIWhole = true
		case "max_wait":
			if i+1 >= len(tokens) {
				logger.Errorf("Missing parameter for option '%s' in balance configuration", token)
				continue
			}
			if maxWait, err := strconv.Atoi(tokens[i+1]); err == nil {
				balance.URLParamMaxWait = int64(maxWait)
			}
			// We already got the next token
			i++
		case "path-only":
			balance.URIPathOnly = true
		case "check_post":
			if i+1 >= len(tokens) {
				logger.Errorf("Missing parameter for option '%s' in balance configuration", token)
				continue
			}
			if checkPost, err := strconv.Atoi(tokens[i+1]); err == nil {
				balance.URLParamCheckPost = int64(checkPost)
			}
			// We already got the next token
			i++
		case "use_domain_only":
			balance.HdrUseDomainOnly = true
		default:
			logger.Warningf("balance configuration '%s' is ignored", token)
		}
	}
	return balance, nil
}
