package main

import (
	"sync"
	"net/http"
	"strconv"
	"encoding/json"
	"io/ioutil"
	"time"
	"log"
	"os"
	"bytes"
	"mime/multipart"
	"sync/atomic"
	"net/url"
	"math/rand"
	"strings"
)

var (
	logInfoOutfile, _ = os.OpenFile("./logs/Info.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	logErrorOutfile, _ = os.OpenFile("./logs/Error.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)

	logInfo = log.New(logInfoOutfile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	logError = log.New(logErrorOutfile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	testClientsNum        int = 10
	testClientMessagesNum int = 10

	totalMessagesCount uint32

	getItemsErrors []ErrGetItems
	buyItemsErrors []ErrBuyItems

	getItemsRequestTimes []GetItemsRequestTime
	buyItemsRequestTimes []BuyItemsRequestTime

	mux *sync.Mutex

	myClient *http.Client

	requestClientNames = []string{"", "saneexclamation","buythroated","infuriatedlutchet","ticketbright","insecureloudmouth","soundingindirect","knowledgewives","gearherring","farmershortcrust","variablehertz","ripplinglens","otherscontrol","turnhotsprings","veincelery","excessfamily","iceskatesbale","ruffsescape","pencilelements","yellstable","mushroomslomo","edgecord","possessivegreeting","hertzodds","groaninfected","interiorrotating","firechargeenzyme","sickshower","leukocytedrink","prominencetub","fieldsmustache","woodcocklawful","leatherarmy","achernarinstance","europalepton","planesalami","customersworkbench","infinityhatching","plughumbug","competingfag","farrumscut","perpetualfallen","unwittinglaying","dirtycopernicium","icehockeymeteoroid","merseybeatstarbucks","milkperoxide","flingwater","flagrantcoins","kraftzing","fellsargon","bobstaysloshed","trymercury","freegantonic","barnacleburnt","masonsstrawberry","delayedmale","xiphoidtutor","asheatable","tengmalmshingles","aquilabummage","spotsbiceps","violinanother","tawnysyntax","frogsfeisty","nodulespity","calledpliocene","soddinggluttonous","billowygillette","stuffboson","collarbonelargest","parliamentblizzard","sadmarkings","streetsbailey","surfernissan","democracydividers","alloythine","frugalmust","plancaplay","normalaleutian","stingandalusian","skuaallee","intendedshark","paradigmboards","ventureskeg","kalmansledder","plaindolphin","singermention","employvolta","womenthorough","huhshare","grumpycepheus","magnetremuda","moralsdisrupt","correctfierce","rollmetrics","skeinboiling","amiablebiotic","actmind","baconsiphon","complexvenison"}
)

const (
	//serverUrl = "http://185.143.173.31"
	serverUrl = "http://localhost:8080"
	warmUpClientsNum = 100
	testMaxClientsNum = 500
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

func Init() {
	//http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 50000

	totalMessagesCount = 0

	getItemsErrors = make([]ErrGetItems, 0)
	buyItemsErrors = make([]ErrBuyItems, 0)

	getItemsRequestTimes = make([]GetItemsRequestTime, 0)
	buyItemsRequestTimes = make([]BuyItemsRequestTime, 0)

	// TODO: Сделать для каждого слайса свой mutex
	mux = &sync.Mutex{}
}

type GetItemsRequestTime struct {
	timeWhileSendingRequest          time.Time
	elapsedTime                      time.Duration
}

type BuyItemsRequestTime struct {
	timeWhileSendingRequest          time.Time
	elapsedTime                      time.Duration
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
	const defaultGoodsNumber = 5

	responseBody := map[string]interface{}{}

	if userName != "" {
		var multiplier = 0

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
		var multiplier = 30

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

	if statusCode == -1 {
		return nil
	}

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

	if statusCode == -1 {
		return nil
	}

	if statusCode != 200 {
		return &ErrGetItems{time : time.Now(), message: "wrong status code"}
	}

	if response != expectedResponse {
		return &ErrGetItems{time : time.Now(), message: "wrong response body"}
	}

	return nil
}

func sendRequest(resource, queryParams, contentType, body string) (statusCode int, responseBody string) {
	var request *http.Request
	var errRequestCreate error

	var response *http.Response
	var errResponse error

	var sendingStartTime time.Time
	var sendingEndTime time.Time

	u, _ := url.ParseRequestURI(serverUrl)
	u.Path = resource
	requestUrl := u.String()

	if body == "" {
		if queryParams != "" {
			requestUrl += "?" + queryParams
		}

		request, errRequestCreate = http.NewRequest("GET", requestUrl, nil)
		if errRequestCreate != nil {
			logError.Printf("[Send Request] Unable to create new request with query params. " +
				"Error: %s", errRequestCreate)
			return -1, ""
		}
	} else {
		switch contentType {
		case "application/x-www-form-urlencoded":
			data := url.Values{}
			data.Set("json", body)

			urlEncodedBody := data.Encode()

			request, errRequestCreate = http.NewRequest("POST", requestUrl, strings.NewReader(urlEncodedBody))
			if errRequestCreate != nil {
				logError.Printf("[Send Request] Unable to create new request with urlencoded body. " +
					"Error: %s", errRequestCreate)
				return -1, ""
			}

			request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Add("Content-Length", strconv.Itoa(len(urlEncodedBody)))
		case "multipart/form-data":
			multipartBody := &bytes.Buffer{}
			writer := multipart.NewWriter(multipartBody)

			writer.WriteField("json", body)

			writer.Close()

			request, errRequestCreate = http.NewRequest("POST", requestUrl, multipartBody)

			if errRequestCreate != nil {
				logError.Printf("[Send Request] Unable to create new request with multipart/form-data body. " +
					"Error: %s", errRequestCreate)
				return -1, ""
			}

			request.Header.Set("Content-Type", writer.FormDataContentType())
		}
	}

	sendingStartTime = time.Now()
	response, errResponse = myClient.Do(request)
	sendingEndTime = time.Now()

	if errResponse != nil {
		logError.Printf("[Send Request] Got error response. Error: %s", errResponse)
		return -1, ""
	}

	switch resource {
	case "/", "":
		requestTime := GetItemsRequestTime{}

		requestTime.timeWhileSendingRequest = sendingStartTime
		requestTime.elapsedTime = sendingEndTime.Sub(sendingStartTime)

		mux.Lock()
		getItemsRequestTimes = append(getItemsRequestTimes, requestTime)
		mux.Unlock()
	case "/buy":
		requestTime := BuyItemsRequestTime{}

		requestTime.timeWhileSendingRequest = sendingStartTime
		requestTime.elapsedTime = sendingEndTime.Sub(sendingStartTime)

		mux.Lock()
		buyItemsRequestTimes = append(buyItemsRequestTimes, requestTime)
		mux.Unlock()
	}

	responseBytes, _ := ioutil.ReadAll(response.Body)
	response.Body.Close()

	return response.StatusCode, string(responseBytes)
}

func BuyItems(currentClientNumber int, contentType string, items []Item) {
	for index, currentItem := range items {
		atomic.AddUint32(&totalMessagesCount, 1)

		requestBody, _ := json.Marshal(currentItem)

		responseStatusCode, responseBody := sendRequest("/buy", "", contentType, string(requestBody))

		if resultCheck := checkBuyItemResponse(currentItem.Name, responseBody, responseStatusCode);
			resultCheck != nil {

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

func startTestClient(userName, queryParam, contentType, body string, currentClientNumber int, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < testClientMessagesNum; currentMessageNumber++ {
		responseStatusCode, responseBody := sendRequest("/", queryParam, contentType, body)

		atomic.AddUint32(&totalMessagesCount, 1)

		if resultCheck := checkGetItemsResponse(userName, responseBody, responseStatusCode);
			resultCheck != nil {

			logError.Printf("[Goroutine %d][Message %d][Get Items Test] Got invalid response. "+
				"Error Message: %s", currentClientNumber, currentMessageNumber, resultCheck)

			mux.Lock()
			getItemsErrors = append(getItemsErrors, *resultCheck)
			mux.Unlock()
		} else {
			logInfo.Printf("[Goroutine %d][Message %d][Get Items Test] Got valid response. " +
				"Testing buying of received items...", currentClientNumber, currentMessageNumber)

			var parsedResponse = ResponseBody{}
			json.Unmarshal([]byte(responseBody), &parsedResponse )

			items := parsedResponse.Items

			BuyItems(currentClientNumber, contentType, items)
		}

		time.Sleep(time.Millisecond * 700)
	}
}

func makeRequestParams(clientName string) (queryParams, contentType, requestBody string) {
	availableContentTypes := []string{"application/x-www-form-urlencoded", "multipart/form-data"}

	contentType = availableContentTypes[rand.Intn(len(availableContentTypes))]

	if clientName != "" {
		if rand.Intn(2) == 1 {
			queryParams = "name=" + clientName
		}

		if queryParams == "" {
			body := map[string]string{"name": clientName}
			jsonBody, _ := json.Marshal(body)

			requestBody = string(jsonBody)
		}
	}

	return
}

func main() {
	Init()

	defaultRoundTripper := http.DefaultTransport
	defaultTransportPointer, _ := defaultRoundTripper.(*http.Transport)

	defaultTransport := *defaultTransportPointer // dereference it to get a copy of the struct that the pointer points to
	defaultTransport.MaxIdleConns = 500000
	defaultTransport.MaxIdleConnsPerHost = 500000
	defaultTransport.IdleConnTimeout = 1 * time.Second

	myClient = &http.Client{Transport: &defaultTransport}

	defer logInfoOutfile.Close()
	defer logErrorOutfile.Close()

	//--------------------
	//Warm Up A Test Ground
	//--------------------

	wgWarmUp := &sync.WaitGroup{}

	for currentClientNumber := 0; currentClientNumber < warmUpClientsNum; currentClientNumber++ {
		wgWarmUp.Add(1)

		currentClientName := requestClientNames[rand.Intn(len(requestClientNames))]

		queryParams, contentType, requestBody := makeRequestParams(currentClientName)

		go startTestClient(currentClientName, queryParams, contentType, requestBody, currentClientNumber, wgWarmUp)
	}
	time.Sleep(time.Millisecond)

	wgWarmUp.Wait()

	logInfo.Println("[MAIN] Warm up has been done")

	//--------------------
	//Load Tests
	//--------------------

	wgTest := &sync.WaitGroup{}

	for {
		if testClientsNum > testMaxClientsNum {
			logInfo.Printf("[MAIN] Reached clients limit. Stopping creating new clients...")
			break
		}

		for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
			wgTest.Add(1)

			currentClientName := requestClientNames[rand.Intn(len(requestClientNames))]

			queryParams, contentType, requestBody := makeRequestParams(currentClientName)

			go startTestClient(
				currentClientName, queryParams, contentType, requestBody, currentClientNumber, wgTest)
		}

		time.Sleep(5 * time.Second)
		testClientsNum += 50
		logInfo.Printf("[MAIN] New clients was added. Current clients count: %d", testClientsNum)
	}

	wgTest.Wait()

	//--------------------

	logInfo.Printf("[MAIN] All tests has been done. Sended requests count: %d. "+
		"Error statistics: %d errors occured during get items tests, %d errors occured during buy items tests",
		totalMessagesCount, len(getItemsErrors), len(buyItemsErrors))
}