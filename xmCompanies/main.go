package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var types = []string{"Corporations", "NonProfit", "Cooperative", "Sole Proprietorship"}

type Company struct {
	ID                uuid.UUID `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	AmountOfEmployees *int      `json:"amount_of_employees"`
	Registered        *bool     `json:"registered"`
	Type              string    `json:"type"`
}

var db *sql.DB

func initialiseDatabase() {
	fmt.Println("Initializing DB connection...")
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	portStr := os.Getenv("MY_PORT")    // Get the PORT value as a string
	port, err := strconv.Atoi(portStr) // Convert PORT string value to int
	if err != nil {
		panic(err)
	}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		os.Getenv("MY_HOST"), port, os.Getenv("MY_USER"), os.Getenv("MY_PASSWORD"), os.Getenv("MY_DB_NAME"))

	fmt.Println(psqlInfo)
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully connected to DB")
}

func initialiseRestAPI() {
	fmt.Println("Initialising REST API...")
	router := gin.Default()
	router.GET("/companies/:id", getCompanies)
	router.GET("/companies/", getCompanies)
	router.POST("/companies", createCompany)
	router.DELETE("/companies/:id", deleteCompany)
	router.PATCH("/companies/:id", updateCompany)
	err := router.Run(":8080")
	if err != nil {
		return
	}
}

func authedicate(context *gin.Context) int {

	auth := context.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		context.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing basic auth"})
		return -1
	}

	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		context.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth encoding"})
		return -1
	}

	parts := strings.SplitN(string(payload), ":", 2)
	if len(parts) != 2 {
		context.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Malformed auth"})
		return -1
	}
	//Normally, we should fix this for sql injection along with post and update
	username := parts[0]
	password := parts[1]

	query := "SELECT id FROM users WHERE username=$1 and password=$2"

	rows, err := db.Query(query, username, password)

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return -1
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	if !rows.Next() {
		context.JSON(http.StatusUnauthorized, gin.H{"error": "Username or password is incorrect"})
		return -1
	}
	return 1

}

func getCompanies(context *gin.Context) {
	var getID = getIDFromRequest(context)
	if getID != "" {
		getID = " where id='" + getID + "'" + ""
	}
	var query = "SELECT id, name, description, amount_of_employees, registered, type FROM companies" + getID

	rows, err := db.Query(query)
	var flag = false

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var companies []Company
	for rows.Next() {
		flag = true
		var c Company
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.AmountOfEmployees, &c.Registered, &c.Type)
		if err != nil {
			context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		companies = append(companies, c)
	}

	if flag == false && getID != "" {
		context.JSON(http.StatusNotFound, gin.H{"error": "No companies found for given ID"})
		return
	}
	context.JSON(http.StatusOK, companies)
}

func createCompany(context *gin.Context) {
	if authedicate(context) == -1 {
		return
	}
	var req Company = validatePayload(context, "create")
	if req.Name == "" {
		return
	}

	query := "INSERT INTO companies(name, description, amount_of_employees, registered, type) VALUES ($1, $2, $3, $4, $5) RETURNING id"

	err := db.QueryRow(query, req.Name, req.Description, *req.AmountOfEmployees, *req.Registered, req.Type).Scan(&req.ID)

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else {
		context.JSON(http.StatusCreated, &req)
		return
	}

}

func deleteCompany(context *gin.Context) {
	if authedicate(context) == -1 {
		return
	}
	var getID = getIDFromRequest(context)

	if getID == "" {
		return
	}
	query := "DELETE FROM companies WHERE ID = $1"
	result, err := db.Exec(query, getID)

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rowsAffected > 0 {
		context.JSON(http.StatusOK, "")
		return
	} else {
		context.JSON(http.StatusNotFound, "Company not found")
		return
	}

}

func updateCompany(context *gin.Context) {
	if authedicate(context) == -1 {
		return
	}
	var getID = getIDFromRequest(context)

	if getID == "" {
		return
	}
	var req Company = validatePayload(context, "update")
	var fieldsToChange []string

	if req.Name != "" {
		fieldsToChange = append(fieldsToChange, " name = '"+req.Name+"'")
	}
	if req.Description != "" {
		fieldsToChange = append(fieldsToChange, "description = '"+req.Description+"'")
	}
	if req.AmountOfEmployees != nil {
		fieldsToChange = append(fieldsToChange, "amount_of_employees = '"+strconv.Itoa(*req.AmountOfEmployees)+"'")
	}
	if req.Registered != nil {
		fieldsToChange = append(fieldsToChange, "registered = '"+strconv.FormatBool(*req.Registered)+"'")
	}
	if req.Type != "" {
		fieldsToChange = append(fieldsToChange, "type = '"+req.Type+"'")
	}
	var updated Company
	query := "UPDATE companies SET " + strings.Join(fieldsToChange, ", ") + " where id='" + getID + "'" + " RETURNING id"
	fmt.Println(query)
	err := db.QueryRow(query).Scan(&updated.ID)

	if err != nil {
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else {
		context.JSON(http.StatusAccepted, "")

		return
	}

}

func getJSONFields(v interface{}) map[string]bool {
	fields := map[string]bool{}
	t := reflect.TypeOf(v)

	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag != "" && tag != "-" {
			name := tag
			if commaIdx := indexComma(tag); commaIdx != -1 {
				name = tag[:commaIdx]
			}
			fields[name] = true
		}
	}
	return fields
}

func indexComma(s string) int {
	for i, ch := range s {
		if ch == ',' {
			return i
		}
	}
	return -1
}

func validatePayload(context *gin.Context, method string) Company {
	var raw map[string]interface{}
	var req Company
	if err := context.ShouldBindJSON(&raw); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "Body is empty"})
		return req
	}
	knownFields := getJSONFields(Company{})
	var unknown []string
	for key := range raw {
		if !knownFields[key] {
			unknown = append(unknown, key)
		}
	}

	if len(unknown) > 0 {
		context.JSON(http.StatusBadRequest, gin.H{
			"error":          "Unknown fields in request",
			"unknown_fields": unknown,
		})
		return req
	}

	data, _ := json.Marshal(raw)
	_ = json.Unmarshal(data, &req)
	fmt.Println(string(data))

	errorMessage := ""
	if method == "create" {

		if req.Name == "" {
			errorMessage = "The field name cannot be empty."
		}

		if req.AmountOfEmployees == nil {
			errorMessage += " The field amount_of_employees cannot be empty."
		} else if *req.AmountOfEmployees < 1 {
			errorMessage += " The field amount_of_employees has to be an integer greater than 0."

		}

		if req.Registered == nil {
			errorMessage += " The field registered cannot be empty."
		}
		if req.Type == "" {
			errorMessage += " The field Type cannot be empty."
		} else {
			exists := false
			for _, value := range types {
				if req.Type == value {
					exists = true
					break
				}
			}
			if exists == false {
				errorMessage += " The type field accepts only the values " + strings.Join(types, ", ") + ". Received:" + req.Type
			}
		}
	}
	if method == "update" {
		var decoded map[string]interface{}
		err := json.Unmarshal(data, &decoded)
		if err != nil {
			log.Fatal(err)
		}
		for key, value := range decoded {

			if key == "id" {
				context.JSON(http.StatusBadRequest, gin.H{"error": "Field 'id' is not updatable"})
				return req
			}
			if key != "description" && value == "" {
				context.JSON(http.StatusBadRequest, gin.H{"error": "Field " + key + " cannot be empty"})
				return req
			}
			if key == "type" {
				exists := false
				for _, typeValue := range types {
					if req.Type == typeValue {
						exists = true
						break
					}
				}
				if exists == false {
					var response string = ""
					response = " The type field accepts only the values " + strings.Join(types, ", ") + ". Received:" + req.Type
					context.JSON(http.StatusBadRequest, gin.H{"error": response})
					return req
				}

			}
		}

		if *req.AmountOfEmployees < 1 {

			var response string = ""
			response = " The field amount_of_employees has to be an integer greater than 0. Received:" + req.Type
			context.JSON(http.StatusBadRequest, gin.H{"error": response})
			return req
		}
	}

	if errorMessage != "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": errorMessage})
		return req
	}
	return req
}

func getIDFromRequest(context *gin.Context) string {
	idParam := context.Param("id")
	var getID = ""
	if idParam != "" {
		r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
		if !r.MatchString(idParam) {
			context.JSON(http.StatusInternalServerError, gin.H{"Please provide a valid company ID. Provided: ": idParam})
			return ""
		}
		getID = idParam

	}
	return getID

}

func main() {
	initialiseDatabase()
	initialiseRestAPI()

}
