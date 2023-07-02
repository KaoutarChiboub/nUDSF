package main

import (
	"context"
	"encoding/json"
	"fmt"
    "os"
	"log"
	"net/http"
	"flag"

	"github.com/joho/godotenv"
	"github.com/gorilla/mux"
	"github.com/go-playground/validator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Timer struct {
	TimerID           string            `json:"timerid" bson:"timerid" validate:"required"`
	Expires           string        `json:"expires" bson:"expires"`
	MetaTags          map[string]string `json:"metaTags" bson:"metaTags"`
	CallbackReference string            `json:"callbackReference" bson:"callbackReference"`
	DeleteAfter       int               `json:"deleteAfter" bson:"deleteAfter"`
}

// Load and get password from .env file
var errenv = godotenv.Load()
var pswd = os.Getenv("MONGO_PSD")
var certPath = os.Getenv("CERTIF_PATH")
var keyPath = os.Getenv("KEY_PATH")
var uri = fmt.Sprintf("mongodb+srv://kaoutarch:%s@kluster.valbk6m.mongodb.net/?retryWrites=true&w=majority", pswd)
var validate = validator.New()

// Connect to our MongoDB server
func ConnectMongo() *mongo.Collection {
	Ops := options.Client().ApplyURI(uri)
	c, err := mongo.Connect(context.TODO(), Ops)
	if err != nil {
		log.Fatal("Error connecting to MongoDB Atlas:", err)
	}
	fmt.Println("Successfully connected to MongoDB!")
	collection := c.Database("Timers").Collection("UDSF")
	return collection
}

var dbcon = ConnectMongo()

func replaceTimer(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	var timer Timer
	var extimer Timer
	p := mux.Vars(r)
	id := p["id"]
	filter := bson.M{"timerid": id}
	// First check if the timer with the given (filtered) id exists for later use
	exerr := dbcon.FindOne(context.TODO(), filter).Decode(&extimer)
	if exerr != nil {
		http.Error(w, "Failed to find requested item inside db!", http.StatusInternalServerError)
	}

    // Check for bad requests: invalid request input type
	err := json.NewDecoder(r.Body).Decode(&timer)
	if err != nil {
        http.Error(w, "Failed to parse request body! Please verify json inputs type!", http.StatusBadRequest)
        return
    }
	// Check the validation of timer request against defined tags
	err = validate.Struct(timer)
    if err != nil {
        http.Error(w, "Invalid request parameters!", http.StatusBadRequest)
        return
    }
	// Check if the request is fully authorized: last entry to not be modified
	if timer.DeleteAfter != extimer.DeleteAfter {
		http.Error(w, "Unauthorized: Modifying deleteAfter field of the last entry is not allowed. Please retry without it!", http.StatusUnauthorized)
    	return
	}
	// Go routine function to update/replace an existing timer in a synchronized way with the response
	// Channel created to synchronize and receive answer from go routine
	updateChan := make(chan struct{})
	go func(){
		defer close(updateChan)
		replace := bson.D {
			{"$set", bson.D {
				{"timerid", timer.TimerID},
				{"expires", timer.Expires},
				{"metaTags", timer.MetaTags},
				{"callbackReference", timer.CallbackReference},             
			}},
		}
		res := dbcon.FindOneAndUpdate(context.TODO(), filter, replace)
		if res != nil {
			if res.Err() == mongo.ErrNoDocuments {
				// Timer not found, return appropriate response
				http.Error(w, "Timer not found. Please verify the timer ID or create a new one!", http.StatusNotFound)
				return
			}
		}
		_ = res.Decode(&timer)
	}()
	// Wait for the update to be complete so we can synchronize the response sent to client
	<- updateChan
	w.WriteHeader(http.StatusOK)
    w.Write([]byte("This timer was updated successfully!\n")) 
	json.NewEncoder(w).Encode(timer) 
}

