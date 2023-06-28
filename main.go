package main

import (
	"context"
	"encoding/json"
	"fmt"
    "os"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Timer struct {
	TimerID           string            `json:"timerid" bson:"timerid"`
	Expires           string        `json:"expires" bson:"expires"`
	MetaTags          map[string]string `json:"metaTags" bson:"metaTags"`
	CallbackReference string            `json:"callbackReference" bson:"callbackReference"`
	DeleteAfter       int               `json:"deleteAfter" bson:"deleteAfter"`
}

// Load and get password from .env file
var errenv = godotenv.Load()
var pswd = os.Getenv("MONGO_PSD")
var uri = fmt.Sprintf("mongodb+srv://kaoutarch:%s@kluster.valbk6m.mongodb.net/?retryWrites=true&w=majority", pswd)

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
	// Check if the request is fully authorized: last entry to not be modified
	if timer.DeleteAfter != extimer.DeleteAfter {
		http.Error(w, "Unauthorized: Modifying deleteAfter field of the last entry is not allowed. Please retry without it!", http.StatusUnauthorized)
    	return
	}
	//res := dbcon.FindOne(context.TODO(), filter)
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
	w.WriteHeader(http.StatusOK)
    w.Write([]byte("Timer updated successfully!\n")) 
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
	// Insert document into our db
	_, err := dbcon.InsertOne(context.TODO(), timer)
	if err != nil {
		http.Error(w, "Failed to insert a new timer to db!", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(timer)
}

func getTimers(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	var timers []Timer
	// Find/fetch all documents in db without any filter	
	cur, err := dbcon.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to read from db", http.StatusInternalServerError)
		return
	}
	// Defer the execution of closing the cursor until cur.Next returns
	defer cur.Close(context.TODO())
	// List all timers existing in database using a loop
	for cur.Next(context.TODO()) {
		var timer Timer
		err := cur.Decode(&timer)
		if err != nil {
			log.Fatal(err)
		}
		timers = append(timers, timer)
	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}
	// If no document is found, return 404 NOT FOUND
	if len(timers) == 0 {
        http.Error(w, "No timers found", http.StatusNotFound) // 404 NOT FOUND
        return
    }
	w.WriteHeader(http.StatusOK) //200 SUCCESS
	json.NewEncoder(w).Encode(timers) // List timers list
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/timers/{id}", replaceTimer).Methods("PUT")
	r.HandleFunc("/timers", getTimers).Methods("GET")
	r.HandleFunc("/timers", createTimer).Methods("POST")

	fmt.Printf("Starting the application at port 8080...\n")
	log.Fatal(http.ListenAndServe(":8080", r))
}