package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

type Response struct {
	Service string
	ID      string
	Num     string
}

func (r Response) Name() string {
	return fmt.Sprintf("%s-%s", r.Service, r.ID)
}

func parseResponse(body []byte) Response {
	response := strings.Split(strings.Trim(string(body), "\n"), "-")
	if len(response) == 3 {
		return Response{
			Service: response[0],
			ID:      response[1],
			Num:     response[2],
		}
	}
	if len(response) == 2 {
		return Response{
			Service: response[0],
			ID:      response[1],
		}
	}
	failAndExit("unexpected result [%s]", string(body))
	return Response{}
}

func failAndExit(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	os.Exit(1)
}

func checkErr(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func callHTTP(host string, port int) Response {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/gidc", port), nil)
	checkErr(err)
	req.Host = host

	resp, err := http.DefaultClient.Do(req)
	checkErr(err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	return parseResponse(body)
}

func main() {
	counter := map[string]int{}
	for i := 0; i < 8; i++ {
		r := callHTTP("hr.haproxy", 30080)
		counter[r.Name()]++
	}
	for k, v := range counter {
		if v != 2 {
			failAndExit("expected 2 responses from %s, got %d", k, v)
		}
	}

	counter = map[string]int{}
	for i := 0; i < 4; i++ {
		r := callHTTP("fr.haproxy", 30080)
		counter[r.Name()]++
	}
	for k, v := range counter {
		if v != 2 {
			failAndExit("expected 2 responses from %s, got %d", k, v)
		}
	}

	counter = map[string]int{}
	for i := 0; i < 4; i++ {
		r := callHTTP("haproxy.org", 32766)
		counter[r.Name()]++
	}
	for k, v := range counter {
		if v != 4 {
			failAndExit("expected 2 responses from %s, got %d", k, v)
		}
	}

	counter = map[string]int{}
	for i := 0; i < 4; i++ {
		r := callHTTP("haproxy.org", 30080)
		counter[r.Name()]++
	}
	if len(counter) != 1 {
		failAndExit("expected one service responding")
	}
	v, ok := counter["default backend - 404"]
	if !ok {
		log.Panic("expected result from `default backend - 404`")
	}
	if v != 4 {
		failAndExit("expected 4 responses from %s, got %d", "default backend - 404", v)
	}
}
