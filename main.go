package main

import (
	"database/sql"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const (
	POLL_INTERVAL = 3 * time.Second
	USER_ID = 1
	SPOT = iota
	HOUR
	DAY
	WEEK
)

var (
	ENGINE = NewSqlEngine()
)

type Watch struct {
	AssetID   string
	UserID    int
	Threshold float32
}


type Alert struct {
	UserID  int
	Message string
}

func PopulateWatches() {
	ENGINE.CreateWatch("bitcoin", USER_ID, 1.0)
	ENGINE.CreateWatch("bitcoin", USER_ID, 2.0)
	ENGINE.CreateWatch("bitcoin", USER_ID, 4.0)
	ENGINE.CreateWatch("bitcoin", USER_ID, 8.0)
	ENGINE.CreateWatch("bitcoin", USER_ID, 16.0)
	ENGINE.CreateWatch("bitcoin", USER_ID, 110.0)
	/*
	ENGINE.CreateWatch("ethereum", USER_ID, 2.0)
	ENGINE.CreateWatch("bitcoin-cash", USER_ID, 3.0)
	ENGINE.CreateWatch("litecoin", USER_ID, 4.0)
	ENGINE.CreateWatch("ethereum-classic", USER_ID, 5.0)
	ENGINE.CreateWatch("ripple", USER_ID, 6.0)
	ENGINE.CreateWatch("eos", USER_ID, 7.0)
	ENGINE.CreateWatch("stellar", USER_ID, 8.0)
	ENGINE.CreateWatch("tether", USER_ID, 9.0)
	ENGINE.CreateWatch("cardano", USER_ID, 10.0)
        */
}

type SqlEngine struct {
	Connection *sql.DB
}

func NewSqlEngine() *SqlEngine {
	conn, err := sql.Open(
		"postgres",
		"postgresql://postgres-dev:Z3R0C00L@localhost:/price-watch?sslmode=disable",
	)
	if err != nil {
		log.Panic(err)
	}

	return &SqlEngine{conn}
}

func (s SqlEngine) CreateWatch(assetID string, userID int, threshold float32) *Watch {
	statement := `
INSERT INTO asset_watches(user_id, asset_id, threshold)
VALUES ($1, $2, $3)
RETURNING id, user_id, asset_id, threshold;`
	dbID := 0
	dbAssetID := ""
	dbUserID := 0
	dbThreshold := float32(0)
	err := s.Connection.QueryRow(statement, userID, assetID, threshold).Scan(&dbID, &dbUserID, &dbAssetID, &dbThreshold)
	if err != nil {
		log.Panic(err)
	}
	return &Watch{dbAssetID, dbUserID, dbThreshold}
}

func (s SqlEngine) WatchesForRange(assetID string, low, high float32) []Watch {
	query := `SELECT user_id, asset_id, threshold FROM asset_watches WHERE asset_id = $1 AND threshold BETWEEN $2 AND $3;`
	rows, err := s.Connection.Query(query, assetID, low, high)
	if err != nil {
		log.Panic(err)
	}
	defer rows.Close()

	var watch Watch
	var watches []Watch

	for rows.Next() {
		err = rows.Scan(
			&watch.UserID,
			&watch.AssetID,
			&watch.Threshold,
		)

		if err != nil {
			log.Panic(err)
		}

		watches = append(watches, watch)
	}
	return watches
}

type ExchangeClient struct {
	Client *http.Client
}

type AssetSummary struct {
	ID string `json:"id"`
	Base string `json:"base"`
	Name string `json:"name"`
	Currency string `json:"currency"`
	MarketCap string `json:"market_cap"`
	PercentChange float32 `json:"percent_change"`
	Latest string `json:"latest"`
}

type AssetSummaryResponse struct {
	Data *AssetSummary `json:"data"`
}

func (e ExchangeClient) SpotPrice() float32 {
	summary := &AssetSummaryResponse{}
	payload, err := e.JSONRequest("http://api.coinbase.com/v2/assets/summary/7b11fea3-4784-54a7-bc33-280c38fff18e")
	if err != nil {
		log.Panic(err)
	}

	json.Unmarshal(payload, summary)
	latest, _ := strconv.ParseFloat(summary.Data.Latest, 32)
	return float32(latest)
}

func (e *ExchangeClient) JSONRequest(URL string) ([]byte, error) {
	var byteStream []byte

	request, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return byteStream, err
	}

	response, err := e.Client.Do(request)
	if err != nil {
		return byteStream, err
	}
	if response.StatusCode != http.StatusOK {
		return byteStream, fmt.Errorf(
			"URL: '%s' returned unexpected status code %d", URL, response.StatusCode)
	}

	var reader io.ReadCloser
	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(response.Body)
		if err != nil {
			return byteStream, err
		}
		defer reader.Close()
	default:
		reader = response.Body
	}

	return ioutil.ReadAll(reader)
}

func PercentWorker(wg *sync.WaitGroup, alert chan Alert) {
	for {
		client := &ExchangeClient{&http.Client{Timeout: time.Second * 5}}
		spotPrice := client.SpotPrice()
		priorPrice := spotPrice / 1.15
		fmt.Println("priorPrice is ", priorPrice)

		activatedWatches := ENGINE.WatchesForRange("bitcoin", priorPrice, spotPrice)

		fmt.Printf("percent activatedWatches is: %v\n", activatedWatches)
		for _, watch := range activatedWatches {
			message := fmt.Sprintf("Notification for asset %s has triggered an alert at price point %f.", watch.AssetID, watch.Threshold)
			alert <- Alert{watch.UserID, message}
		}	

		time.Sleep(POLL_INTERVAL)
	}
}

func ThresholdWorker(wg *sync.WaitGroup, alert chan Alert) {
	for {
		client := &ExchangeClient{&http.Client{Timeout: time.Second * 5}}
		spotPrice := client.SpotPrice()
		priorPrice := client.SpotPrice()

		activatedWatches := ENGINE.WatchesForRange("bitcoin", priorPrice, spotPrice)

		fmt.Printf("threshold activatedWatches is: %v\n", activatedWatches)
		for _, watch := range activatedWatches {
			message := fmt.Sprintf("Notification for asset %s has triggered an alert at price point %f.", watch.AssetID, watch.Threshold)
			alert <- Alert{watch.UserID, message}
		}

		time.Sleep(POLL_INTERVAL)
	}
}

func main() {
	ENGINE.Connection.QueryRow(`DELETE FROM asset_watches`)
	PopulateWatches()
	wg := &sync.WaitGroup{}
	wg.Add(2)
	alertsPipeline := make(chan Alert)
	// go ThresholdWorker(wg)
	go PercentWorker(wg, alertsPipeline)
	wg.Wait()
	for alert := range alertsPipeline {
		fmt.Printf("New alert triggered for user %d. Message: %s.\n", alert.UserID, alert.Message)
	}
	// Store a time-based snapshot of Asset Spot Prices in Redis
	// Get current Asset Spot Price
	// Find prior spot price
	// Select and notify all watches that fall within range of
 	// Prior and current spot price
}
