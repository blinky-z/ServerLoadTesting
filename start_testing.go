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
	"net/url"
)

var (
	logInfo  *log.Logger
	logError *log.Logger

	testClientsNum        int
	testClientMessagesNum int

	totalMessagesCount uint32

	getItemsErrors []ErrGetItems
	buyItemsErrors []ErrBuyItems

	mux *sync.Mutex
)

const (
	warmUpClientsNum int = 5
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

func Init(infoHandle, errorHandle io.Writer, initTestClientsNum, initTestClientMessagesNum int) {
	logInfo = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	logError = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	totalMessagesCount = 0

	testClientsNum = initTestClientsNum
	testClientMessagesNum = initTestClientMessagesNum

	getItemsErrors = make([]ErrGetItems, 0)
	buyItemsErrors = make([]ErrBuyItems, 0)

	mux = &sync.Mutex{}
}

type ResponseBody struct {
	Nickname string `json:"nickname"`
	Items    []Item `json:"items"`
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

func sendRequest(path, queryParams, contentType, body string) (*http.Response, error) {
	requestUrl := "http://localhost:8080" + path

	var response *http.Response
	var err error

	if body == "" {
		if queryParams != "" {
			requestUrl += "?" + queryParams
		}

		response, err = http.Get(requestUrl)
	} else {
		switch contentType {
		case "application/x-www-form-urlencoded":
			response, err = http.PostForm(requestUrl, url.Values{"json": {body}})
		case "multipart/form-data":
			client := &http.Client{}

			multipartBody := &bytes.Buffer{}
			writer := multipart.NewWriter(multipartBody)

			writer.WriteField("json", body)

			writer.Close()

			request, _ := http.NewRequest("POST", requestUrl, multipartBody)
			request.Header.Set("Content-Type", writer.FormDataContentType())

			response, err = client.Do(request)
		}
	}

	return response, err
}

func startTestClient(userName, path, queryParam, contentType, body string, currentClientNumber int, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < testClientMessagesNum; currentMessageNumber++ {
		response, _ := sendRequest(path, queryParam, contentType, body)

		atomic.AddUint32(&totalMessagesCount, 1)

		responseBytes, _ := ioutil.ReadAll(response.Body)

		if resultCheck := checkGetItemsResponse(userName, string(responseBytes), response.StatusCode); resultCheck != nil {
			logError.Printf("[Goroutine %d][Message %d][Get Items Test] Got invalid response. "+
				"Error Message: %s", currentClientNumber, currentMessageNumber, resultCheck)

			mux.Lock()
			getItemsErrors = append(getItemsErrors, *resultCheck)
			mux.Unlock()
		} else {
			logInfo.Printf("[Goroutine %d][Message %d][Get Items Test] Got valid response",
				currentClientNumber, currentMessageNumber)

			var responseBody = ResponseBody{}
			json.Unmarshal(responseBytes, &responseBody)

			items := responseBody.Items

			for index, currentItem := range items {
				atomic.AddUint32(&totalMessagesCount, 1)

				buyItemRequestBody, _ := json.Marshal(currentItem)

				response, _ := sendRequest("/buy", queryParam, contentType, string(buyItemRequestBody))

				responseBytes, _ := ioutil.ReadAll(response.Body)

				resultCheck := checkBuyItemResponse(currentItem.Name, string(responseBytes), response.StatusCode)

				if resultCheck != nil {
					logError.Printf("[Goroutine %d][Message %d][Buy Items Test] Got invalid response. "+
						"Error Message: %s", currentClientNumber, index, resultCheck)

					mux.Lock()
					buyItemsErrors = append(buyItemsErrors, *resultCheck)
					mux.Unlock()
				} else {
					logInfo.Printf("[Goroutine %d][Message %d][Buy Items Test] Got valid response",
						currentClientNumber, index)
				}
			}
		}
	}
}

func main() {
	clientsNum, clientsMessagesNum := 20, 10
	Init(os.Stdout, os.Stderr, clientsNum, clientsMessagesNum)

	//--------------------
	//Warm Up A Test Ground

	wgWarmUp := &sync.WaitGroup{}

	for currentClientNumber := 0; currentClientNumber < warmUpClientsNum; currentClientNumber++ {
		wgWarmUp.Add(1)

		userName, path, queryParams, contentType, requestBody := "", "/", "", "multipart/form-data", ""

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgWarmUp)
	}
	time.Sleep(time.Millisecond)

	wgWarmUp.Wait()

	logInfo.Println("[MAIN] Warm up has been done")

	//--------------------
	//Load Tests
	//--------------------

	wgTest := &sync.WaitGroup{}

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		userName, path, queryParams, contentType, requestBody := "", "/", "", "application/x-www-form-urlencoded", ""

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgTest)
	}

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		userName, path, queryParams, contentType, requestBody := "", "/", "", "multipart/form-data", ""

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgTest)
	}

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		userName, path, queryParams, contentType, requestBody := "dmitry", "/", "name=dmitry", "application/x-www-form-urlencoded", ""

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgTest)
	}

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		userName, path, queryParams, contentType, requestBody := "dmitry", "/", "name=dmitry", "multipart/form-data", ""

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgTest)
	}

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		userName, path, queryParams, contentType, requestBody := "maxim", "/", "", "application/x-www-form-urlencoded", `{"name":"maxim"}`

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgTest)
	}

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		userName, path, queryParams, contentType, requestBody := "maxim", "/", "", "multipart/form-data", `{"name":"maxim"}`

		go startTestClient(userName, path, queryParams, contentType, requestBody, currentClientNumber, wgTest)
	}

	wgTest.Wait()

	logInfo.Printf("[MAIN] All tests has been done. Sended requests count: %d. "+
		"Error statistics: %d errors occured during get items tests, %d errors occured during buy items tests",
		totalMessagesCount, len(getItemsErrors), len(buyItemsErrors))
}