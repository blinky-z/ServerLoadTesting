package main

import (
	"sync"
	"net/http"
	"fmt"
	"strconv"
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"
)

const (
	messagesNum = 10
	warmUpClientsNum = 5
	testClientsNum = 10
)

type good struct {
	Name  string `json:"name"`
	Price string `json:"price"`
}

type anonymousClientResponseBody struct {
	Items []good `json:"items"`
}

type certainClientResponseBody struct {
	Nickname string `json:"nickname"`
	Items    []good `json:"items"`
}

type ResponseBody struct {
	Nickname string `json:"nickname"`
	Items    []good `json:"items"`
}

func getExpectedBodyAnonymousClient() string {
	var defaultGoodsNumber int = 5
	var multiplier int = 30

	responseBody := &anonymousClientResponseBody{}

	var goods []good = make([]good, defaultGoodsNumber)

	for currentGoodNumber := 0; currentGoodNumber < defaultGoodsNumber; currentGoodNumber++ {
		newGood := good{}
		newGood.Name = "default" + strconv.Itoa(currentGoodNumber)
		newGood.Price = strconv.Itoa(currentGoodNumber * multiplier)

		goods[currentGoodNumber] = newGood
	}

	responseBody.Items = goods

	jsonBody, _ := json.Marshal(responseBody)

	response := string(jsonBody)

	return response
}

func getExpectedResponseBody(userName string) string {
	const defaultGoodsNumber int = 5
	var multiplier int

	responseBody := &ResponseBody{}

	if userName != "" {
		goods := make([]good, len(userName))

		responseBody.Nickname = userName
		for _, charValue := range userName {
			multiplier += int(charValue)
		}

		for currentGoodNumber := 0; currentGoodNumber < len(goods); currentGoodNumber++ {
			newGood := good{}
			newGood.Name = userName + strconv.Itoa(currentGoodNumber)
			newGood.Price = strconv.Itoa((currentGoodNumber + 1) * multiplier)

			goods[currentGoodNumber] = newGood
		}

		responseBody.Items = goods

	} else {
		goods := make([]good, defaultGoodsNumber)

		for currentGoodNumber := 0; currentGoodNumber < len(goods); currentGoodNumber++ {
			newGood := good{}
			newGood.Name = "default" + strconv.Itoa(currentGoodNumber)
			newGood.Price = strconv.Itoa(currentGoodNumber * multiplier)

			goods[currentGoodNumber] = newGood
		}

		responseBody.Items = goods
	}

	jsonBody, _ := json.Marshal(responseBody)

	response := string(jsonBody)

	return response
}

func checkClientResponse(response *http.Response) error {
	expectedBody := getExpectedBodyAnonymousClient()

	receivedResponseByteBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	var convertedReceivedResponseBody = string(receivedResponseByteBody)

	if response.StatusCode != 200 {
		return errors.New("wrong status code")
	}

	if convertedReceivedResponseBody != expectedBody {
		return errors.New("wrong response body")
	}

	return nil
}

func startAnonymousTestClient(currentClientNumber int, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < messagesNum; currentMessageNumber++ {
		response, err := http.Get("http://localhost:8080/")
		if err != nil {
			panic(err)
		}

		fmt.Printf("[Goroutine %d] Message %d was successfully sent", currentClientNumber, currentMessageNumber)

		if resultCheck := checkClientResponse(response); resultCheck != nil {
			fmt.Printf("[Goroutine %d][Message %d] Got invalid response. Error Message: %s\n", currentMessageNumber, currentClientNumber, resultCheck)
		} else {
			fmt.Printf("[Goroutine %d][Message %d] Got valid response\n", currentMessageNumber, currentClientNumber)
		}
	}
}

func startWarmUpClient(currentClientNumber int, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentMessageNumber := 0; currentMessageNumber < messagesNum; currentMessageNumber++ {
		_, err := http.Get("http://localhost:8080/")

		if err != nil {
			panic(err)
		}

		fmt.Println("Message", currentMessageNumber, "was sent from goroutine", currentClientNumber)
	}
}

func main() {
	wg := &sync.WaitGroup{}

	// create clients to warm up a test ground
	for currentClientNumber := 0; currentClientNumber < warmUpClientsNum; currentClientNumber++ {
		wg.Add(1)
		go startWarmUpClient(currentClientNumber, wg)
	}
	time.Sleep(time.Millisecond)

	// create clients for web server load testing
	for currentClientNumber := 0; currentClientNumber < testClientsNum; currentClientNumber++ {
		wg.Add(1)
		go startAnonymousTestClient(currentClientNumber, wg)
	}
	time.Sleep(time.Millisecond)

	wg.Wait()

	fmt.Println("[MAIN] All tests are passed")
}