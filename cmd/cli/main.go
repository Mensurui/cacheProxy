package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
)

type config struct {
	port   int
	origin string
}

type cachedResponse struct {
	Body    []byte
	Headers http.Header
}

type cache map[string]cachedResponse

type application struct {
	config config
	cache  cache
}

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 3000, "Used port")
	flag.StringVar(&cfg.origin, "origin", "https://google.com", "Destination url")
	flag.Parse()

	cache := make(map[string]cachedResponse)
	app := &application{
		config: cfg,
		cache:  cache,
	}

	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.port),
		Handler: app,
	}

	err := srv.ListenAndServe()
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

}

func (app *application) request(w http.ResponseWriter, r *http.Request) {
	forwardedURL := app.config.origin + r.URL.Path

	// Check if the response is cached
	if cachedResp, found := app.cache[forwardedURL]; found {
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(cachedResp.Body)
		if err != nil {
			http.Error(w, "Failed to write cached response", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("X-Cache", "MISS")

	req, err := http.NewRequest(r.Method, forwardedURL, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header = r.Header

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	// Cache the response
	app.cache[forwardedURL] = cachedResponse{
		Body:    body,
		Headers: resp.Header,
	}

	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(body)
	if err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

func (app *application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.request(w, r)
}
