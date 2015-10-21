package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"bitbucket.org/atlassianlabs/hipchat-golang-base/util"

	"github.com/gorilla/mux"
	"github.com/tbruyelle/hipchat-go/hipchat"
)

// RoomConfig holds information to send messages to a specific room
type RoomConfig struct {
	token *hipchat.OAuthAccessToken
	hc    *hipchat.Client
	name  string
}

// Context keep context of the running application
type Context struct {
	baseURL string
	static  string
	//rooms per room OAuth configuration and client
	rooms map[string]*RoomConfig
	db    *sql.DB
}

// Key store class
type UserKey struct {
	userMention string
	keyText     string
	keyType     string
	userID      int
}

func (c *Context) healthcheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode([]string{"OK"})
}

func (c *Context) atlassianConnect(w http.ResponseWriter, r *http.Request) {
	lp := path.Join(c.static, "keybot-connect.json")
	vals := map[string]string{
		"LocalBaseUrl": c.baseURL,
	}
	tmpl, err := template.ParseFiles(lp)
	if err != nil {
		log.Fatalf("%v", err)
	}
	tmpl.ExecuteTemplate(w, "config", vals)
}

func (c *Context) installable(w http.ResponseWriter, r *http.Request) {
	authPayload, err := util.DecodePostJSON(r, true)
	if err != nil {
		log.Fatalf("Parsed auth data failed:%v\n", err)
	}

	credentials := hipchat.ClientCredentials{
		ClientID:     authPayload["oauthId"].(string),
		ClientSecret: authPayload["oauthSecret"].(string),
	}
	roomName := strconv.Itoa(int(authPayload["roomId"].(float64)))
	newClient := hipchat.NewClient("")
	tok, _, err := newClient.GenerateToken(credentials, []string{hipchat.ScopeSendNotification})
	if err != nil {
		log.Fatalf("Client.GetAccessToken returns an error %v", err)
	}
	rc := &RoomConfig{
		name: roomName,
		hc:   tok.CreateClient(),
	}
	c.rooms[roomName] = rc

	util.PrintDump(w, r, false)
	json.NewEncoder(w).Encode([]string{"OK"})
}

func (c *Context) config(w http.ResponseWriter, r *http.Request) {
	signedRequest := r.URL.Query().Get("signed_request")
	lp := path.Join(c.static, "layout.hbs")
	fp := path.Join(c.static, "config.hbs")
	vals := map[string]string{
		"LocalBaseUrl":  c.baseURL,
		"SignedRequest": signedRequest,
		"HostScriptUrl": c.baseURL,
	}
	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		log.Fatalf("%v", err)
	}
	tmpl.ExecuteTemplate(w, "layout", vals)
}

func (c *Context) triageMessage(w http.ResponseWriter, r *http.Request) {
	payLoad, err := util.DecodePostJSON(r, true)
	if err != nil {
		log.Fatalf("Parsed payload failed:%v\n", err)
	}
	payloadMsg := payLoad["item"].(map[string]interface{})["message"].(map[string]interface{})["message"]

	if payloadMsg, ok := payloadMsg.(string); ok {
		if strings.Contains(payloadMsg, "keybot set") {
			set_keys(w, r, payLoad)
		}
	}

}

