package main

import (
	"sync"
	"net/http"
	"strconv"
	"encoding/json"
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

	logError = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

const (
	warmUpClientsNum = 5
	testClientsNum   = 10
	testMessagesNum  = 10
)

type ErrWrongGetItemsResponse struct {
	message string
}

func NewErrWrongGetItemsResponse(message string) *ErrWrongGetItemsResponse {
	return &ErrWrongGetItemsResponse{
		message: message,
	}
}

func (e *ErrWrongGetItemsResponse) Error() string {
	return e.message
}

type ErrWrongBuyItemResponse struct {
	message string
}

func NewErrWrongBuyItemResponse(message string) *ErrWrongBuyItemResponse {
	return &ErrWrongBuyItemResponse{
		message: message,
	}
}

func (e *ErrWrongBuyItemResponse) Error() string {
	return e.message
}

type Item struct {
	Name  string `json:"name"`
	Price string `json:"price"`
}

func getExpectedGetItemsResponseBody(userName string) string {
	const defaultGoodsNumber int = 5

	responseBody := map[string]interface{}{}

	if userName != "" {
		var multiplier int = 0

		items := make([]Item, len(userName))

		responseBody["nickname"] = userName
		for _, charValue := range userName {
			multiplier += int(charValue)
		}

		for currentItemNumber := 0; currentItemNumber < len(items); currentItemNumber++ {
			newItem := Item{}
			newItem.Name = userName + strconv.Itoa(currentItemNumber)
			newItem.Price = strconv.Itoa((currentItemNumber + 1) * multiplier)

			items[currentItemNumber] = newItem
		}

		responseBody["items"] = items

	} else {
		var multiplier int = 30

		items := make([]Item, defaultGoodsNumber)

		for currentItemNumber := 0; currentItemNumber < len(items); currentItemNumber++ {
			newItem := Item{}
			newItem.Name = "default" + strconv.Itoa(currentItemNumber)
			newItem.Price = strconv.Itoa(currentItemNumber * multiplier)

			items[currentItemNumber] = newItem
		}

		responseBody["items"] = items
	}

	jsonBody, _ := json.Marshal(responseBody)
	return string(jsonBody)
}

func getExpectedBuyItemResponseBody(itemName string) string {
	responseBody := map[string]interface{}{}

	successPurchaseMessage := "success"
	failurePurchaseMessage := "failure"

	if len(itemName) % 2 == 0 {
		responseBody["result"] = successPurchaseMessage
	} else {
		responseBody["result"] = failurePurchaseMessage
	}

	jsonBody, _ := json.Marshal(responseBody)
	return string(jsonBody)
}

func checkBuyItemResponse(requestedItemName, response string, responseStatusCode int) error {
	expectedResponse := getExpectedBuyItemResponseBody(requestedItemName)

	if responseStatusCode != 200 {
		return NewErrWrongBuyItemResponse("Wrong Status Code")
	}

	if response != expectedResponse {
		return NewErrWrongBuyItemResponse("wrong response body")
	}

	return nil
}

func checkGetItemsResponse(userName, response string, statusCode int) error {
	expectedResponse := getExpectedGetItemsResponseBody(userName)

	if statusCode != 200 {
		return NewErrWrongGetItemsResponse("Wrong Status Code")
	}

	if response != expectedResponse {
		return NewErrWrongGetItemsResponse("Wrong response body")
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

type ResponseBody struct {
	Nickname string `json:"nickname"`
	Items    []Item `json:"items"`
}

func startTestClient(path, queryParam, body string, currentClientNumber int, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < testMessagesNum; currentMessageNumber++ {
		response, _ := sendRequest(path, queryParam, body)

		responseBytes, _ := ioutil.ReadAll(response.Body)

		logInfo.Printf("[Goroutine %d] Message %d was successfully sent\n", currentClientNumber, currentMessageNumber)

		if resultCheck := checkGetItemsResponse("", string(responseBytes), response.StatusCode); resultCheck.(type) != nil {
			logError.Printf("[Goroutine %d][Message %d][Get Items Test] Got invalid response. Error Message: %s\n", currentClientNumber, currentMessageNumber, resultCheck)
		} else {
			logInfo.Printf("[Goroutine %d][Message %d][Get Items Test] Got valid response\n", currentClientNumber, currentMessageNumber)

			var responseBody = ResponseBody{}

			json.Unmarshal(responseBytes, responseBody)

			items := responseBody.Items

			for _, currentItem := range items {
				requestBody, _ := json.Marshal(currentItem)

				response, _ := sendRequest("/buy", queryParam, string(requestBody))

				responseBytes, _ := ioutil.ReadAll(response.Body)

				if resultCheck := checkBuyItemResponse(currentItem.Name, string(responseBytes), response.StatusCode); resultCheck != nil {
					logError.Printf("[Goroutine %d][Message %d][Buy Items Test] Got invalid response. Error Message: %s\n", currentClientNumber, currentMessageNumber, resultCheck)
				} else {
					logInfo.Printf("[Goroutine %d][Message %d][Buy Items Test] Got valid response\n", currentClientNumber, currentMessageNumber)
				}
			}
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