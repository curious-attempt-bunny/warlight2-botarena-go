package main

import (
    "fmt"
    "log"
    "io"
    "os"
    "strings"
    "net/http"

    "github.com/PuerkitoBio/goquery"
)

//
// To use this you need to set bot_name and session_cookie
//

func scrape_maps() {
    bot_name := "BobTheBot" // your bot's name here
    for i := 1; i < 10; i++ {
        listing_url := fmt.Sprintf("http://theaigames.com/competitions/warlight-ai-challenge-2/game-log/%s/%d", bot_name, i+1)
        doc, err := goquery.NewDocument(listing_url)
        if err != nil {
            log.Fatal(err)
        }

        doc.Find("a").Each(func(i int, s *goquery.Selection) {
            link, _ := s.Attr("href")
            if strings.Index(link, "http://theaigames.com/competitions/warlight-ai-challenge-2/games/") == 0 {
                scrape_map(link)
            }
        })
    }
}

func scrape_map(link string) {
    session_cookie := "YOUR PHPSESSID SESSION COOKIE VALUE HERE"

    parts := strings.Split(link, "/")
    game_id := parts[len(parts)-1]

    fmt.Printf("%s:\n", game_id)

    dump_url := link + "/dump"

    client := &http.Client{}

    req, err := http.NewRequest("GET", dump_url, nil)
    req.Header.Add("Cookie", "PHPSESSID="+session_cookie)
    resp, err := client.Do(req)

    doc, err := goquery.NewDocumentFromResponse(resp)
    if err != nil {
        log.Fatal(err)
    }

    doc.Find("pre").Each(func(i int, s *goquery.Selection) {
        if i == 1 {
            output := s.Text()
            parts := strings.Split(output, "pick_starting_region")
            terrain := parts[0]

            file, err := os.Create(fmt.Sprintf("maps/%s.txt", game_id))
            if err != nil {
                fmt.Println(err)
            }

            io.WriteString(file, terrain)
        }
    });
}

func main() {
    scrape_maps()
}