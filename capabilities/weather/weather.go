package weather

import (
	"context"
	"strings"

	"github.com/kacperpaczos/agents/tool"
)

// GetWeatherTool returns a tool that looks up simulated weather data for a city.
func GetWeatherTool() tool.Tool {
	return tool.Func("get_weather", "Get the current weather for a given city.").
		StringParam("city", "City name, e.g. 'London'.", true).
		Handler(func(_ context.Context, args map[string]any, _ map[string]any) (map[string]any, error) {
			city, _ := args["city"].(string)
			data := map[string]map[string]any{
				"london":   {"temperature": "15°C", "condition": "Rainy", "humidity": "80%"},
				"new york": {"temperature": "22°C", "condition": "Sunny", "humidity": "45%"},
				"tokyo":    {"temperature": "18°C", "condition": "Cloudy", "humidity": "70%"},
			}
			if w, ok := data[strings.ToLower(city)]; ok {
				w["city"] = city
				return w, nil
			}
			return map[string]any{
				"city":        city,
				"temperature": "20°C",
				"condition":   "Unknown",
				"humidity":    "50%",
			}, nil
		}).
		Build()
}
