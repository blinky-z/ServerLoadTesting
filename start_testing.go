package main

import (
	"sync"
	"net/http"
	"strconv"
	"encoding/json"
	"io/ioutil"
	"time"
	"log"
	"io"
	"os"
	"bytes"
	"mime/multipart"
	"sync/atomic"
)

var (
	logInfo  *log.Logger
	logError *log.Logger

	totalMessagesCount uint32

	getItemsErrors []ErrGetItems
	buyItemsErrors []ErrBuyItems

	mux *sync.Mutex
)

type ErrGetItems struct {
	time time.Time
	message string
}

type ErrBuyItems struct {
	time time.Time
	message string
}

func (err *ErrGetItems) Error() string {
	return "[" + err.time.Format("15:04:05") + "] " + err.message
}

func (err *ErrBuyItems) Error() string {
	return "[" + err.time.Format("15:04:05") + "] " + err.message
}

func Init(infoHandle io.Writer, errorHandle io.Writer) {
	logInfo = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	logError = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	totalMessagesCount = 0

	getItemsErrors = make([]ErrGetItems, 0)
	buyItemsErrors = make([]ErrBuyItems, 0)

	mux = &sync.Mutex{}
}

const (
	warmUpClientsNum = 5
	testClientsNum   = 10
	testMessagesNum  = 10
)

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

func checkBuyItemResponse(requestedItemName, response string, statusCode int) *ErrBuyItems {
	expectedResponse := getExpectedBuyItemResponseBody(requestedItemName)

	if statusCode != 200 {
		return &ErrBuyItems{time : time.Now(), message: "wrong status code"}
	}

	if response != expectedResponse {
		return &ErrBuyItems{time : time.Now(), message: "wrong response body"}
	}

	return nil
}

func checkGetItemsResponse(userName, response string, statusCode int) *ErrGetItems {
	expectedResponse := getExpectedGetItemsResponseBody(userName)

	if statusCode != 200 {
		return &ErrGetItems{time : time.Now(), message: "wrong status code"}
	}

	if response != expectedResponse {
		return &ErrGetItems{time : time.Now(), message: "wrong response body"}
	}

	return nil
}

func sendRequest(path, queryParams, body, contentType string) (*http.Response, error) {
	url := "http://localhost:8080" + path

	var response *http.Response
	var err error

	if body == "" {
		if queryParams != "" {
			url += "?" + queryParams
		}

		response, err = http.Get(url)
	} else {
		switch contentType {
		case "application/x-www-form-urlencoded":
			body = "json=" + body
			response, err = http.Post(url, contentType, bytes.NewBuffer([]byte(body)))
		case "multipart/form-data":
			client := &http.Client{}

			multipartBody := &bytes.Buffer{}
			writer := multipart.NewWriter(multipartBody)

			writer.WriteField("json", body)

			writer.Close()

			request, _ := http.NewRequest("POST", url, multipartBody)
			request.Header.Set("Content-Type", writer.FormDataContentType())

			response, err = client.Do(request)
		}
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
		response, _ := sendRequest(path, queryParam, body, "")

		atomic.AddUint32(&totalMessagesCount, 1)

		responseBytes, _ := ioutil.ReadAll(response.Body)

		if resultCheck := checkGetItemsResponse("", string(responseBytes), response.StatusCode); resultCheck != nil {
			logError.Printf("[Goroutine %d][Message %d][Get Items Test] Got invalid response. " +
				"Error Message: %s\n", currentClientNumber, currentMessageNumber, resultCheck)

			mux.Lock()
			getItemsErrors = append(getItemsErrors, *resultCheck)
			mux.Unlock()

		} else {
			logInfo.Printf("[Goroutine %d][Message %d][Get Items Test] Got valid response\n",
				currentClientNumber, currentMessageNumber)

			var responseBody = ResponseBody{}
			json.Unmarshal(responseBytes, &responseBody)

			items := responseBody.Items

			for index, currentItem := range items {
				atomic.AddUint32(&totalMessagesCount, 1)

				requestBody, _ := json.Marshal(currentItem)

				response, _ := sendRequest("/buy", queryParam, string(requestBody), "application/x-www-form-urlencoded")
				//response, _ := sendRequest("/buy", queryParam, string(requestBody), "multipart/form-data")

				responseBytes, _ := ioutil.ReadAll(response.Body)

				resultCheck := checkBuyItemResponse(currentItem.Name, string(responseBytes), response.StatusCode)

				if resultCheck != nil {
					logError.Printf("[Goroutine %d][Message %d][Buy Items Test] Got invalid response. " +
						"Error Message: %s\n", currentClientNumber, index, resultCheck)

					mux.Lock()
					buyItemsErrors = append(buyItemsErrors, *resultCheck)
					mux.Unlock()
				} else {
					logInfo.Printf("[Goroutine %d][Message %d][Buy Items Test] Got valid response\n",
						currentClientNumber, index)
				}
			}
		}
	}
}

func main() {
	Init(os.Stdout, os.Stderr)

	wgWarmUp := &sync.WaitGroup{}

	// create clients to warm up a test ground
	for currentClientNumber := 0; currentClientNumber < warmUpClientsNum; currentClientNumber++ {
		wgWarmUp.Add(1)

		path, queryParams, requestBody := "/", "", ""

		go startTestClient(path, queryParams, requestBody, currentClientNumber, wgWarmUp)
	}
	time.Sleep(time.Millisecond)

	wgWarmUp.Wait()

	logInfo.Println("[MAIN] Warm up has been done")

	wgTest := &sync.WaitGroup{}

	// create clients for web server load testing
	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		path, queryParams, requestBody := "/", "", ""

		go startTestClient(path, queryParams, requestBody, currentClientNumber, wgTest)
	}
	time.Sleep(time.Millisecond)

	wgTest.Wait()

	logInfo.Printf("[MAIN] All tests has been done. Requests was sended count: %d. " +
		"Error statistics: %d errors occured during get items tests, %d errors occured during buy items tests",
		totalMessagesCount, len(getItemsErrors), len(buyItemsErrors))
}