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

	requestClientNames = []string{"saneexclamation","buythroated","infuriatedlutchet","ticketbright","insecureloudmouth","soundingindirect","knowledgewives","gearherring","farmershortcrust","variablehertz","ripplinglens","otherscontrol","turnhotsprings","veincelery","excessfamily","iceskatesbale","ruffsescape","pencilelements","yellstable","mushroomslomo","edgecord","possessivegreeting","hertzodds","groaninfected","interiorrotating","firechargeenzyme","sickshower","leukocytedrink","prominencetub","fieldsmustache","woodcocklawful","leatherarmy","achernarinstance","europalepton","planesalami","customersworkbench","infinityhatching","plughumbug","competingfag","farrumscut","perpetualfallen","unwittinglaying","dirtycopernicium","icehockeymeteoroid","merseybeatstarbucks","milkperoxide","flingwater","flagrantcoins","kraftzing","fellsargon","bobstaysloshed","trymercury","freegantonic","barnacleburnt","masonsstrawberry","delayedmale","xiphoidtutor","asheatable","tengmalmshingles","aquilabummage","spotsbiceps","violinanother","tawnysyntax","frogsfeisty","nodulespity","calledpliocene","soddinggluttonous","billowygillette","stuffboson","collarbonelargest","parliamentblizzard","sadmarkings","streetsbailey","surfernissan","democracydividers","alloythine","frugalmust","plancaplay","normalaleutian","stingandalusian","skuaallee","intendedshark","paradigmboards","ventureskeg","kalmansledder","plaindolphin","singermention","employvolta","womenthorough","huhshare","grumpycepheus","magnetremuda","moralsdisrupt","correctfierce","rollmetrics","skeinboiling","amiablebiotic","actmind","baconsiphon","complexvenison"}
)

const (
	serverUrl = "http://185.143.173.31"
	warmUpClientsNum = 100
	testMaxClientsNum = 200

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
	totalMessagesCount = 0

	getItemsErrors = make([]ErrGetItems, 0)
	buyItemsErrors = make([]ErrBuyItems, 0)

	getItemsRequestTimes = make([]GetItemsRequestTime, 0)
	buyItemsRequestTimes = make([]BuyItemsRequestTime, 0)

	mux = &sync.Mutex{}
}

type GetItemsRequestTime struct {
	clientsNumberWhileSendingRequest int
	timeWhileSendingRequest          time.Time
	elapsedTime                      time.Duration
}

type BuyItemsRequestTime struct {
	clientsNumberWhileSendingRequest int
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

func sendRequest(path, queryParams, contentType, body string) (statusCode int, responseBody string) {
	requestUrl := serverUrl + path

	var response *http.Response

	var sendingStartTime time.Time
	var sendingEndTime time.Time
	var clientsNum int

	if body == "" {
		if queryParams != "" {
			requestUrl += "?" + queryParams
		}

		sendingStartTime = time.Now()
		clientsNum = testClientsNum

		response, _ = http.Get(requestUrl)

		sendingEndTime = time.Now()
	} else {
		switch contentType {
		case "application/x-www-form-urlencoded":
			sendingStartTime = time.Now()
			clientsNum = testClientsNum

			response, _ = http.PostForm(requestUrl, url.Values{"json": {body}})

			sendingEndTime = time.Now()
		case "multipart/form-data":
			client := &http.Client{}

			multipartBody := &bytes.Buffer{}
			writer := multipart.NewWriter(multipartBody)

			writer.WriteField("json", body)

			writer.Close()

			request, _ := http.NewRequest("POST", requestUrl, multipartBody)
			request.Header.Set("Content-Type", writer.FormDataContentType())

			sendingStartTime = time.Now()
			clientsNum = testClientsNum

			response, _ = client.Do(request)

			sendingEndTime = time.Now()
		}
	}

	switch path {
	case "/", "":
		requestTime := GetItemsRequestTime{}

		requestTime.clientsNumberWhileSendingRequest = clientsNum
		requestTime.timeWhileSendingRequest = sendingStartTime
		requestTime.elapsedTime = sendingEndTime.Sub(sendingStartTime)

		mux.Lock()
		getItemsRequestTimes = append(getItemsRequestTimes, requestTime)
		mux.Unlock()
	case "/buy":
		requestTime := BuyItemsRequestTime{}

		requestTime.clientsNumberWhileSendingRequest = clientsNum
		requestTime.timeWhileSendingRequest = sendingStartTime
		requestTime.elapsedTime = sendingEndTime.Sub(sendingStartTime)

		mux.Lock()
		buyItemsRequestTimes = append(buyItemsRequestTimes, requestTime)
		mux.Unlock()
	}

	defer response.Body.Close()
	responseBytes, _ := ioutil.ReadAll(response.Body)

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

	if rand.Intn(2) == 1 {
		queryParams = "name=" + clientName
	}

	if queryParams == "" {
		body := map[string]string{"name": clientName}
		jsonBody, _ := json.Marshal(body)

		requestBody = string(jsonBody)
	}

	return
}

func main() {
	Init()
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

		time.Sleep(10 * time.Second)
		testClientsNum += 50
		logInfo.Printf("[MAIN] Added new clients. Current clients count: %d", testClientsNum)
	}

	wgTest.Wait()

	//--------------------

	logInfo.Printf("[MAIN] All tests has been done. Sended requests count: %d. "+
		"Error statistics: %d errors occured during get items tests, %d errors occured during buy items tests",
		totalMessagesCount, len(getItemsErrors), len(buyItemsErrors))
}