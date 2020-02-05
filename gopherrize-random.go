package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var (
	format       = "json"
	responseType = "in_channel"
)

type SlackResponse struct {
	ResponseType string       `json:"response_type"`
	Attachments  []Attachment `json:"attachments,omitempty"`
	Text         string       `json:"text,omitempty"`
}

type Attachment struct {
	Fallback  string `json:"fallback"`
	ImageURL  string `json:"image_url"`
	Title     string `json:"title"`
	TitleLink string `json:"title_link"`
}

type Artwork struct {
	Categories []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Images []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Href          string `json:"href"`
			ThumbnailHref string `json:"thumbnail_href"`
		} `json:"images"`
	} `json:"categories"`
	TotalCombinations int64 `json:"total_combinations"`
}

func GetArtwork() (Artwork, error) {
	var artwork Artwork
	resp, err := http.Get("https://gopherize.me/api/artwork")

	if err != nil {
		log.Printf("ERROR : %v", err)
		return artwork, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("ERROR : %v", err)
		return artwork, err
	}

	err = json.Unmarshal(body, &artwork)
	if err != nil {
		log.Printf("ERROR : %v", err)
		return artwork, err
	}

	return artwork, nil
}

func GenerateRandomGopher() (string, error) {
	url, err := GenerateGopherURL()
	if err != nil {
		log.Printf("ERROR : %v", err)
		return url, err
	}
	img, err := GenerateGopherImage(url)

	if err != nil {
		log.Printf("ERROR : %v", err)
		return img, err
	}
	return img, nil
}

func GenerateGopherImage(url string) (string, error) {
	var imageURL string
	getImageURL := fmt.Sprintf("https://gopherize.me/save?images=%s", url)
	resp, err := http.Get(getImageURL)

	if err != nil {
		log.Printf("ERROR : %v", err)
		return imageURL, err
	}

	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)

	//crawl
	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			return imageURL, nil
		case tt == html.StartTagToken:
			t := z.Token()
			isAnchor := t.Data == "img"
			if isAnchor {

				//parse
				for _, a := range t.Attr {
					if a.Key == "src" {
						return a.Val, nil
					}
				}
			}
		}
	}

	return imageURL, nil
}

func GenerateGopherURL() (string, error) {
	var url string

	artwork, err := GetArtwork()

	if err != nil {
		log.Printf("ERROR : %v", err)
		return url, err
	}

	for _, category := range artwork.Categories {
		rand.Seed(time.Now().UnixNano())
		r := rand.Intn(len(category.Images))
		url += category.Images[r].ID + "|"
	}

	url = strings.TrimSuffix(url, "|")

	return url, nil
}

func GetRandomGopher(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		response := make(map[string]interface{})
		img, err := GenerateRandomGopher()
		queryFormat := r.URL.Query().Get("format")
		if err != nil {
			log.Printf("ERROR : %v", err)

			w.WriteHeader(http.StatusInternalServerError)
			response["error"] = err.Error()
			json.NewEncoder(w).Encode(response)
			return
		} else {

			if queryFormat == format {
				w.WriteHeader(http.StatusOK)
				response["img"] = img
				json.NewEncoder(w).Encode(response)
				return
			}

			//redirect to the image oage
			http.Redirect(w, r, img, http.StatusSeeOther)
			return
		}
	} else if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("ERROR : %v", err)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("_Something went wrong :(_"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("_New Gopher incoming_"))
		go asyncResponse(string(body))
		return
	} else {
		//not supported
		w.WriteHeader(http.StatusNotImplemented)

	}

}

func asyncResponse(body string) {
	re := regexp.MustCompile("(response_url=)(.*)&")
	match := re.FindStringSubmatch(body)
	if len(match) >= 2 {
		slackResponseURL := match[2]

		decodedURL, err := url.QueryUnescape(slackResponseURL)
		if err != nil {
			log.Printf("ERROR : %v", err)
			return
		}

		img, err := GenerateRandomGopher()

		if err != nil {
			log.Printf("ERROR : %v", err)
			return
		}

		err = sendDelayedResponse(decodedURL, img)
		if err != nil {
			log.Printf("ERROR : %v", err)
		}
	}
}

func sendDelayedResponse(responseURL, imageURL string) error {

	response := SlackResponse{}
	response.ResponseType = responseType
	response.Attachments = append(response.Attachments, Attachment{
		Fallback:  "Gopher !!",
		ImageURL:  imageURL,
		Title:     "gopher",
		TitleLink: imageURL,
	})

	responseBody, err := json.Marshal(response)
	if err != nil {
		return err
	}
	resp, err := http.Post(responseURL, "application/json", bytes.NewBuffer(responseBody))

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response to slack with the image return a non %d", 200)
	}
	return nil
}

func main() {

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	http.HandleFunc("/random-gopher", GetRandomGopher)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port : %d \n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))

}
