package usage

// ModelPricing holds per-token costs for a chat model (USD).
type ModelPricing struct {
	InputPerToken  float64
	OutputPerToken float64
}

// ImagePricing holds per-image costs keyed by size (USD).
type ImagePricing struct {
	PricePerImage map[string]float64
}

var chatPricing = map[string]ModelPricing{
	"gpt-4o-mini": {
		InputPerToken:  0.00000015, // $0.15 / 1M input tokens
		OutputPerToken: 0.0000006,  // $0.60 / 1M output tokens
	},
	"gpt-4o": {
		InputPerToken:  0.0000025, // $2.50 / 1M input tokens
		OutputPerToken: 0.00001,   // $10.00 / 1M output tokens
	},
}

var imagePricing = map[string]ImagePricing{
	"dall-e-3": {
		PricePerImage: map[string]float64{
			"1024x1024": 0.040,
			"1024x1792": 0.080,
			"1792x1024": 0.080,
		},
	},
	"dall-e-2": {
		PricePerImage: map[string]float64{
			"256x256":   0.016,
			"512x512":   0.018,
			"1024x1024": 0.020,
		},
	},
	"gpt-image-1": {
		PricePerImage: map[string]float64{
			"1024x1024": 0.042,
			"1024x1536": 0.063,
			"1536x1024": 0.063,
			"auto":      0.042,
		},
	},
}

// CostForTokens calculates the cost for a chat completion.
func CostForTokens(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := chatPricing[model]
	if !ok {
		pricing = chatPricing["gpt-4o-mini"]
	}
	return float64(inputTokens)*pricing.InputPerToken + float64(outputTokens)*pricing.OutputPerToken
}

// CostForImageGeneration calculates the cost for image generation.
func CostForImageGeneration(model, size string, n int) float64 {
	pricing, ok := imagePricing[model]
	if !ok {
		// Fallback: try gpt-image-1 pricing for unknown image models
		pricing, ok = imagePricing["gpt-image-1"]
		if !ok {
			return 0
		}
	}
	price, ok := pricing.PricePerImage[size]
	if !ok {
		// Use first available price as fallback
		for _, p := range pricing.PricePerImage {
			price = p
			break
		}
	}
	return price * float64(n)
}