func createTimer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var timer Timer

	// Decode our body request params and parse to verify inputs
	pserr := json.NewDecoder(r.Body).Decode(&timer)
	if pserr != nil {
        http.Error(w, "Failed to parse request body! Please verify json inputs type!", http.StatusBadRequest)
        return
    }
	// Request validation check
	pserr = validate.Struct(timer)
    if pserr != nil {
        http.Error(w, "Invalid request parameters!", http.StatusBadRequest)
        return
    }
	// Custom validation: Check if the timerid is unique
	errUnique := UniqueField(timer.TimerID)
	if errUnique != nil {
		http.Error(w, "The timer ID must be unique!", http.StatusBadRequest)
		return
	}

	// Go routine to insert document into our db and synchronize the reponses with the use of a channel
	createChan := make(chan error)
	go func(){
		_, err := dbcon.InsertOne(context.TODO(), timer)
		createChan <- err
	}()
	// Again we wait for the go routine to complete and receive its value
	err := <-createChan
	if err != nil {
		http.Error(w, "Failed to insert a new timer to db!", http.StatusInternalServerError)
		return 
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("The timer was created successfully!\n")) 
	json.NewEncoder(w).Encode(timer)
}

func getTimers(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	var timers []Timer
	getChan := make(chan []Timer)
	errChan := make(chan error)
	// Go routine to fetch all docs (timers) from db
	go func (){
		cur, err := dbcon.Find(context.TODO(), bson.M{})
		if err != nil {
			errChan <- err
			return
		}
		// Defer the execution of closing the cursor until cur.Next returns
		defer cur.Close(context.TODO())
		// List all timers existing in database using a loop
		for cur.Next(context.TODO()) {
			var timer Timer
			err := cur.Decode(&timer)
			if err != nil {
				errChan <- err
			}
			timers = append(timers, timer)
		}
		if err := cur.Err(); err != nil {
			errChan <- err
			return
		}
		getChan <- timers
	}()	
	select {
	case timers := <- getChan:
		if len(timers) == 0 {
			http.Error(w, "No timers found", http.StatusNotFound) // 404 NOT FOUND
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(timers)
	case err := <- errChan :
		http.Error(w, "Failed to read from db!", http.StatusInternalServerError)
		log.Fatal(err)
	}
}

// UniqueField function checks the uniqueness of the timerid in the database
func UniqueField(timerID string) error {
	filter := bson.M{"timerid": timerID}

	// Check if a document with the given timerid already exists in the database
	count, err := dbcon.CountDocuments(context.TODO(), filter, nil)
	if err != nil {
		return err
	}

	if count > 0 {
		// A document with the same timerid already exists
		return fmt.Errorf("timer ID must be unique")
	}

	return nil
}

func main() {
	// Configuring server/tls
	host := flag.String("host", "localhost", "Required flag, must be the hostname that is resolvable via DNS, or 'localhost'")
	port := flag.String("port", "8443", "The https port, defaults to 8443")
	certFile := flag.String("certfile", certPath, "certificate file")
	keyFile := flag.String("keyfile", keyPath, "key file")
	flag.Parse()

	if *host == "" || *certFile == "" || *keyFile == "" {
		log.Fatalf("One or more required fields missing: serverCertFile, serverKeyFile, hostname or port")
	}

	// Initiating router to handle requests
	r := mux.NewRouter()
	r.HandleFunc("/timers/{id}", replaceTimer).Methods("PUT")
	r.HandleFunc("/timers", getTimers).Methods("GET")
	r.HandleFunc("/timers", createTimer).Methods("POST")

	// For a simple HTTP server, please uncomment this section and do the ≠ for all TLS congif
	//fmt.Printf("Starting the application at port 8080...\n")
	//log.Fatal(http.ListenAndServe(":8080", r))
	

	log.Printf("Starting HTTPS server on %s and port %s", *host, *port)
	if err := http.ListenAndServeTLS(*host+":"+*port, *certFile, *keyFile, r); err != nil {
		log.Fatal(err)
	}

}
