package main

import (
	"encoding/json"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

const x_tokyo float64 = 139.7673068
const y_tokyo float64 = 35.6809591
const r_earth float64 = 6371.0

type GeoApiResponse struct {
	Response Location `json:"response"`
}

type Location struct {
	Location []LocationInfo `json:"location"`
}

type LocationInfo struct {
	City       string `json:"city"`
	CityKana   string `json:"city_kana"`
	Town       string `json:"town"`
	TownKana   string `json:"town_kana"`
	X          string `json:"x"`
	Y          string `json:"y"`
	Prefecture string `json:"prefecture"`
	Postal     string `json:"postal"`
}

type Response struct {
	PostalCode       string  `json:"postal_code"`
	HitCount         int     `json:"hit_count"`
	Address          string  `json:"address"`
	TokyoStaDistance float64 `json:"tokyo_sta_distance"`
}

type PostalCodeCount struct {
	PostalCode   string `json:"postal_code"`
	RequestCount int    `json:"request_count"`
}

type Log struct {
	ID         uint
	PostalCode string
	CreatedAt  time.Time
}

func handleAddress(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	if v == nil {
		return
	}
	postal_code := v["postal_code"][0]

	db.Create(&Log{PostalCode: postal_code})

	url := fmt.Sprintf("https://geoapi.heartrails.com/api/json?method=searchByPostal&postal=%s", postal_code)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("Error: status code", resp.StatusCode)
		return
	}

	body, _ := io.ReadAll(resp.Body)

	var response_geo GeoApiResponse
	if err := json.Unmarshal(body, &response_geo); err != nil {
		fmt.Println(err)
		return
	}

	locationInfo := response_geo.Response.Location[0]

	prefecture_res := locationInfo.Prefecture
	city_res := locationInfo.City
	town_res := locationInfo.Town
	x_res_str := locationInfo.X
	y_res_str := locationInfo.Y
	postal_res := locationInfo.Postal

	x_res, err := strconv.ParseFloat(x_res_str, 64)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return
	}

	y_res, err := strconv.ParseFloat(y_res_str, 64)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return
	}

	tokyoStaDistance := (math.Pi * r_earth / 180) * math.Sqrt(math.Pow((x_res-x_tokyo)*math.Cos(math.Pi*(y_res-y_tokyo)/360), 2)+math.Pow(y_res-y_tokyo, 2))
	tokyoStaDistance, err = strconv.ParseFloat(fmt.Sprintf("%.1f", tokyoStaDistance), 64)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return
	}

	hitCount := len(response_geo.Response.Location)
	address := prefecture_res + city_res + town_res

	response := Response{
		PostalCode:       postal_res,
		HitCount:         hitCount,
		Address:          address,
		TokyoStaDistance: tokyoStaDistance,
	}
	res, err := json.Marshal(response)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}

func handleAddressAccessLogs(w http.ResponseWriter, r *http.Request) {
	var result []PostalCodeCount
	db.Raw("SELECT postal_code, count(postal_code) as request_count FROM logs GROUP BY postal_code ORDER BY count(postal_code) DESC;").Scan(&result)

	logs := map[string][]PostalCodeCount{"access_logs": result}
	res, err := json.Marshal(logs)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open(sqlite.Open("gorm.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
}

func main() {
	db.AutoMigrate(&Log{})

	server := http.Server{
		Addr: ":8080",
	}
	http.HandleFunc("/address", handleAddress)
	http.HandleFunc("/address/access_logs", handleAddressAccessLogs)
	server.ListenAndServe()
}
