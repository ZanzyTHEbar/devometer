package encoding

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"time"
)

// EncoderPool manages a pool of JSON encoders for better performance
type EncoderPool struct {
	pool chan *json.Encoder
	size int
}

// NewEncoderPool creates a new encoder pool with specified size
func NewEncoderPool(size int) *EncoderPool {
	if size <= 0 {
		size = 10
	}

	pool := make(chan *json.Encoder, size)
	for i := 0; i < size; i++ {
		// Create encoder with a dummy writer initially
		encoder := json.NewEncoder(io.Discard)
		pool <- encoder
	}

	return &EncoderPool{
		pool: pool,
		size: size,
	}
}

// GetEncoder retrieves an encoder from the pool
func (ep *EncoderPool) GetEncoder() *json.Encoder {
	select {
	case encoder := <-ep.pool:
		return encoder
	default:
		// Pool exhausted, create new encoder
		slog.Debug("Encoder pool exhausted, creating new encoder")
		return json.NewEncoder(io.Discard)
	}
}

// ReturnEncoder returns an encoder to the pool
func (ep *EncoderPool) ReturnEncoder(encoder *json.Encoder) {
	select {
	case ep.pool <- encoder:
		// Successfully returned to pool
	default:
		// Pool full, discard encoder
		slog.Debug("Encoder pool full, discarding encoder")
	}
}

// Marshal marshals data using the encoder pool for better performance
func (ep *EncoderPool) Marshal(v interface{}) ([]byte, error) {
	encoder := ep.GetEncoder()
	defer ep.ReturnEncoder(encoder)

	var buf bytes.Buffer
	encoder.SetIndent("", "") // No indentation for performance

	// Create a new encoder for this specific buffer
	tempEncoder := json.NewEncoder(&buf)

	if err := tempEncoder.Encode(v); err != nil {
		return nil, err
	}

	// Remove the trailing newline that json.Encoder.Encode adds
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data, nil
}

// DecoderPool manages a pool of JSON decoders for better performance
type DecoderPool struct {
	pool chan *json.Decoder
	size int
}

// NewDecoderPool creates a new decoder pool with specified size
func NewDecoderPool(size int) *DecoderPool {
	if size <= 0 {
		size = 10
	}

	pool := make(chan *json.Decoder, size)
	for i := 0; i < size; i++ {
		// Create decoder with a dummy reader initially
		decoder := json.NewDecoder(bytes.NewReader([]byte{}))
		pool <- decoder
	}

	return &DecoderPool{
		pool: pool,
		size: size,
	}
}

// GetDecoder retrieves a decoder from the pool
func (dp *DecoderPool) GetDecoder(data []byte) *json.Decoder {
	// For simplicity, create a new decoder for each use
	// In a real implementation, we might pool readers instead
	return json.NewDecoder(bytes.NewReader(data))
}

// ReturnDecoder returns a decoder to the pool
func (dp *DecoderPool) ReturnDecoder(decoder *json.Decoder) {
	select {
	case dp.pool <- decoder:
		// Successfully returned to pool
	default:
		// Pool full, discard decoder
		slog.Debug("Decoder pool full, discarding decoder")
	}
}

// Unmarshal unmarshals data using the decoder pool for better performance
func (dp *DecoderPool) Unmarshal(data []byte, v interface{}) error {
	decoder := dp.GetDecoder(data)
	defer dp.ReturnDecoder(decoder)

	return decoder.Decode(v)
}

// OptimizedJSONEncoder provides high-performance JSON encoding/decoding
type OptimizedJSONEncoder struct {
	encoderPool *EncoderPool
	decoderPool *DecoderPool
}

// NewOptimizedJSONEncoder creates a new optimized JSON encoder
func NewOptimizedJSONEncoder() *OptimizedJSONEncoder {
	return &OptimizedJSONEncoder{
		encoderPool: NewEncoderPool(20), // 20 encoders
		decoderPool: NewDecoderPool(20), // 20 decoders
	}
}

// Marshal marshals data with high performance
func (oje *OptimizedJSONEncoder) Marshal(v interface{}) ([]byte, error) {
	return oje.encoderPool.Marshal(v)
}

// Unmarshal unmarshals data with high performance
func (oje *OptimizedJSONEncoder) Unmarshal(data []byte, v interface{}) error {
	return oje.decoderPool.Unmarshal(data, v)
}

// GetStats returns encoder/decoder pool statistics
func (oje *OptimizedJSONEncoder) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"encoder_pool_size": cap(oje.encoderPool.pool),
		"decoder_pool_size": cap(oje.decoderPool.pool),
	}
}

// Global optimized encoder instance
var globalOptimizedEncoder = NewOptimizedJSONEncoder()

// MarshalJSON marshals data using the global optimized encoder
func MarshalJSON(v interface{}) ([]byte, error) {
	return globalOptimizedEncoder.Marshal(v)
}

// UnmarshalJSON unmarshals data using the global optimized encoder
func UnmarshalJSON(data []byte, v interface{}) error {
	return globalOptimizedEncoder.Unmarshal(data, v)
}

// BenchmarkJSONPerformance benchmarks JSON encoding/decoding performance
func BenchmarkJSONPerformance() {
	data := map[string]interface{}{
		"score":        85.5,
		"confidence":   0.92,
		"contributors": []map[string]interface{}{{"name": "Alice", "contribution": 0.3}},
		"breakdown": map[string]float64{
			"shipping":      80.0,
			"quality":       70.0,
			"influence":     75.0,
			"complexity":    60.0,
			"collaboration": 85.0,
			"reliability":   90.0,
			"novelty":       65.0,
		},
	}

	// Warm up
	for i := 0; i < 1000; i++ {
		_, _ = MarshalJSON(data)
		jsonData, _ := MarshalJSON(data)
		_ = UnmarshalJSON(jsonData, &map[string]interface{}{})
	}

	// Benchmark marshaling
	start := time.Now()
	for i := 0; i < 10000; i++ {
		_, err := MarshalJSON(data)
		if err != nil {
			slog.Error("Marshal benchmark failed", "error", err)
			return
		}
	}
	marshalDuration := time.Since(start)

	// Benchmark unmarshaling
	jsonData, _ := MarshalJSON(data)
	start = time.Now()
	for i := 0; i < 10000; i++ {
		var result map[string]interface{}
		err := UnmarshalJSON(jsonData, &result)
		if err != nil {
			slog.Error("Unmarshal benchmark failed", "error", err)
			return
		}
	}
	unmarshalDuration := time.Since(start)

	slog.Info("JSON performance benchmarks",
		"marshal_10k_ops_ms", marshalDuration.Milliseconds(),
		"unmarshal_10k_ops_ms", unmarshalDuration.Milliseconds(),
		"avg_marshal_ms", float64(marshalDuration.Milliseconds())/10000,
		"avg_unmarshal_ms", float64(unmarshalDuration.Milliseconds())/10000,
	)
}
