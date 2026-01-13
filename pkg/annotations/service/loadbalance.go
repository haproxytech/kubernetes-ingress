package service

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

type LoadBalance struct {
	backend *models.Backend
	name    string
}

func NewLoadBalance(n string, b *models.Backend) *LoadBalance {
	return &LoadBalance{name: n, backend: b}
}

func (a *LoadBalance) GetName() string {
	return a.name
}

func (a *LoadBalance) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.backend.Balance = nil
		return nil
	}
	var params *models.Balance
	var err error
	params, err = getParamsFromInput(input)
	if err != nil {
		return fmt.Errorf("load-balance: %w", err)
	}

	if err := params.Validate(nil); err != nil {
		return fmt.Errorf("load-balance: %w", err)
	}
	a.backend.Balance = params
	return nil
}

func getParamsFromInput(value string) (*models.Balance, error) {
	balance := &models.Balance{}
	tokens := strings.Split(value, " ")
	if len(tokens) == 0 {
		return nil, errors.New("missing algorithm name")
	}

	reg := regexp.MustCompile(`(\(|\))`)
	algorithmTokens := reg.Split(tokens[0], -1)
	algorithm := algorithmTokens[0]
	balance.Algorithm = &algorithm
	if len(algorithmTokens) == 3 {
		switch algorithm {
		case "hdr":
			balance.HdrName = algorithmTokens[1]
		case "random":
			rand, err := strconv.Atoi(algorithmTokens[1])
			if err != nil {
				return balance, err
			}
			balance.RandomDraws = int64(rand)
		case "rdp-cookie":
			balance.RdpCookieName = algorithmTokens[1]
		}
	}
	i := 1
	if algorithm == "url_param" {
		if i >= len(tokens) {
			return balance, fmt.Errorf("missing parameter for algorithm '%s' in balance configuration", algorithm)
		}
		balance.URLParam = tokens[i]
		i++
	}

	for ; i < len(tokens); i++ {
		token := tokens[i]
		switch token {
		case "len":
			if i+1 >= len(tokens) {
				return balance, fmt.Errorf("missing parameter for option '%s' in balance configuration", token)
			}
			length, err := strconv.Atoi(tokens[i+1])
			if err != nil {
				return balance, err
			}
			balance.URILen = int64(length)
			// We already got the next token
			i++
		case "depth":
			if i+1 >= len(tokens) {
				return balance, fmt.Errorf("missing parameter for option '%s' in balance configuration", token)
			}
			depth, err := strconv.Atoi(tokens[i+1])
			if err != nil {
				return balance, err
			}
			balance.URIDepth = int64(depth)
			// We already got the next token
			i++
		case "whole":
			balance.URIWhole = true
		case "max_wait":
			if i+1 >= len(tokens) {
				return balance, fmt.Errorf("missing parameter for option '%s' in balance configuration", token)
			}
			maxWait, err := strconv.Atoi(tokens[i+1])
			if err != nil {
				return balance, err
			}
			balance.URLParamMaxWait = int64(maxWait)
			// We already got the next token
			i++
		case "path-only":
			balance.URIPathOnly = true
		case "check_post":
			if i+1 >= len(tokens) {
				return balance, fmt.Errorf("missing parameter for option '%s' in balance configuration", token)
			}
			checkPost, err := strconv.Atoi(tokens[i+1])
			if err != nil {
				return balance, err
			}
			balance.URLParamCheckPost = int64(checkPost)
			// We already got the next token
			i++
		case "use_domain_only":
			balance.HdrUseDomainOnly = true
		default:
			return balance, fmt.Errorf("unknown balance configuration '%s' ", token)
		}
	}
	return balance, nil
}
