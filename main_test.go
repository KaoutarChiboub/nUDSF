package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"os"
	"fmt"

	"github.com/gorilla/mux"
)
func TestCreateTimer(t *testing.T) {
	// Create a new HTTP POST request with the timer data: we test an already existing timerid to create and check the fail error
	timer := Timer{
		TimerID:           "kaoutar",
		Expires:           "2023-06-26T13:40:17.396Z",
		MetaTags:          map[string]string{"tag1": "value1", "tag2": "value2"},
		CallbackReference: "example",
		DeleteAfter:       0,
	}
	jsonTimer, _ := json.Marshal(timer)
	req, err := http.NewRequest("POST", "/timers", bytes.NewBuffer(jsonTimer))
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder to capture the API response
	rr := httptest.NewRecorder()

	// Create a new instance of the router and handle the request
	r := mux.NewRouter()
	r.HandleFunc("/timers", createTimer).Methods("POST")
	r.ServeHTTP(rr, req)

	// Check the HTTP status code
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d but got %d", http.StatusCreated, rr.Code)
	}

	// Check the response body
	expectedResponse := "The timer was created successfully!\n"
	if rr.Body.String() != expectedResponse {
		t.Errorf("Expected response body '%s' but got '%s'", expectedResponse, rr.Body.String())
	}
}

func TestGetTimers(t *testing.T) {
	// Create a new HTTP GET request
	req, err := http.NewRequest("GET", "/timers", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder to capture the API response
	rr := httptest.NewRecorder()

	// Create a new instance of the router and handle the request
	r := mux.NewRouter()
	r.HandleFunc("/timers", getTimers).Methods("GET")
	r.ServeHTTP(rr, req)

	// Check the HTTP status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, rr.Code)
	}

	// Check the response body
	// We out an empty timer array to test in case our db was empty what could be the reponse
	expectedTimers := []Timer{}
	expectedJSON, _ := json.Marshal(expectedTimers)
	if rr.Body.String() != string(expectedJSON) {
		t.Errorf("Expected response body '%s' but got '%s'", string(expectedJSON), rr.Body.String())
	}
}

func TestReplaceTimer(t *testing.T) {
	// Create a new HTTP PUT request with the timer data: Put a timerid that do NOT exists in db but a MODIFIED DeleteAfter field to test the response
	timer := Timer{
		TimerID:           "kaoutar22",
		Expires:           "2023-06-26T13:40:17.396Z",
		MetaTags:          map[string]string{"tag1": "value1", "tag2": "value2"},
		CallbackReference: "PUTexample",
		DeleteAfter:       1000,
	}
	jsonTimer, _ := json.Marshal(timer)
	req, err := http.NewRequest("PUT", "/timers/123", bytes.NewBuffer(jsonTimer))
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder to capture the API response
	rr := httptest.NewRecorder()

	// Create a new instance of the router and handle the request
	r := mux.NewRouter()
	r.HandleFunc("/timers/{id}", replaceTimer).Methods("PUT")
	r.ServeHTTP(rr, req)

	// Check the HTTP status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d but got %d", http.StatusOK, rr.Code)
	}

	// Check the response body
	expectedResponse := "The timer was updated successfully!\n"
	if rr.Body.String() != expectedResponse {
		t.Errorf("Expected response body '%s' but got '%s'", expectedResponse, rr.Body.String())
	}
}

func TestMain(m *testing.M) {
	fmt.Println("Running tests...")
	// Run the tests
	test := m.Run()
	os.Exit(test)
}
