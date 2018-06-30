package main

import (
	"sync"
	"net/http"
	"strconv"
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"
	"bytes"
	"log"
	"io"
	"os"
)

var (
	logInfo  *log.Logger
	logError *log.Logger
)

func InitLoggers(infoHandle io.Writer, errorHandle io.Writer) {
	logInfo = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	logInfo.Output(0, "adasd")

	logError = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

const (
	warmUpClientsNum = 5
	testClientsNum   = 10
	testMessagesNum  = 10
)

type item struct {
	Name  string `json:"name"`
	Price string `json:"price"`
}

func getExpectedResponseBody(userName string) string {
	const defaultGoodsNumber int = 5

	responseBody := map[string]interface{}{}

	if userName != "" { // in case of username was passed
		var multiplier int = 0

		items := make([]item, len(userName))

		responseBody["nickname"] = userName
		for _, charValue := range userName {
			multiplier += int(charValue)
		}

		for currentItemNumber := 0; currentItemNumber < len(items); currentItemNumber++ {
			newItem := item{}
			newItem.Name = userName + strconv.Itoa(currentItemNumber)
			newItem.Price = strconv.Itoa((currentItemNumber + 1) * multiplier)

			items[currentItemNumber] = newItem
		}

		responseBody["items"] = items

	} else { // default case (anonymous client)
		var multiplier int = 30

		items := make([]item, defaultGoodsNumber)

		for currentItemNumber := 0; currentItemNumber < len(items); currentItemNumber++ {
			newItem := item{}
			newItem.Name = "default" + strconv.Itoa(currentItemNumber)
			newItem.Price = strconv.Itoa(currentItemNumber * multiplier)

			items[currentItemNumber] = newItem
		}

		responseBody["items"] = items
	}

	jsonBody, _ := json.Marshal(responseBody)
	return string(jsonBody)
}

func checkClientResponse(userName string, response *http.Response) error {
	expectedBody := getExpectedResponseBody(userName)

	defer response.Body.Close()
	receivedResponseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	if response.StatusCode != 200 {
		return errors.New("wrong status code")
	}

	if string(receivedResponseBody) != expectedBody {
		return errors.New("wrong response body")
	}

	return nil
}

func sendRequest(path, queryParams, body string) (*http.Response, error) {
	url := "http://localhost:8080" + path

	var response *http.Response
	var err error

	if body == "" {
		if queryParams != "" {
			url += "?" + queryParams
		}

		response, err = http.Get(url)
	} else {
		response, err = http.Post(url, "application/json", bytes.NewBuffer([]byte(body)))
	}

	return response, err
}

func startTestClient(path, queryParam, body string, currentClientNumber int, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < testMessagesNum; currentMessageNumber++ {
		response, err := sendRequest(path, queryParam, body)
		if err != nil {
			panic(err)
		}

		logInfo.Printf("[Goroutine %d] Message %d was successfully sent\n", currentClientNumber, currentMessageNumber)

		if resultCheck := checkClientResponse("", response); resultCheck != nil {
			logError.Printf("[Goroutine %d][Message %d] Got invalid response. Error Message: %s\n", currentMessageNumber, currentClientNumber, resultCheck)
		} else {
			logInfo.Printf("[Goroutine %d][Message %d] Got valid response\n", currentMessageNumber, currentClientNumber)
		}
	}
}

func main() {
	InitLoggers(os.Stdout, os.Stderr)

	wgWarmUp := &sync.WaitGroup{}

	// create clients to warm up a test ground
	for currentClientNumber := 0; currentClientNumber < warmUpClientsNum; currentClientNumber++ {
		wgWarmUp.Add(1)

		path, queryParams, requestBody := "/", "", ""

		go startTestClient(path, queryParams, requestBody, currentClientNumber, wgWarmUp)
	}
	time.Sleep(time.Millisecond)

	wgWarmUp.Wait()

	logInfo.Println("[MAIN] Warm up is done")

	wgTest := &sync.WaitGroup{}

	startTestingTime := time.Now()

	// create clients for web server load testing
	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		path, queryParams, requestBody := "/", "", ""

		go startTestClient(path, queryParams, requestBody, currentClientNumber, wgTest)
	}
	time.Sleep(time.Millisecond)

	wgTest.Wait()

	endTestingTime := time.Now()
	elapsed := endTestingTime.Sub(startTestingTime)

	logInfo.Printf("[MAIN] All tests are passed. Elapsed time: %v seconds", elapsed.Seconds())
}