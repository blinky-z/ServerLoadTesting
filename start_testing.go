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
	"sort"
)

var (
	logInfoOutfile, _  = os.OpenFile("./logs/Info.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	logErrorOutfile, _ = os.OpenFile("./logs/Error.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	logStatOutfile, _  = os.OpenFile("./logs/Stat.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	logInfo  = log.New(logInfoOutfile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	logError = log.New(logErrorOutfile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	logStat  = log.New(logStatOutfile, "STAT: ", log.Ldate|log.Ltime|log.Lshortfile)

	testClientsNum        int
	testClientMessagesNum int

	totalMessagesCount uint32

	getItemsErrors    []ErrResponse
	muxGetItemsErrors *sync.Mutex
	buyItemsErrors    []ErrResponse
	muxBuyItemsErrors *sync.Mutex

	getItemsResponseTimeSlice    []ResponseTime
	muxGetItemsResponseTimeSlice *sync.Mutex
	buyItemsResponseTimeSlice    []ResponseTime
	muxBuyItemsResponseTimeSlice *sync.Mutex

	myClient *http.Client

	requestClientNames = []string{"", "saneexclamation", "buythroated", "infuriatedlutchet", "ticketbright", "insecureloudmouth", "soundingindirect", "knowledgewives", "gearherring", "farmershortcrust", "variablehertz", "ripplinglens", "otherscontrol", "turnhotsprings", "veincelery", "excessfamily", "iceskatesbale", "ruffsescape", "pencilelements", "yellstable", "mushroomslomo", "edgecord", "possessivegreeting", "hertzodds", "groaninfected", "interiorrotating", "firechargeenzyme", "sickshower", "leukocytedrink", "prominencetub", "fieldsmustache", "woodcocklawful", "leatherarmy", "achernarinstance", "europalepton", "planesalami", "customersworkbench", "infinityhatching", "plughumbug", "competingfag", "farrumscut", "perpetualfallen", "unwittinglaying", "dirtycopernicium", "icehockeymeteoroid", "merseybeatstarbucks", "milkperoxide", "flingwater", "flagrantcoins", "kraftzing", "fellsargon", "bobstaysloshed", "trymercury", "freegantonic", "barnacleburnt", "masonsstrawberry", "delayedmale", "xiphoidtutor", "asheatable", "tengmalmshingles", "aquilabummage", "spotsbiceps", "violinanother", "tawnysyntax", "frogsfeisty", "nodulespity", "calledpliocene", "soddinggluttonous", "billowygillette", "stuffboson", "collarbonelargest", "parliamentblizzard", "sadmarkings", "streetsbailey", "surfernissan", "democracydividers", "alloythine", "frugalmust", "plancaplay", "normalaleutian", "stingandalusian", "skuaallee", "intendedshark", "paradigmboards", "ventureskeg", "kalmansledder", "plaindolphin", "singermention", "employvolta", "womenthorough", "huhshare", "grumpycepheus", "magnetremuda", "moralsdisrupt", "correctfierce", "rollmetrics", "skeinboiling", "amiablebiotic", "actmind", "baconsiphon", "complexvenison"}
)

const (
	//serverUrl = "http://185.143.173.31"
	serverUrl         = "http://localhost:8080"
	warmUpClientsNum  = 100
	testMaxClientsNum = 50
)

type ResponseTime struct {
	clientsNum              int
	timeWhileSendingRequest time.Time
	elapsedTime             time.Duration
}

type ErrResponse struct {
	time    time.Time
	message string
}

func (err *ErrResponse) Error() string {
	return "[" + err.time.Format("15:04:05") + "] " + err.message
}

func Init() {
	totalMessagesCount = 0

	getItemsErrors = make([]ErrResponse, 0)
	buyItemsErrors = make([]ErrResponse, 0)

	getItemsResponseTimeSlice = make([]ResponseTime, 0)
	buyItemsResponseTimeSlice = make([]ResponseTime, 0)

	muxGetItemsErrors = &sync.Mutex{}
	muxBuyItemsErrors = &sync.Mutex{}
	muxGetItemsResponseTimeSlice = &sync.Mutex{}
	muxBuyItemsResponseTimeSlice = &sync.Mutex{}

	defaultRoundTripper := http.DefaultTransport
	defaultTransportPointer, _ := defaultRoundTripper.(*http.Transport)

	defaultTransport := *defaultTransportPointer
	defaultTransport.MaxIdleConns = 5000
	defaultTransport.MaxIdleConnsPerHost = 5000

	myClient = &http.Client{Transport: &defaultTransport}
}

type ResponseBody struct {
	Nickname string `json:"nickname"`
	Items    []Item `json:"items"`
}

type Item struct {
	Name  string `json:"name"`
	Price string `json:"price"`
}

func getExpectedGetItemsResponse(userName string) string {
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

func getExpectedBuyItemsResponse(itemName string) string {
	responseBody := map[string]interface{}{}

	successPurchaseMessage := "success"
	failurePurchaseMessage := "failure"

	if len(itemName)%2 == 0 {
		responseBody["result"] = successPurchaseMessage
	} else {
		responseBody["result"] = failurePurchaseMessage
	}

	jsonBody, _ := json.Marshal(responseBody)
	return string(jsonBody)
}

func checkResponse(
	objectName, response string, statusCode int, getExpectedResponse func(objectName string) string) *ErrResponse {

	expectedResponse := getExpectedResponse(objectName)

	if statusCode != 200 {
		if statusCode == -1 {
			return &ErrResponse{time: time.Now(), message: "bad response"}
		}
		return &ErrResponse{time: time.Now(), message: "wrong status code"}
	}

	if response != expectedResponse {
		return &ErrResponse{time: time.Now(), message: "wrong response body"}
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
			logError.Printf("[Send Request] Unable to create new request with query params. "+
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
				logError.Printf("[Send Request] Unable to create new request with urlencoded body. "+
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
				logError.Printf("[Send Request] Unable to create new request with multipart/form-data body. "+
					"Error: %s", errRequestCreate)
				return -1, ""
			}

			request.Header.Set("Content-Type", writer.FormDataContentType())
		}
	}

	sendingStartTime = time.Now()
	response, errResponse = myClient.Do(request)
	sendingEndTime = time.Now()

	responseTime := ResponseTime{}
	responseTime.clientsNum = testClientsNum
	responseTime.timeWhileSendingRequest = sendingStartTime
	responseTime.elapsedTime = sendingEndTime.Sub(sendingStartTime)

	if errResponse != nil {
		logError.Printf("[Send Request] Got error response. Error message: %s", errResponse)
		return -1, ""
	}

	defer response.Body.Close()

	switch resource {
	case "/":
		muxGetItemsResponseTimeSlice.Lock()
		getItemsResponseTimeSlice = append(getItemsResponseTimeSlice, responseTime)
		muxGetItemsResponseTimeSlice.Unlock()
	case "/buy":
		muxBuyItemsResponseTimeSlice.Lock()
		buyItemsResponseTimeSlice = append(buyItemsResponseTimeSlice, responseTime)
		muxBuyItemsResponseTimeSlice.Unlock()
	}

	responseBytes, _ := ioutil.ReadAll(response.Body)
	return response.StatusCode, string(responseBytes)
}

func BuyItems(currentClientNumber int, contentType string, items []Item) {
	for index, currentItem := range items {
		atomic.AddUint32(&totalMessagesCount, 1)

		requestBody, _ := json.Marshal(currentItem)

		responseStatusCode, responseBody := sendRequest("/buy", "", contentType, string(requestBody))

		if resultCheck :=
			checkResponse(currentItem.Name, responseBody, responseStatusCode, getExpectedBuyItemsResponse);
			resultCheck != nil {

			logError.Printf("[Goroutine %d][Message %d][Buy Items Test] Got invalid response. "+
				"Error Message: %s", currentClientNumber, index, resultCheck)

			muxBuyItemsErrors.Lock()
			buyItemsErrors = append(buyItemsErrors, *resultCheck)
			muxBuyItemsErrors.Unlock()
		} else {
			logInfo.Printf("[Goroutine %d][Message %d][Buy Items Test] Got valid response",
				currentClientNumber, index)
		}
	}
}

func startTestClient(userName, queryParam, contentType, body string, currentClientNumber int, wg *sync.WaitGroup, sendDelay time.Duration) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < testClientMessagesNum; currentMessageNumber++ {
		responseStatusCode, responseBody := sendRequest("/", queryParam, contentType, body)

		atomic.AddUint32(&totalMessagesCount, 1)

		if resultCheck := checkResponse(userName, responseBody, responseStatusCode, getExpectedGetItemsResponse);
			resultCheck != nil {

			logError.Printf("[Goroutine %d][Message %d][Get Items Test] Got invalid response. "+
				"Error Message: %s", currentClientNumber, currentMessageNumber, resultCheck)

			muxGetItemsErrors.Lock()
			getItemsErrors = append(getItemsErrors, *resultCheck)
			muxGetItemsErrors.Unlock()
		} else {
			logInfo.Printf("[Goroutine %d][Message %d][Get Items Test] Got valid response. "+
				"Testing buying of received items...", currentClientNumber, currentMessageNumber)

			var parsedResponse = ResponseBody{}
			json.Unmarshal([]byte(responseBody), &parsedResponse)

			items := parsedResponse.Items

			BuyItems(currentClientNumber, contentType, items)
		}

		time.Sleep(sendDelay)
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

	defer logInfoOutfile.Close()
	defer logErrorOutfile.Close()
	defer logStatOutfile.Close()

	//--------------------
	//Warm Up A Test Ground
	//--------------------

	wgWarmUp := &sync.WaitGroup{}

	testClientsNum = 10
	testClientMessagesNum = 10

	logStat.Print("Warm Up is started")

	for currentClientNumber := 0; currentClientNumber < warmUpClientsNum; currentClientNumber++ {
		wgWarmUp.Add(1)

		currentClientName := requestClientNames[rand.Intn(len(requestClientNames))]

		queryParams, contentType, requestBody := makeRequestParams(currentClientName)
		sendDelay := time.Duration(time.Millisecond * 700)

		go startTestClient(
			currentClientName, queryParams, contentType, requestBody, currentClientNumber, wgWarmUp, sendDelay)
	}
	wgWarmUp.Wait()

	logStat.Print("[MAIN] Warm up has been done")

	resetTestGround()
	//--------------------
	//Load Tests with a large number of clients
	//--------------------

	wgTest := &sync.WaitGroup{}

	testClientsNum = 10
	testClientMessagesNum = 10

	logStat.Print("[MAIN] Load tests with a large number of clients has been started")

	for {
		if testClientsNum > testMaxClientsNum {
			logStat.Printf("[MAIN] Reached clients limit. Stopping creating new clients...")
			break
		}
		for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
			wgTest.Add(1)

			currentClientName := requestClientNames[rand.Intn(len(requestClientNames))]

			queryParams, contentType, requestBody := makeRequestParams(currentClientName)
			sendDelay := time.Duration(time.Millisecond * 700)

			go startTestClient(
				currentClientName, queryParams, contentType, requestBody, currentClientNumber, wgTest, sendDelay)
		}

		time.Sleep(5 * time.Second)
		testClientsNum += 20
		logStat.Printf("[MAIN] New clients was added. Current clients number: %d", testClientsNum)
	}

	wgTest.Wait()

	//--------------------

	logStat.Print("[MAIN] Load tests with a large number of clients has been done")
	logStat.Print("[MAIN] Load tests with a large number of clients statistics:")
	showStat()

	resetTestGround()
	//--------------------
	//Load Tests with a large number of request from each client
	//--------------------

	testClientsNum = 8
	testClientMessagesNum = 1000

	logStat.Print("[MAIN] Load tests with a large number of requests from each client has been started")

	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wgTest.Add(1)

		currentClientName := requestClientNames[rand.Intn(len(requestClientNames))]

		queryParams, contentType, requestBody := makeRequestParams(currentClientName)
		sendDelay := time.Duration(time.Millisecond * 200)

		go startTestClient(
			currentClientName, queryParams, contentType, requestBody, currentClientNumber, wgTest, sendDelay)
	}

	wgTest.Wait()

	//--------------------

	logStat.Print("[MAIN] Load tests with a large number of requests from each client has been done")
	logStat.Print("[MAIN] Load tests with a large number of requests from each client statistics:")
	showStat()
}

func resetTestGround() {
	getItemsResponseTimeSlice = nil
	buyItemsResponseTimeSlice = nil
	getItemsErrors = nil
	buyItemsErrors = nil
	totalMessagesCount = 0
}

func showStat() {
	logStat.Printf("Sent requests count: %d", totalMessagesCount)

	logStat.Printf("Error statistics: "+
		"%d errors occurred during get items tests, %d errors occurred during buy items tests",
		len(getItemsErrors), len(buyItemsErrors))

	logStat.Print("Get items requests statistics:")
	showResponseTimeSliceStat(getItemsResponseTimeSlice)

	logStat.Print("Buy items requests statistics:")
	showResponseTimeSliceStat(buyItemsResponseTimeSlice)

	var allRequestsTimeSlice []ResponseTime
	allRequestsTimeSlice = append(allRequestsTimeSlice, getItemsResponseTimeSlice...)
	allRequestsTimeSlice = append(allRequestsTimeSlice, buyItemsResponseTimeSlice...)

	logStat.Print("General requests statistics:")
	showResponseTimeSliceStat(allRequestsTimeSlice)
}

func showResponseTimeSliceStat(timeSlice []ResponseTime) {
	averageResponseTime := findAverageResponseTime(timeSlice).Seconds() * 1000
	logStat.Printf("Average response time: %f ms", averageResponseTime)

	responseTimeMedian := findTimeMedian(timeSlice).Seconds() * 1000
	logStat.Printf("Response time median: %f ms", responseTimeMedian)

	timePercentile95Value := findTimePercentile(timeSlice, 95).Seconds() * 1000
	logStat.Printf("Response time 95th percentile: %f ms", timePercentile95Value)

	showRequestsNumTimeDependency(timeSlice)
	showRequestsNumClientsNumDependency(timeSlice)
	showResponseTimeClientsNumDependency(timeSlice)
}

func showRequestsNumTimeDependency(timeSlice []ResponseTime) {
	sort.Slice(timeSlice,
		func(i, j int) bool { return timeSlice[i].timeWhileSendingRequest.Before(timeSlice[j].timeWhileSendingRequest) })

	timeRequestsStat := make(map[time.Time]int)

	intervalStartTime := timeSlice[0].timeWhileSendingRequest
	intervalTimeStep := time.Duration(time.Second)
	requestsCount := 0
	for _, currentClientTimeStat := range timeSlice {
		if currentClientTimeStat.timeWhileSendingRequest.Sub(intervalStartTime) >= intervalTimeStep {
			timeRequestsStat[currentClientTimeStat.timeWhileSendingRequest] = requestsCount
			intervalStartTime = currentClientTimeStat.timeWhileSendingRequest
		}
		requestsCount++
	}
	timeRequestsStat[timeSlice[len(timeSlice)-1].timeWhileSendingRequest] = requestsCount

	mapTimeKeys := make([]time.Time, 0)
	for currentTimeKey := range timeRequestsStat {
		mapTimeKeys = append(mapTimeKeys, currentTimeKey)
	}
	sort.Slice(mapTimeKeys,
		func(i, j int) bool { return mapTimeKeys[i].Before(mapTimeKeys[j]) })

	logStat.Print("Statistics of the number of requests in a certain time:")
	for _, currentTime := range mapTimeKeys {
		logStat.Printf("[" + currentTime.Format("15:04:05") + "] " +
			strconv.Itoa(timeRequestsStat[currentTime]) + " requests")
	}
}

func showRequestsNumClientsNumDependency(timeSlice []ResponseTime) {
	sort.Slice(timeSlice,
		func(i, j int) bool { return timeSlice[i].timeWhileSendingRequest.Before(timeSlice[j].timeWhileSendingRequest) })

	timeClientsRequestsStat := make(map[int]int)

	intervalStartClientsNum := timeSlice[0].clientsNum
	requestsCount := 0
	for _, currentClientTimeStat := range timeSlice {
		if currentClientTimeStat.clientsNum != intervalStartClientsNum {
			timeClientsRequestsStat[currentClientTimeStat.clientsNum] = requestsCount
			intervalStartClientsNum = currentClientTimeStat.clientsNum
		}
		requestsCount++
	}
	timeClientsRequestsStat[timeSlice[len(timeSlice)-1].clientsNum] = requestsCount

	mapClientsNumKeys := make([]int, 0)
	for currentClientsNumKey := range timeClientsRequestsStat {
		mapClientsNumKeys = append(mapClientsNumKeys, currentClientsNumKey)
	}
	sort.Slice(mapClientsNumKeys,
		func(i, j int) bool { return mapClientsNumKeys[i] < mapClientsNumKeys[j] })

	logStat.Print("Statistics of the number of requests at a certain number of clients:")
	for _, currentClientsNum := range mapClientsNumKeys {
		logStat.Printf("[%d clients] %d requests", currentClientsNum, timeClientsRequestsStat[currentClientsNum])
	}
}

func showResponseTimeClientsNumDependency(timeSlice []ResponseTime) {
	sort.Slice(timeSlice,
		func(i, j int) bool { return timeSlice[i].timeWhileSendingRequest.Before(timeSlice[j].timeWhileSendingRequest) })

	averageResponseTimeStat := make(map[int]time.Duration)
	responseTimeMedianStat := make(map[int]time.Duration)
	response95thPercentileStat := make(map[int]time.Duration)

	intervalStartClientsNum := timeSlice[0].clientsNum
	for index, currentClientTimeStat := range timeSlice {
		if currentClientTimeStat.clientsNum != intervalStartClientsNum {
			currentAverageResponseTime := findAverageResponseTime(timeSlice[:index+1])
			currentTimeMedian := findTimeMedian(timeSlice[:index+1])
			current95thPercentile := findTimePercentile(timeSlice[:index+1], 95)

			responseTimeMedianStat[currentClientTimeStat.clientsNum] = currentTimeMedian
			averageResponseTimeStat[currentClientTimeStat.clientsNum] = currentAverageResponseTime
			response95thPercentileStat[currentClientTimeStat.clientsNum] = current95thPercentile

			intervalStartClientsNum = currentClientTimeStat.clientsNum
		}
	}
	averageResponseTimeStat[timeSlice[len(timeSlice)-1].clientsNum] = findAverageResponseTime(timeSlice)
	responseTimeMedianStat[timeSlice[len(timeSlice)-1].clientsNum] = findTimeMedian(timeSlice)
	response95thPercentileStat[timeSlice[len(timeSlice)-1].clientsNum] = findTimePercentile(timeSlice, 95)

	mapClientsNumKeys := make([]int, 0)
	for currentClientsNumKey := range averageResponseTimeStat {
		mapClientsNumKeys = append(mapClientsNumKeys, currentClientsNumKey)
	}
	sort.Slice(mapClientsNumKeys,
		func(i, j int) bool { return mapClientsNumKeys[i] < mapClientsNumKeys[j] })

	logStat.Print("Average response time statistics at a certain number of clients:")
	for _, currentClientsNum := range mapClientsNumKeys {
		logStat.Printf("[%d clients] Average response time: %f ms",
			currentClientsNum, averageResponseTimeStat[currentClientsNum].Seconds()*1000)
	}

	logStat.Print("Response time median statistics at a certain number of clients:")
	for _, currentClientsNum := range mapClientsNumKeys {
		logStat.Printf("[%d clients] Response time median: %f ms",
			currentClientsNum, responseTimeMedianStat[currentClientsNum].Seconds()*1000)
	}

	logStat.Print("Response time 95th percentile at a certain number of clients:")
	for _, currentClientsNum := range mapClientsNumKeys {
		logStat.Printf("[%d clients] Response time 95th percentile: %f ms",
			currentClientsNum, response95thPercentileStat[currentClientsNum].Seconds()*1000)
	}
}

func findTimePercentile(timeSlice []ResponseTime, percentile float32) time.Duration {
	sort.Slice(timeSlice,
		func(i, j int) bool { return timeSlice[i].elapsedTime < timeSlice[j].elapsedTime })

	percentileValuePosition := int(percentile/100*float32(len(timeSlice))+0.5) - 1

	return timeSlice[percentileValuePosition].elapsedTime
}

func findTimeMedian(timeSlice []ResponseTime) time.Duration {
	sort.Slice(timeSlice,
		func(i, j int) bool { return timeSlice[i].elapsedTime < timeSlice[j].elapsedTime })

	if len(timeSlice)%2 != 0 {
		return timeSlice[(len(timeSlice)+1)/2].elapsedTime
	} else {
		return (timeSlice[len(timeSlice)/2].elapsedTime + timeSlice[len(timeSlice)/2+1].elapsedTime) / 2
	}
}

func findAverageResponseTime(timeSlice []ResponseTime) time.Duration {
	var totalTime time.Duration

	for _, currentResponseTime := range timeSlice {
		totalTime += currentResponseTime.elapsedTime
	}

	return totalTime / time.Duration(len(timeSlice))
}
