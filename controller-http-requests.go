package main

import (
	"sort"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) RequestsHTTPRefresh(transaction *models.Transaction) (err error) {
	if c.cfg.HTTPRequestsStatus == EMPTY {
		return nil
	}
	nativeAPI := c.NativeAPI

	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", "http", transaction.ID, 0)
	}
	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", "https", transaction.ID, 0)
	}
	//INFO: order is reversed, first you insert last ones
	if len(c.cfg.HTTPRequests[HTTP_REDIRECT]) > 0 {
		request1 := &c.cfg.HTTPRequests[HTTP_REDIRECT][0]

		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", request1, transaction.ID, 0)
		LogErr(err)
	}
	if len(c.cfg.HTTPRequests[RATE_LIMIT]) > 0 {
		request1 := &c.cfg.HTTPRequests[RATE_LIMIT][0]
		request2 := &c.cfg.HTTPRequests[RATE_LIMIT][1]

		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", request2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", request1, transaction.ID, 0)
		LogErr(err)

		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "https", request2, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "https", request1, transaction.ID, 0)
		LogErr(err)
	}

	sortedList := []string{}
	exclude := map[string]struct{}{
		HTTP_REDIRECT: struct{}{},
		RATE_LIMIT:    struct{}{},
	}
	for name, _ := range c.cfg.HTTPRequests {
		_, excluding := exclude[name]
		if !excluding {
			sortedList = append(sortedList, name)
		}
	}
	sort.Strings(sortedList)
	for _, name := range sortedList {
		for _, request := range c.cfg.HTTPRequests[name] {
			err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", &request, transaction.ID, 0)
			LogErr(err)
			err = nativeAPI.Configuration.CreateHTTPRequestRule("frontend", "https", &request, transaction.ID, 0)
			LogErr(err)
		}
	}

	return nil
}

func (c *HAProxyController) requestsTCPRefresh(transaction *models.Transaction) (err error) {
	if c.cfg.TCPRequestsStatus == EMPTY {
		return nil
	}
	nativeAPI := c.NativeAPI

	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", "http", transaction.ID, 0)
	}
	err = nil
	for err == nil {
		err = nativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", "https", transaction.ID, 0)
	}

	if len(c.cfg.TCPRequests[RATE_LIMIT]) > 0 {
		request1 := &c.cfg.TCPRequests[RATE_LIMIT][0]
		request2 := &c.cfg.TCPRequests[RATE_LIMIT][1]

		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "http", request1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "http", request2, transaction.ID, 0)
		LogErr(err)

		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "https", request1, transaction.ID, 0)
		LogErr(err)
		err = nativeAPI.Configuration.CreateTCPRequestRule("frontend", "https", request2, transaction.ID, 0)
		LogErr(err)
	}

	return nil
}
