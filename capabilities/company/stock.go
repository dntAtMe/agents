package company

import (
	"fmt"
	"strings"
)

// StockEntry records a single stock price change.
type StockEntry struct {
	Round     int
	Price     float64
	Delta     float64
	Sentiment string
}

// StockTracker tracks the company stock price over time.
type StockTracker struct {
	History   []StockEntry
	Current   float64
	Sentiment string
}

// NewStockTracker creates a StockTracker with an initial price.
func NewStockTracker(initialPrice float64) *StockTracker {
	return &StockTracker{
		Current:   initialPrice,
		Sentiment: "neutral",
		History: []StockEntry{
			{Round: 0, Price: initialPrice, Delta: 0, Sentiment: "neutral"},
		},
	}
}

// Update records a new stock price and sentiment.
func (st *StockTracker) Update(round int, newPrice float64, sentiment string) {
	delta := newPrice - st.Current
	st.Current = newPrice
	st.Sentiment = sentiment
	st.History = append(st.History, StockEntry{
		Round:     round,
		Price:     newPrice,
		Delta:     delta,
		Sentiment: sentiment,
	})
}

// Render returns a markdown table of the full price history.
func (st *StockTracker) Render() string {
	var sb strings.Builder
	sb.WriteString("## Stock Price History\n\n")
	sb.WriteString("| Round | Price | Delta | Sentiment |\n")
	sb.WriteString("|-------|-------|-------|-----------|\n")
	for _, e := range st.History {
		sign := ""
		if e.Delta > 0 {
			sign = "+"
		}
		sb.WriteString(fmt.Sprintf("| %d | $%.2f | %s%.2f | %s |\n",
			e.Round, e.Price, sign, e.Delta, e.Sentiment))
	}
	return sb.String()
}

// RenderBrief returns a one-line summary: current price, last delta, sentiment.
func (st *StockTracker) RenderBrief() string {
	if len(st.History) == 0 {
		return fmt.Sprintf("Stock: $%.2f | %s", st.Current, st.Sentiment)
	}
	last := st.History[len(st.History)-1]
	arrow := "→"
	sign := ""
	if last.Delta > 0 {
		arrow = "↑"
		sign = "+"
	} else if last.Delta < 0 {
		arrow = "↓"
	}
	return fmt.Sprintf("Stock: $%.2f %s %s%.2f | Sentiment: %s",
		st.Current, arrow, sign, last.Delta, st.Sentiment)
}