func (c *Context) set_keys(w http.ResponseWriter, r *http.Request, payload map[string]interface{}) {

	/*

	 Get the basic infomation to proceed:
	  * RoomID -> where did this message come from
	  * senderID -> who did this message come from
	  * payloadMsg - > what is this message

	*/

	roomID := strconv.Itoa(int((payLoad["item"].(map[string]interface{}))["room"].(map[string]interface{})["id"].(float64)))
	senderID := int((payLoad["item"].(map[string]interface{}))["message"].(map[string]interface{})["from"].(map[string]interface{})["id"].(float64))
	mentionID := (payLoad["item"].(map[string]interface{}))["message"].(map[string]interface{})["from"].(map[string]interface{})["mention_name"]
	payloadMsg := payLoad["item"].(map[string]interface{})["message"].(map[string]interface{})["message"]

	log.Printf("Room %s \n sender: %n: \n payload: %s ", roomID, senderID, payloadMsg)

	// Prep response
	var messageStr string
	var colorStr string

	var keyText string
	var keyType string

	if payloadMsg, ok := payloadMsg.(string); ok {
		payloadMsg = strings.Replace(payloadMsg, "/set_key ", "", -1)
		//Get the type of key
		var strParts = strings.Split(payloadMsg, " ")
		for _, pair := range strParts {
			tokens := strings.Split(pair, "=")
			if strings.ToLower(tokens[0]) == "type" {
				keyType = tokens[1]
			}
		}
		//Get the start of key
		if strings.Contains(payloadMsg, "-----BEGIN") {
			keyText = payloadMsg[strings.Index(payloadMsg, "-----BEGIN"):]
		}
		if strings.Contains(payloadMsg, "----- BEGIN") {
			keyText = payloadMsg[strings.Index(payloadMsg, "----- BEGIN"):]
		}
		if strings.Contains(payloadMsg, "ssh-rsa") {
			keyText = payloadMsg[strings.Index(payloadMsg, "ssh-rsa"):]
		}

		uk := UserKey{
			keyText: keyText,
			keyType: keyType,
			userID:  senderID,
		}

		if c.db == nil {
			log.Printf("db is nil")
		}

		err = c.db.Ping()
		if err == nil {
			log.Printf("No ping to db")
		} else {
			log.Printf("Ping successful")
		}
		stmt, dberr := c.db.Prepare("INSERT INTO keys(userid, keytype, keytext) VALUES($1,$2,$3)")
		if dberr != nil {
			log.Fatal(dberr)
		}
		res, dberr := stmt.Exec(uk.userID, uk.keyType, uk.keyText)
		if dberr != nil {
			log.Fatal(dberr)
		}
		lastId, dberr := res.LastInsertId()
		if dberr != nil {
			log.Fatal(dberr)
		}
		rowCnt, dberr := res.RowsAffected()
		if err != nil {
			log.Fatal(dberr)
		}
		log.Printf("ID = %d, affected = %d\n", lastId, rowCnt)

		checkErr(err)
		colorStr = "blue"
		messageStr = "Saved Key"
	} else {
		messageStr = "Error, bad message "
		colorStr = "red"
	}

	log.Printf("Sending notification to %s\n", roomID)
	notifRq := &hipchat.NotificationRequest{
		Message:       messageStr,
		MessageFormat: "html",
		Color:         colorStr,
	}
	if _, ok := c.rooms[roomID]; ok {
		_, err = c.rooms[roomID].hc.Room.Notification(roomID, notifRq)
		if err != nil {
			log.Printf("Failed to notify HipChat channel:%v\n", err)
		}
	} else {
		log.Printf("Room is not registered correctly:%v\n", c.rooms)
	}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// routes all URL routes for app add-on
func (c *Context) routes() *mux.Router {
	r := mux.NewRouter()

	//healthcheck route required by Micros
	r.Path("/healthcheck").Methods("GET").HandlerFunc(c.healthcheck)

	//descriptor for Atlassian Connect
	r.Path("/").Methods("GET").HandlerFunc(c.atlassianConnect)
	r.Path("/keybot-connect.json").Methods("GET").HandlerFunc(c.atlassianConnect)

	// HipChat specific API routes
	r.Path("/installable").Methods("POST").HandlerFunc(c.installable)
	r.Path("/config").Methods("GET").HandlerFunc(c.config)

	r.Path("/keybot").Methods("POST").HandlerFunc(c.set_keys)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(c.static)))
	return r
}

func main() {
	var (
		port       = flag.String("port", "7631", "web server port")
		static     = flag.String("static", "/opt/hc_keybot/static/", "static folder")
		baseURL    = flag.String("baseurl", os.Getenv("BASE_URL"), "local base url")
		dbhost     = flag.String("database", "localhost", "database server")
		dbuser     = flag.String("dbuser", os.Getenv("BOTDB_USER"), "databse user")
		dbpassword = flag.String("dbpassword", os.Getenv("BOTDB_PASS"), "databse user")
		dbname     = flag.String("dbname", os.Getenv("BOTDB_DB"), "databse user")
	)
	flag.Parse()

	dbinfo := fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=disable", *dbuser, *dbpassword, *dbname, *dbhost)
	log.Printf(dbinfo)

	db, err := sql.Open("postgres", dbinfo)
	checkErr(err)

	defer db.Close()

	err = db.Ping()
	if err == nil {
		log.Printf("No ping to db")
	} else {
		log.Printf("Ping successful")
	}

	c := &Context{
		baseURL: *baseURL,
		static:  *static,
		db:      db,
		rooms:   make(map[string]*RoomConfig),
	}

	log.Printf("QOELabs hc_keybot v0.10 - running on port:%v", *port)

	r := c.routes()
	http.Handle("/", r)
	http.ListenAndServe(":"+*port, nil)
}
