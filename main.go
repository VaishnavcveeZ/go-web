package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ThisUser struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	FName     string             `json:"Fname,omitempty" bson:"Fname,omitempty"`
	LName     string             `json:"Lname,omitempty" bson:"Lname,omitempty"`
	UName     string             `json:"Uname,omitempty" bson:"Uname,omitempty"`
	EmailId   string             `json:"emailId,omitempty" bson:"emailId,omitempty"`
	Gender    string             `json:"gender,omitempty" bson:"gender,omitempty"`
	Password  string             `json:"password,omitempty" bson:"password,omitempty"`
	RPassword string             `json:"Rpassword,omitempty" bson:"Rpassword,omitempty"`
}

var templates *template.Template
var client *mongo.Client

var store = sessions.NewCookieStore([]byte(os.Getenv("myKey")))

func main() {
	fmt.Println("Server Started--=>")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, _ = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	templates = template.Must(template.ParseGlob("templates/*.html"))
	fs := http.FileServer(http.Dir("./static/"))

	router := mux.NewRouter()
	router.PathPrefix("/static/").Handler((http.StripPrefix("/static", fs)))

	router.HandleFunc("/", login).Methods("GET")
	router.HandleFunc("/loginerror", loginerror).Methods("GET")
	router.HandleFunc("/signup", signup).Methods("GET")
	router.HandleFunc("/register", Register).Methods("POST")
	router.HandleFunc("/user", validate).Methods("POST")
	router.HandleFunc("/profile", userIN).Methods("GET")
	router.HandleFunc("/logout", logout).Methods("GET")
	router.HandleFunc("/updateImg", updateImg).Methods("GET")
	router.HandleFunc("/updateImgHandler", updateImgHandler).Methods("POST")

	http.Handle("/", router)
	http.ListenAndServe(":2000", nil)
}

func signup(response http.ResponseWriter, request *http.Request) {
	profileImageName, _, _ := logInCheck(response, request)
	if profileImageName != "null" {
		http.Redirect(response, request, "/profile", http.StatusSeeOther)
		return
	}
	templates.ExecuteTemplate(response, "signup.html", nil)
}

func Register(response http.ResponseWriter, request *http.Request) {
	profileImageName, _, _ := logInCheck(response, request)
	if profileImageName != "null" {
		http.Redirect(response, request, "/profile", http.StatusSeeOther)
		return
	}

	response.Header().Add("content-type", "application/json")
	var user ThisUser
	user.FName = request.FormValue("Fname")
	user.LName = request.FormValue("Lname")
	x := request.FormValue("Uname")
	user.UName = strings.ToLower(x)
	user.EmailId = request.FormValue("emailId")
	user.Gender = request.FormValue("gender")
	user.Password = request.FormValue("password")
	user.RPassword = request.FormValue("Rpassword")
	json.NewDecoder(request.Body).Decode(&user)
	collection := client.Database("CompanyUser").Collection("members")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	collection.InsertOne(ctx, user)
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func login(response http.ResponseWriter, request *http.Request) {
	profileImageName, _, _ := logInCheck(response, request)
	if profileImageName != "null" {
		http.Redirect(response, request, "/profile", http.StatusSeeOther)
		return
	}
	templates.ExecuteTemplate(response, "login.html", nil)
}

func loginerror(response http.ResponseWriter, request *http.Request) {
	profileImageName, _, _ := logInCheck(response, request)
	if profileImageName != "null" {
		http.Redirect(response, request, "/profile", http.StatusSeeOther)
		return
	}
	data := "username or password error !"
	templates.ExecuteTemplate(response, "login.html", data)
}

func validate(response http.ResponseWriter, request *http.Request) {
	profileImageName, _, _ := logInCheck(response, request)
	if profileImageName != "null" {
		http.Redirect(response, request, "/profile", http.StatusSeeOther)
		return
	}
	response.Header().Add("contet-type", "application/json")
	username := request.FormValue("user")
	password := request.FormValue("pass")
	var user ThisUser
	collection := client.Database("CompanyUser").Collection("members")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err := collection.FindOne(ctx, ThisUser{UName: username}).Decode(&user)
	psw := user.Password
	if psw != password || err != nil {
		http.Redirect(response, request, "/loginerror", http.StatusSeeOther)
		return
	}

	session, _ := store.Get(request, "session") // creating or fetching cookie session with name session
	session.Values["uname"] = user.UName
	session.Save(request, response)

	fmt.Println("uname : ", session.Values["uname"])
	http.Redirect(response, request, "/profile", http.StatusFound)
}

//function to check whether the user is login or not
func logInCheck(response http.ResponseWriter, request *http.Request) (string, ThisUser, error) {
	session, _ := store.Get(request, "session")
	data, ok := session.Values["uname"]     // data =interface ( "value"), ok = true/false
	sessionUname := fmt.Sprintf("%v", data) //converting interface data to a string
	var user ThisUser
	collection := client.Database("CompanyUser").Collection("members")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err := collection.FindOne(ctx, ThisUser{UName: sessionUname}).Decode(&user)
	if !ok || err != nil {
		sessionUname = "null"
		return sessionUname, user, err
	}
	return sessionUname, user, err
}

func userIN(response http.ResponseWriter, request *http.Request) {

	profileImageName, user, err := logInCheck(response, request)
	if err != nil {
		http.Redirect(response, request, "/", http.StatusSeeOther)
	}
	bucket, _ := gridfs.NewBucket(
		client.Database("CompanyUser"),
	)
	fscollection := client.Database("CompanyUser").Collection("fs.files")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	var result bson.M
	fserr := fscollection.FindOne(ctx, bson.M{}).Decode(&result)
	if fserr != nil {
		http.Redirect(response, request, "/", http.StatusSeeOther)
		return
	}
	var buf bytes.Buffer
	dstream, _ := bucket.DownloadToStreamByName(profileImageName, &buf)
	writeName := "static/profile/images/Dp/" + profileImageName + ".jpg"
	fmt.Printf("file size : %v \n", dstream)
	ioutil.WriteFile(writeName, buf.Bytes(), 0600)

	var outData = map[string]string{"image": writeName, "FName": user.FName, "LName": user.LName, "UName": profileImageName}
	templates.ExecuteTemplate(response, "profile.html", outData)

}
func logout(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	session, _ := store.Get(request, "session") // creating or fetching cookie session with name session
	session.Values["uname"] = "null"
	session.Save(request, response)
	http.Redirect(response, request, "/", http.StatusSeeOther)
}

func updateImg(response http.ResponseWriter, request *http.Request) {
	Uname, _, _ := logInCheck(response, request)
	if Uname == "null" {
		http.Redirect(response, request, "/", http.StatusSeeOther)
		return
	}
	templates.ExecuteTemplate(response, "update_img.html", nil)
}
func updateImgHandler(response http.ResponseWriter, request *http.Request) {
	Uname, _, _ := logInCheck(response, request)
	if Uname == "null" {
		http.Redirect(response, request, "/", http.StatusSeeOther)
		return
	}
	profileImageName := Uname
	response.Header().Add("contet-type", "application/json")
	file, _, _ := request.FormFile("profileImg")
	var buf bytes.Buffer
	io.Copy(&buf, file)
	content := buf.String()

	bucket, _ := gridfs.NewBucket(
		client.Database("CompanyUser"),
	)
	uploadStream, _ := bucket.OpenUploadStream(profileImageName)
	fileSize, _ := uploadStream.Write([]byte(content))
	log.Printf("write done , file size : %d \n", fileSize)
	buf.Reset()
	uploadStream.Close()
	http.Redirect(response, request, "/profile", http.StatusSeeOther)
	return

}
