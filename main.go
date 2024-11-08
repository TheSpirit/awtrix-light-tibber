package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"
)

var tibberDemoToken = "5K4MVS-OjfWhK_4yrjOlFe1F6kJXPVf7eQYggo8ebAE"
var flagTibberToken = flag.String("tibberToken", lookupEnv("TIBBER_TOKEN", tibberDemoToken), "Your Tibber developer API token")
var flagAwtrixIP = flag.String("awtrixIP", lookupEnv("AWTRIX_IP", "127.0.0.1"), "The IPv4 address of your Awtrix light device")

var customAppName = "tibberPrices"
var chartBarCount = 36 - 12

var knownPrices []tibberPrice

func main() {
	flag.Parse()
	if *flagTibberToken == tibberDemoToken {
		log.Print("Using Tibber demo token. Please provide your own developer token via --tibberToken for real data")
	}

	for {
		fetchPrices()
		updateKnowPrices()
		updateDisplay()

		log.Printf("Sleeping for 1 hour")
		time.Sleep(1 * time.Minute)
	}
}

func fetchPrices() {
	log.Println("Fetching Tibber prices...")
	prices, err := readPrices(*flagTibberToken)
	if err != nil {
		log.Fatalf("Could not fetch prices: %v", err)
	}

	knownPrices = prices
}

func updateKnowPrices() {
	if len(knownPrices) == 0 {
		return
	}

	historicPrices, upcomingPrices := splitPrices(knownPrices)

	// Limit historic prices to the last 4
	if len(historicPrices) >= 4 {
		historicPrices = historicPrices[len(historicPrices)-4:]
	}
	relevantPrices := append(historicPrices, upcomingPrices...)

	log.Print("Updating known prices")
	knownPrices = relevantPrices
}

func updateDisplay() {
	relevantPrices := knownPrices
	if len(relevantPrices) > chartBarCount {
		relevantPrices = relevantPrices[:chartBarCount]
	}

	// Print prices
	log.Printf("Identified the following relevant prices")
	for _, price := range relevantPrices {
		log.Printf("Starting at %s: %f", price.StartsAt, price.Total)
	}

	currentPriceString := "?"
	currentPrice, err := currentPrice(relevantPrices)
	if err == nil {
		// currentPriceString = fmt.Sprintf("%d¢", roundedPrice(currentPrice.Total))
		currentPriceString = fmt.Sprintf(" %d", roundedPrice(currentPrice.Total))
	}

	commandsText := []AwtrixDrawCommand{{Command: "dt", X: 0, Y: 1, Text: currentPriceString, Color: "#FFFFFF"}}
	commandsChart := mapToDrawingCommands(relevantPrices)
	app := AwtrixApp{Draw: append(commandsText, commandsChart...)}

	log.Printf("Drawing %d prices...", len(commandsChart))
	err = postApplication(*flagAwtrixIP, customAppName, app)
	if err != nil {
		log.Fatalf("Could not update custom application: %v", err)
	}
}

func splitPrices(prices []tibberPrice) ([]tibberPrice, []tibberPrice) {
	var historicPrices []tibberPrice
	var upcomingPrices []tibberPrice

	for _, price := range prices {
		if price.StartsAt.Before(time.Now()) {
			historicPrices = append(historicPrices, price)
		} else if price.StartsAt.After(time.Now()) {
			upcomingPrices = append(upcomingPrices, price)
		} else {
			log.Fatalf("Can't place price %+v", price)
		}
	}

	return historicPrices, upcomingPrices
}

func currentPrice(prices []tibberPrice) (tibberPrice, error) {
	for _, price := range prices {
		if price.StartsAt.Day() == time.Now().Day() && price.StartsAt.Hour() == time.Now().Hour() {
			return price, nil
		}
	}

	return tibberPrice{}, fmt.Errorf("could not find current price")
}

func mapToDrawingCommands(prices []tibberPrice) []AwtrixDrawCommand {
	var commands []AwtrixDrawCommand

	if len(prices) == 0 {
		return commands
	}

	// Find min and max price
	minPrice := prices[0].Total
	maxPrice := prices[0].Total
	for _, price := range prices {
		if price.Total < minPrice {
			minPrice = price.Total
		}
		if price.Total > maxPrice {
			maxPrice = price.Total
		}

	}

	// Map price range to pixel range
	yMin := 1
	yMax := 8
	slope := 1.0 * float64(yMax-yMin) / (maxPrice - minPrice)
	xOffset := 12

	for i, price := range prices {
		scaledPrice := float64(yMin) + slope*(price.Total-minPrice)
		color := mapPriceToColor(price)
		log.Printf("Mapping price %f to %d (Min: %f, Max: %f, Color: %s)", price.Total, int(scaledPrice), minPrice, maxPrice, color)
		command := AwtrixDrawCommand{Command: "df", X: xOffset + i, Y: yMax - int(scaledPrice), Width: 1, Height: yMax, Color: color}
		commands = append(commands, command)
	}

	return commands
}

func roundedPrice(price float64) int {
	return int(math.Round(price * 100))
}

func mapPriceToColor(price tibberPrice) string {
	if price.StartsAt.Day() == time.Now().Day() && price.StartsAt.Hour() == time.Now().Hour() {
		return "#FFFFFF"
	}

	switch {
	case price.Total <= 0.20:
		return "#6464ff"
	case price.Total < 0.25:
		return "#00ff00"
	case price.Total < 0.30:
		return "#ffff00"
	case price.Total < 0.35:
		return "#ff8000"
	case price.Total < 0.4:
		return "#ff0000"
	default:
		return "#800080"
	}
}
